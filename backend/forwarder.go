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
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/vulcand/oxy/forward"
	"github.com/vulcand/oxy/testutils"
	"github.com/vulcand/oxy/trace"
	"gitlab.com/DSASanFrancisco/co-chair/proto/server"
	"google.golang.org/grpc"
)

// ProxyForwarder is our type that actually handles connections from the
// internet that want to proxy to services behind co-chair.
type ProxyForwarder struct {
	fwd         *forward.Forwarder
	logger      *logrus.Logger
	metrics     chan *bytes.Buffer
	metricsStop chan bool
	// ProxyClient is itself an interface, so we do not use a pointer here
	// in our struct definition. But if a pointer to a concrete implementation
	// is passed, "c" will still be a pointer/reference, so we can share the
	// client connection with other ProxyForwarder methods.
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
func (pf *ProxyForwarder) GetConfigForClient(hello *tls.ClientHelloInfo) (*tls.Config, error) {
	cert, err := pf.GetCertificate(hello)
	if err != nil {
		return nil, err
	}
	var conf tls.Config
	conf.Certificates = []tls.Certificate{*cert}
	return &conf, nil
}

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
