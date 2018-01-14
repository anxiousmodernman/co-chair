package backend

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anxiousmodernman/co-chair/proto/server"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func TestTCPProxyForwarder(t *testing.T) {

	_, pc, cleanup := grpcListenerClientCleanup()
	defer cleanup()

	l, _ := net.Listen("tcp", "0.0.0.0:0")

	fwd := NewTCPForwarderFromGRPCClient(l, pc, logrus.New())
	if fwd == nil {
	}

	ca, capriv, err := createCA()
	if err != nil {
		t.Fatal(err)
	}

	// cert scenarios
	// always frontend cert
	// if no backend cert: http
	// if backend cert: https
	s1Cert, s1Key, err := createSignedCert("server1", ca, capriv)
	if err != nil {
		t.Fatal(err)
	}
	s1, err := newTestServer(200, s1Cert, s1Key, ca)
	if err != nil {
		t.Fatal(err)
	}
	s2Cert, s2Key, err := createSignedCert("server2", ca, capriv)
	s2, err := newTestServer(202, s2Cert, s2Key, ca)
	if err != nil {
		t.Fatal(err)
	}

	s1.StartTLS()
	defer s1.Close()
	s2.StartTLS()
	defer s2.Close()

	go fwd.Start()
	defer fwd.Stop()

	// make a TLS client

}

func newTestServer(code int, myCert, myPriv, caCert []byte) (*httptest.Server, error) {
	s := httptest.NewUnstartedServer(NewTestHandler(code))
	certs := x509.NewCertPool()
	certs.AppendCertsFromPEM(caCert)
	cert, err := tls.X509KeyPair(myCert, myPriv)
	if err != nil {
		return nil, err
	}
	s.TLS = &tls.Config{
		RootCAs:      certs,
		Certificates: []tls.Certificate{cert},
	}

	return s, nil
}

func TestProxyForwarder(t *testing.T) {

	// TODO: refactor or remove.
	t.Skip("this test is deprecated")

	// Testing ProxyForwarder
	// * create listeners for: proxy, grpcMgmtApi; a shared *Proxy between them
	_, pc, cleanup := grpcListenerClientCleanup()
	defer cleanup()
	// * create independent listeners on random ports
	server1 := NewFakeServer(200)
	server2 := NewFakeServer(401)
	server3 := NewFakeServer(405)
	server1.Start()
	defer server1.Stop()
	server2.Start()
	defer server2.Stop()
	server3.Start()
	defer server3.Stop()
	// * send http requests to proxy,
	fwd, _ := NewProxyForwarderFromGRPCClient(pc, logrus.New())

	// hardcoded port means our "proxied" requests in our test
	// cases need to find this port
	httpProxy := &http.Server{
		Addr:    ":42069",
		Handler: fwd,
	}

	go func() { httpProxy.ListenAndServe() }()
	defer httpProxy.Shutdown(context.TODO())

	// Backend is a domain:port and a slice of ip addresses
	backends := []server.Backend{
		{
			Domain: "server1",
			Ips:    []string{server1.Addr},
		},
		{
			Domain: "server2",
			Ips:    []string{server2.Addr},
		},
		{
			Domain: "server3",
			Ips:    []string{server3.Addr},
		},
	}

	for _, b := range backends {
		_, err := pc.Put(context.TODO(), &b)
		if err != nil {
			t.Fatalf("backend setup err: %v", err)
		}
	}

	cases := []struct {
		name string
		code int
	}{
		{
			name: "server1",
			code: 200,
		},
		{
			name: "server2",
			code: 401,
		},
		{
			name: "server3",
			code: 405,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			url := fmt.Sprintf("http://%s:42069", tc.name)
			req, _ := http.NewRequest("GET", url, nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			if resp.StatusCode != tc.code {
				t.Errorf("expected %v got %v", tc.code, resp.StatusCode)
			}
		})
	}

	t.Log("remove server2")
	_, err := pc.Remove(context.TODO(), &backends[1])
	if err != nil {
		t.Errorf("remove: %v", err)
	}

	t.Log("server2 now returns 404")
	req, _ := http.NewRequest("GET", "http://server2:42069", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 got %v", resp.StatusCode)
	}
}

func grpcListenerClientCleanup() (*grpc.Server, server.ProxyClient, func()) {
	// Proxy, our concrent implementation
	dir, _ := ioutil.TempDir("", "co-chair-test")
	dbPath := filepath.Join(dir, "co-chair-test.db")
	px, _ := NewProxy(dbPath)

	// grpc server setup
	gs := grpc.NewServer()
	server.RegisterProxyServer(gs, px)
	// tcp listener on a random high port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	go func() { gs.Serve(lis) }()

	// grpc client setup; use the address of server to make new conn
	addr := lis.Addr().String()
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(3*time.Second))
	if err != nil {
		panic(err)
	}
	// ProxyClient, our generated client interface
	pc := server.NewProxyClient(conn)

	// stop grpc server, remove the Proxy's temp db
	cleanup := func() {
		gs.Stop()
		os.RemoveAll(dir)
	}

	return gs, pc, cleanup
}

// NewFakeServer sets up an http.Server that will only respond with the provided
// response code. Useful for tests.
func NewFakeServer(respCode int) *FakeServer {

	h := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(respCode)
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	srv := http.Server{
		Addr:    lis.Addr().String(),
		Handler: http.HandlerFunc(h),
	}

	return &FakeServer{lis, &srv, lis.Addr().String()}
}

func NewTestHandler(respCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(respCode)
	}
}

type FakeServer struct {
	lis  net.Listener
	srv  *http.Server
	Addr string
}

func (f *FakeServer) Start() {
	go func() {
		f.srv.Serve(f.lis)
	}()
}

func (f *FakeServer) Stop() {
	f.srv.Shutdown(context.TODO())
}

// returns ca cert, private key, and error
func createCA() ([]byte, []byte, error) {
	// The newly-generated RSA key priv has a public key field as well.
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %s", err)
	}
	template := x509.Certificate{
		SerialNumber: serialNumber, // big.Int
		Subject: pkix.Name{
			CommonName:   "Test Org CA",
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Duration(10) * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"server1,server2,server3"},
	}
	// the 3rd param is the "parent" cert; in this case, parent is the same as the 2nd param, so
	// the new cert is self-signed. Priv must always be the private key of the signer.
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("create cert: %v", err)
	}

	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	// the root CA has to be distributed to all clients and servers
	return pemCert, pemKey, nil
}

func createSignedCert(name string, parentCert, parentPrivateKey []byte) ([]byte, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %s", err)
	}
	template := x509.Certificate{
		SerialNumber: serialNumber, // big.Int
		Subject: pkix.Name{
			CommonName:   name,
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Duration(10) * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{name},
	}
	blck, rest := pem.Decode(parentCert)
	if len(rest) > 0 {
		panic("expected exactly one pem-encoded block")
	}
	ca, err := x509.ParseCertificate(blck.Bytes)
	if err != nil {
		return nil, nil, err
	}

	blck, rest = pem.Decode(parentPrivateKey)
	if len(rest) > 0 {
		panic("expected exactly one pem-encoded block")
	}

	parentPriv, err := x509.ParsePKCS1PrivateKey(blck.Bytes)
	if err != nil {
		return nil, nil, err
	}

	// the new damn cert
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, ca, &priv.PublicKey, parentPriv)
	if err != nil {
		return nil, nil, fmt.Errorf("create cert: %v", err)
	}

	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return pemCert, pemKey, nil
}

func newTestHTTPSClient(rootCA []byte) *http.Client {
	certs := x509.NewCertPool()
	certs.AppendCertsFromPEM(rootCA)
	tlsConf := &tls.Config{RootCAs: certs}

	c := &http.Client{}
	tpt := &http.Transport{
		TLSClientConfig: tlsConf,
	}
	c.Transport = tpt
	return c
}
