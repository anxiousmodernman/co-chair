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
	"regexp"
	"testing"
	"time"

	"github.com/anxiousmodernman/co-chair/proto/server"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func TestTCPProxyForwarder(t *testing.T) {
	os.TempDir()
	f, _ := ioutil.TempFile("", "tempdb")
	f.Close()

	svr, pc, cleanup := grpcListenerClientCleanup()
	defer cleanup()
	_ = svr // Serve() has already been called

	ca, capriv, err := createCA()
	if err != nil {
		t.Fatal(err)
	}

	s1Cert, s1Key, err := createSignedCert("server1", ca, capriv)
	if err != nil {
		t.Fatal(err)
	}
	s1, err := newTestServer(201, s1Cert, s1Key, ca)
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

	// Wacky setup because we need the GetCertificate method implementation
	// on fwd, our pointer receiver.
	fwd, err := NewTCPForwarder(
		WithDB(svr.DB),
		WithLogger(logrus.New()),
	)
	if err != nil {
		t.Fatal(err)
	}
	var proxyTLSConf tls.Config
	proxyTLSConf.GetCertificate = fwd.GetCertificate
	// proxyTLSConf := newProxyTLSConfig(ca, s1Cert, s1Key, s2Cert, s2Key)

	l, _ := tls.Listen("tcp", "0.0.0.0:0", &proxyTLSConf)
	fwd.L = l

	// fwd := NewTCPForwarderFromGRPCClient(l, pc, svr.DB, logrus.New())
	go fwd.Start()
	defer fwd.Stop()

	// match port in an address that looks like [::]:12345
	re := regexp.MustCompile(`[\[\]:]+(\d+)`)
	matches := re.FindStringSubmatch(l.Addr().String())
	proxyPort := matches[1]

	c := newTestHTTPSClient(ca)
	req, err := http.NewRequest("GET", fmt.Sprintf("https://server2:%s/", proxyPort), nil)
	if err != nil {
		t.Fatal(err)
	}
	b := makeBackend("server2", s2.Listener.Addr().String(), s2Cert, s2Key)
	_, err = pc.Put(context.TODO(), b)
	if err != nil {
		t.Fatalf("could not add backend with grpc: %v", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 202 {
		t.Errorf("expected 202 got %v", resp.StatusCode)
	}
}

func makeBackend(domain, addr string, cert, key []byte) *server.Backend {
	var b server.Backend
	b.Domain = domain
	b.Ips = []string{addr}
	var c server.X509Cert
	c.Cert = cert
	c.Key = key
	b.BackendCert = &c
	return &b
}

func newProxyTLSConfig(caCert, s1cert, s1key, s2cert, s2key []byte) *tls.Config {

	ca := x509.NewCertPool()
	ca.AppendCertsFromPEM(caCert)
	cert1, err := tls.X509KeyPair(s1cert, s1key)
	if err != nil {
		panic(err)
	}
	cert2, err := tls.X509KeyPair(s2cert, s2key)
	if err != nil {
		panic(err)
	}
	conf := &tls.Config{
		RootCAs:      ca,
		Certificates: []tls.Certificate{cert1, cert2},
	}
	// We must do this to create an internal map that the Go TLS
	// implementation will use to pick the right cert for an
	// incoming TLS conn. If we do not call this method, the
	// first certificate is always chosen.
	conf.BuildNameToCertificate()
	return conf
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

func grpcListenerClientCleanup() (*Proxy, server.ProxyClient, func()) {
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

	return px, pc, cleanup
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
		fmt.Println("RESPONSE CODE?", respCode)
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
		IsCA:         true,         // true, else we fail at runtime :(
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
