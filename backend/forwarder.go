package backend

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/soheilhy/cmux"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"

	"github.com/anxiousmodernman/co-chair/proto/server"
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/sirupsen/logrus"
	"github.com/vulcand/oxy/trace"
)

// NewTCPForwarder constructs a TCPForwarder from a variable list of options.
// Passing a database (either by pass or by reference) is required or the
// TCPForwarder will fail at runtime.
func NewTCPForwarder(opts ...Opt) (*TCPForwarder, error) {

	var fwdr TCPForwarder
	for _, opt := range opts {
		opt(&fwdr)
	}

	return &fwdr, nil
}

// An Opt lets us set values on a TCPForwarder.
type Opt func(*TCPForwarder)

// WithDBPath opens a DB at path and sets it on our TCPForwarder.
func WithDBPath(path string) Opt {
	return func(fwdr *TCPForwarder) {
		db, err := storm.Open(path)
		if err != nil {
			panic(err)
		}
		fwdr.DB = db
	}
}

// WithDB sets a *storm.DB directly on our TCPForwarder.
func WithDB(db *storm.DB) Opt {
	return func(fwdr *TCPForwarder) {
		fwdr.DB = db
	}
}

// WithAddr sets the ip:port our TCPForwarder will listen on. Has
// no effect if used in conjunction with WithListener.
func WithAddr(addr string) Opt {
	return func(fwdr *TCPForwarder) {
		fwdr.Addr = addr
	}
}

// WithLogger sets our logger.
func WithLogger(logger *logrus.Logger) Opt {
	return func(fwdr *TCPForwarder) {
		fwdr.logger = logger
	}
}

// WithProxyClient sets our grpc server.ProxyClient.
func WithProxyClient(pc server.ProxyClient) Opt {
	return func(fwdr *TCPForwarder) {
		fwdr.C = pc
	}
}

// WithListener sets our TCPForwarder's net.Listener.
func WithListener(l net.Listener) Opt {
	return func(fwdr *TCPForwarder) {
		fwdr.L = l
	}
}

// TCPForwarder is our actual listener type that clients will connect to. This
// implementation then inspects the requests that come in on connections, and
// selects an appropriate backend by talking gRPC to a Proxy instance via C, its
// embedded server.ProxyClient.
type TCPForwarder struct {
	C      server.ProxyClient
	L      net.Listener
	logger *logrus.Logger
	DB     *storm.DB
	Addr   string
}

// GetCertificate fetches tls.Certificate from the database for
// each connection. This lets us dynamically fetch certs.
func (f *TCPForwarder) GetCertificate(hi *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host := hi.ServerName
	var bd BackendData
	err := f.DB.One("Domain", host, &bd)
	if err != nil {
		if err == storm.ErrNotFound {
			return nil, fmt.Errorf("%s no found", host)
		}
		f.logger.Error(err)
		return nil, err
	}

	cert, err := tls.X509KeyPair(bd.BackendCert, bd.BackendKey)
	return &cert, err
}

// Start accepts TCP connections.
func (f *TCPForwarder) Start() error {
	if f.DB == nil {
		return errors.New("database is nil")
	}
	// If we did not have a listener set directly, spin one up.
	// This is the normal path, because we do not set a listener
	// in main.go, currently.
	if f.L == nil {
		var tlsConf tls.Config
		tlsConf.GetCertificate = f.GetCertificate
		tlsConf.NextProtos = []string{"h2"}
		lis, err := tls.Listen("tcp", f.Addr, &tlsConf)
		if err != nil {
			return err
		}
		f.L = lis
	}
	go func() {
		for {
			conn, err := f.L.Accept()
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Temporary() {
					f.logger.Errorf("accept err (temporary): %v", err)
					continue
				}
				f.logger.Errorf("accept err: %v", err)
				return
			}
			ctx := context.Background()
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			go f.handleConn(ctx, conn)
		}
	}()

	return nil
}

// Where all the fun happens!
func (f *TCPForwarder) handleConn(ctx context.Context, conn net.Conn) {
	_, done := context.WithCancel(ctx)
	defer done()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	// bufForBackend collects all the connection's reads until we select a backend,
	// then we write all of bufForBackend's contents to the backend conn before
	// tunneling the rest of the bytes through.
	bufForBackend := bytes.NewBuffer([]byte(""))
	// tee is how we copy/collect our initial reads of conn into bufForBackend
	tee := io.TeeReader(conn, bufForBackend)
	prefaceBytes := make([]byte, len([]byte(http2.ClientPreface)))
	n, err := tee.Read(prefaceBytes)
	if err != nil && err != io.EOF {
		// EOF is not an error here?
		f.logger.Errorf("first read: %v", err)
		return
	}
	// prefaceBuffer can be eliminated if we make hasHTTP2Preface do an exact
	// byte-for-byte match instead of using an io.Reader
	prefaceBuffer := bytes.NewReader(prefaceBytes[:n])

	var matched BackendData
	var found []BackendData
	if hasHTTP2Preface(prefaceBuffer) {
		headers := gatherHTTP2Headers(tee)
		query := f.DB.Select(
			q.In("Protocol", []server.Backend_Protocol{
				server.Backend_GRPC,
				server.Backend_HTTP2,
			}),
			q.Eq("Domain", HostWithoutPort(headers[":authority"])),
		)
		err := query.Find(&found)
		if err != nil {
			f.logger.Error(err)
			conn.Close()
			return
		}
		matched = found[0]

		//for _, backend := range found {
		//	if MatchHeaders(headers, backend.MatchHeaders) {
		//		matched = backend
		//		break
		//	}
		//}
	} else {
		partial := make([]byte, 4096)
		n, err := tee.Read(partial)
		if err != nil && err != io.EOF {
			f.logger.Errorf("http1 error: %v", err)
			conn.Close()
			return
		}
		joined := bytes.Join([][]byte{bufForBackend.Bytes(), partial[:n]}, []byte(""))
		var host string
		lines := strings.Split(string(joined), "\r\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Host:") {
				host = strings.TrimSpace(strings.Split(line, ":")[1])
			}
		}
		query := f.DB.Select(q.Eq("Domain", HostWithoutPort(host)))
		err = query.Find(&found)
		if err != nil {
			f.logger.Errorf("http1 query error: %v", err)
			conn.Close()
			return
		}
		matched = found[0]
	}
	// expect only one found, for now
	if len(found) != 1 {
		f.logger.Error("no backends found")
		conn.Close()
		return
	}

	if len(matched.IPs) < 1 {
		f.logger.Errorf("backend %s has no configured IPs", matched.Domain)
		conn.Close()
		return
	}
	f.logger.Debugf("dialing backend: %v", matched.IPs[0])
	// TODO when we dial backend, we need to set HTTP2 connection params
	bConn, err := tls.Dial("tcp", matched.IPs[0], &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		f.logger.Errorf("dial backend: %v", err)
		return
	}
	bConn.SetDeadline(time.Now().Add(3 * time.Second))
	// our first backend write is the little buffer we read
	// from the incoming conn, by writing here we
	// pass it upstream after we've inspected it.
	_, err = bConn.Write(bufForBackend.Bytes())
	if err != nil {
		f.logger.Errorf("first write to backend: %v", err)
		conn.Close()
		bConn.Close()
		return
	}

	var t = Tunnel{
		ErrorState:  nil,
		ErrorSig:    make(chan error),
		ServerConn:  conn,
		BackendConn: bConn,
	}

	go t.pipe(conn, bConn, "conn->bConn")
	go t.pipe(bConn, conn, "bConn->conn")
	f.logger.Debug("waiting")
	err = <-t.ErrorSig
	f.logger.Debugf("closing conns: %v", err)
	bConn.Close()
	conn.Close()
}

func tryAllMatchers(r io.Reader, matchers []cmux.Matcher) bool {
	var goodSoFar bool
	for _, matcher := range matchers {
		if matcher(r) {
			goodSoFar = true
		} else {
			return false
		}
	}
	return goodSoFar
}

// Stop ...
func (f *TCPForwarder) Stop() error {
	return f.L.Close()
}

func (t *Tunnel) pipe(src, dst net.Conn, dir string) {

	buff := make([]byte, 0xffff)
	for {
		if t.ErrorState != nil {
			return
		}
		n, err := src.Read(buff)
		if err != nil {
			t.err(fmt.Errorf("%s read: %v", dir, err))
			return
		}
		b := buff[:n]

		n, err = dst.Write(b)
		if err != nil {
			t.err(fmt.Errorf("%s write: %v", dir, err))
			return
		}
	}
}

// A Tunnel streams data between two conns.
type Tunnel struct {
	ServerConn  net.Conn
	BackendConn net.Conn
	ErrorState  error
	ErrorSig    chan error
}

func (t *Tunnel) err(err error) {
	t.ErrorState = err
	t.ErrorSig <- err
}

// NewTCPForwarderFromGRPCClient ...
func NewTCPForwarderFromGRPCClient(l net.Listener, pc server.ProxyClient, db *storm.DB, logger *logrus.Logger) *TCPForwarder {
	return &TCPForwarder{
		C:      pc,
		L:      l,
		logger: logger,
		DB:     db,
	}
}

func certFromFile(path string) ([][]byte, *x509.Certificate, error) {
	data, err := ioutil.ReadFile(path)
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, nil, errors.New("invalid public key")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, err
	}
	ret := make([][]byte, 1)
	ret = append(ret, cert.Raw)
	return ret, cert, nil
}

func privateKeyFromFile(path string) (*rsa.PrivateKey, error) {

	privateKey, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	priv, _ := pem.Decode(privateKey)

	return x509.ParsePKCS1PrivateKey(priv.Bytes)
}

type TimedRecord struct {
	TS   []byte
	Data trace.Record
}

func debugRequest(req *http.Request) {
	data, _ := httputil.DumpRequest(req, false)
	fmt.Printf("%s\n\n", string(data))
}

// adapted from cmux
func gatherHTTP2Headers(r io.Reader) map[string]string {

	headers := make(map[string]string)

	done := false
	// w, r

	framer := http2.NewFramer(ioutil.Discard, r)
	hdec := hpack.NewDecoder(uint32(4<<16), func(hf hpack.HeaderField) {
		headers[hf.Name] = hf.Value
	})
	for {
		f, err := framer.ReadFrame()
		if err != nil {
			return nil
		}

		switch f := f.(type) {
		case *http2.SettingsFrame:
			// Sender acknoweldged the SETTINGS frame. No need to write
			// SETTINGS again.
			if f.IsAck() {
				break
			}
			if err := framer.WriteSettings(); err != nil {
				return nil
			}
		case *http2.ContinuationFrame:
			if _, err := hdec.Write(f.HeaderBlockFragment()); err != nil {
				return nil
			}
			done = done || f.FrameHeader.Flags&http2.FlagHeadersEndHeaders != 0
		case *http2.HeadersFrame:
			if _, err := hdec.Write(f.HeaderBlockFragment()); err != nil {
				return nil
			}
			done = done || f.FrameHeader.Flags&http2.FlagHeadersEndHeaders != 0
		case *http2.WindowUpdateFrame:
			// TODO do we need to write this?
			//err = framer.WriteWindowUpdate(f.StreamID, f.Increment)
			//if err != nil {
			//	fmt.Println("window update err", err)
			//}
		}

		if done {
			return headers
		}
	}
}

func hasHTTP2Preface(r io.Reader) bool {
	var b [len(http2.ClientPreface)]byte
	last := 0

	for {
		n, err := r.Read(b[last:])
		if err != nil {
			return false
		}

		last += n
		eq := string(b[:last]) == http2.ClientPreface[:last]
		if last == len(http2.ClientPreface) {
			return eq
		}
		if !eq {
			return false
		}
	}
}

// HostWithoutPort extracts a hostname from an request, omitting
// any ":PORT" portion, if present. This is the value from the "Host:" header
// in HTTP1, or the ":authority" header in HTTP2.
func HostWithoutPort(s string) string {
	if strings.Contains(s, ":") {
		return strings.Split(s, ":")[0]
	}
	return s
}

// MatchHeaders compares headers from an HTTP2 request to values in our database.
func MatchHeaders(fromReq, fromDB map[string]string) bool {
	var matched = false
	for k, dbVal := range fromDB {
		if headerVal, ok := fromReq[k]; ok && headerVal == dbVal {
			matched = true
		} else {
			// return early if we get a different value
			return false
		}
	}
	return matched
}
