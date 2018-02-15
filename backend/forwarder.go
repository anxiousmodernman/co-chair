package backend

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/anxiousmodernman/co-chair/proto/server"
	"github.com/asdine/storm"
	"github.com/sirupsen/logrus"
	"github.com/vulcand/oxy/forward"
	"github.com/vulcand/oxy/testutils"
	"github.com/vulcand/oxy/trace"
	"google.golang.org/grpc"
)

// NewTCPForwarder ...
func NewTCPForwarder(certPath, keyPath, port string) (net.Listener, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	conf := tls.Config{Certificates: []tls.Certificate{cert}}
	addr := fmt.Sprintf("0.0.0.0:%s", port)
	return tls.Listen("tcp", addr, &conf)
}

// An Opt lets us set values on a fwdConf.
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

// TCPForwarder ...
type TCPForwarder struct {
	C      server.ProxyClient
	L      net.Listener
	logger *logrus.Logger
	DB     *storm.DB
}

// Start accepts TCP connections.
func (f *TCPForwarder) Start() error {
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

func (f *TCPForwarder) handleConn(ctx context.Context, conn net.Conn) {
	_, done := context.WithCancel(ctx)
	defer done()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		f.logger.Errorf("first read: %v", err)
		return
	}
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	var host string
	lines := strings.Split(string(buf[:n]), "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Host:") {
			host = strings.TrimSpace(strings.Split(line, ":")[1])
		}
	}
	fmt.Println("time for a lookup")
	if f.DB == nil {
		fmt.Println("DB IS NIL")
	}

	// look up the domain in the db
	var bd BackendData
	err = f.DB.One("Domain", host, &bd)
	if err != nil {
		if err == storm.ErrNotFound {
			fmt.Println("NOT FOUND", err)
			tlsconn, ok := conn.(*tls.Conn)
			if ok {
				fmt.Println("we are tls")
				tlsconn.Write([]byte("HTTP/2.0 404 Not Found\r\n\r\n\r\n"))
			} else {
				conn.Write([]byte("HTTP/2.0 404 Not Found\r\n"))
			}

			f.logger.Debug("backend not found: ", host)
			conn.Close()
			return
		}
		f.logger.Error(err)
		fmt.Println("ERROR", err)
		conn.Close()
		return
	}
	if len(bd.IPs) < 1 {
		f.logger.Errorf("backend %s has no configured IPs", host)
		conn.Close()
		return
	}
	f.logger.Debugf("dialing backend: %v", bd.IPs[0])
	bConn, err := tls.Dial("tcp", bd.IPs[0], &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		f.logger.Errorf("dial backend: %v", err)
		return
	}
	bConn.SetDeadline(time.Now().Add(3 * time.Second))
	// our first backend write is the little buffer we read
	// from the incoming conn, by writing here we
	// pass it upstream after we've inspected it.
	_, err = bConn.Write(buf[:n])
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
	fmt.Println("waiting now")
	f.logger.Debug("waiting")
	err = <-t.ErrorSig
	f.logger.Debugf("closing conns: %v", err)
	bConn.Close()
	conn.Close()
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

// ProxyForwarder is our type that actually handles connections from the
// internet that want to proxy to services behind co-chair.
type ProxyForwarder struct {
	fwd         *forward.Forwarder
	logger      *logrus.Logger
	metrics     chan *bytes.Buffer
	metricsStop chan bool
	// ProxyClient is a generated gRPC interface
	c server.ProxyClient
}

// NewProxyForwarder is our constructor for ProxyForwarder. It needs a client
// to the grpc API, so that it can inspect the state of the service.
func NewProxyForwarder(apiAddr string, logger *logrus.Logger) (*ProxyForwarder, error) {
	var pf ProxyForwarder
	pf.logger = logger
	pf.metrics = make(chan *bytes.Buffer, 100)
	conn, err := grpc.Dial(apiAddr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %v", err)
	}
	pf.c = server.NewProxyClient(conn)
	fwd, err := forward.New()
	if err != nil {
		return nil, err
	}
	pf.fwd = fwd

	return &pf, nil
}

// NewProxyForwarderFromGRPCClient is a constructor for ProxyForwarder that takes
// a pre-configured server.ProxyClient as its connection to the management api.
func NewProxyForwarderFromGRPCClient(pc server.ProxyClient, logger *logrus.Logger) (*ProxyForwarder, error) {
	var pf ProxyForwarder
	pf.logger = logger
	pf.c = pc
	fwd, err := forward.New()
	if err != nil {
		return nil, err
	}
	pf.fwd = fwd
	return &pf, nil
}

// ServeHTTP implements the standard library's http.Handler interface.
func (pf *ProxyForwarder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dom := orElse(r.Host, r.URL.Host)
	if pf.c == nil {
		w.WriteHeader(500)
		w.Write([]byte("client is nil"))
		return
	}

	// Hmm... https://github.com/vulcand/oxy/issues/57#issuecomment-286491548
	r.URL.Opaque = ""

	splitted := strings.Split(dom, ":")

	req := &server.StateRequest{Domain: splitted[0]}
	upstreams, err := pf.c.State(context.Background(), req)
	if err != nil {
		pf.logger.Errorf("splitted domain: %s db error: %v", splitted, err)
	}
	if upstreams != nil {
		if len(upstreams.Backends) == 1 {
			if len(upstreams.Backends[0].Ips) == 1 {
				// we assert exactly one domain -> ip mapping
				// only because we do not support load balancing yet
				// Note: field Host must be host or host:port
				// Also note: merely setting fields on the request is all the oxy
				// library needs to forward the request.

				//r.URL = testutils.ParseURI(protocolFmt(r) + upstreams.Backends[0].Ips[0])
				be := upstreams.Backends[0]
				if r.TLS != nil {
					r.Header.Set(forward.XForwardedProto, "https")
				}
				r.URL = testutils.ParseURI(be.Protocol + be.Ips[0])
				pf.logger.Infof("proxying %s -> %s", dom, r.Host)

				// TODO: re-use a pool of buffers for GC optimization:
				// https://blog.cloudflare.com/recycling-memory-buffers-in-go/
				var buf bytes.Buffer
				tracer, err := trace.New(pf.fwd, &buf)
				if err != nil {
					w.WriteHeader(500)
					w.Write([]byte("could not create metrics middleware"))
				}

				tracer.ServeHTTP(w, r)
				// TODO sensible, fast metrics serialization?
				//go func() {
				//	pf.metrics <- &buf
				//}()
				return
			}
		}
	}
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(fmt.Sprintf("upstream %s not found", dom)))
	return
}

// GetConfigForClient ...
func (pf *ProxyForwarder) GetConfigForClient(hello *tls.ClientHelloInfo) (*tls.Config, error) {
	cert, err := pf.GetCertificate(hello)
	if err != nil {
		return nil, err
	}
	var conf tls.Config
	conf.Certificates = []tls.Certificate{*cert}
	return &conf, nil
}

// GetCertificate ...
func (pf *ProxyForwarder) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {

	fmt.Println("hello", hello.ServerName)

	priv, err := privateKeyFromFile("/opt/pki/dev_key.pem")
	if err != nil {
		return nil, err
	}
	crt, x509cert, err := certFromFile("/opt/pki/dev_cert.pem")
	if err != nil {
		return nil, err
	}

	cert := &tls.Certificate{
		PrivateKey:  priv,
		Certificate: crt,
		Leaf:        x509cert,
	}
	return cert, nil
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

func (pf *ProxyForwarder) startMetricsListener() {

	go func() {
		// let's tempt fate with a long-lived stream to the grpc server.
		stream, err := pf.c.PutKVStream(context.TODO())
		if err != nil {
			pf.logger.Errorf("PutKVStream: %v", err)
			return
		}
		for {
			select {
			case m := <-pf.metrics:
				var k, v []byte
				_ = k
				// deserialize m to review timestamp
				var record TimedRecord
				err = json.Unmarshal(m.Bytes(), &record)
				if err != nil {
					pf.logger.Errorf("malformed trace record: %v", err)
					continue
				}

				// TODO some inefficient re-serialization going on here.

				kv := server.KV{Key: record.TS, Value: v}
				stream.Send(&kv)
			case <-pf.metricsStop:
				return
			}
		}
	}()
}

type TimedRecord struct {
	TS   []byte
	Data trace.Record
}

func orElse(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func protocolFmt(r *http.Request) string {
	if r.TLS == nil {
		return "http://"
	}
	return "https://"
}
