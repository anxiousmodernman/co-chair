package backend

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
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
	"github.com/vulcand/oxy/trace"
)

// NewTCPForwarder ...
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

// TCPForwarder ...
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
	// If we did not have a listener set directly, spin one up
	if f.L == nil {
		var tlsConf tls.Config
		tlsConf.GetCertificate = f.GetCertificate
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

	// look up the domain in the db
	var bd BackendData
	err = f.DB.One("Domain", host, &bd)
	if err != nil {
		if err == storm.ErrNotFound {
			// This should never happen, since our GetCertificate implemenation
			// has already performed this exact query, but hey.
			f.logger.Debug("backend not found: ", host)
			conn.Close()
			return
		}
		f.logger.Error(err)
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
