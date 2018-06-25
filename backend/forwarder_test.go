package backend

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/anxiousmodernman/co-chair/proto/server"
	ls "github.com/anxiousmodernman/localserver"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func testNewCAAndCert(t *testing.T) (*ls.CA, *ls.SignedCert) {
	ca, err := ls.NewCA()
	if err != nil {
		t.Fatal(err)
	}
	signed, err := ca.CreateSignedCert("server1")
	if err != nil {
		t.Fatal(err)
	}
	return ca, signed
}
func TestTCPProxyForwarderGRPC(t *testing.T) {
	f, _ := ioutil.TempFile("", "tempdb")
	f.Close()

	svr, pc, cleanup := grpcListenerClientCleanup()
	defer cleanup()

	ca, signed1 := testNewCAAndCert(t)

	// CA, SignedCert, Authority
	s1, err := ls.NewGRPCServer(ca, signed1, "server1")
	if err != nil {
		t.Fatal(err)
	}

	s1.ServeGRPC()
	defer s1.Stop()

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
	proxyTLSConf.NextProtos = []string{"h2"}
	l, _ := tls.Listen("tcp", "0.0.0.0:0", &proxyTLSConf)
	fwd.L = l // this sucks: refactor

	go fwd.Start()
	defer fwd.Stop()

	// match port in an address that looks like [::]:12345
	re := regexp.MustCompile(`[\[\]:]+(\d+)`)
	matches := re.FindStringSubmatch(l.Addr().String())
	proxyPort := matches[1]

	b := makeBackend(server.Backend_HTTP2, "server1", s1.Lis.Addr().String(), signed1.Cert, signed1.PrivateKey)
	if _, err := pc.Put(context.TODO(), b); err != nil {
		t.Fatalf("could not add backend with grpc: %v", err)
	}

	// need to put this after makeBackend?
	c, err := ls.NewGRPCClientForServer(ca, "server1:"+proxyPort)
	if err != nil {
		t.Fatal(err)
	}
	key := ls.Key{"key"}
	resp, err := c.Get(context.TODO(), &key)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Value != key.Key {
		t.Errorf("expected %s got %s", key.Key, resp.Value)
	}
	stream, err := c.GetKVStream(context.TODO(), &key)
	if err != nil {
		t.Fatalf("get kv stream: %v", err)
	}
	for _, expected := range []string{"1", "2", "3"} {
		kv, err := stream.Recv()
		if err != nil {
			t.Errorf("recv: %v", err)
		}
		if kv.Value != expected {
			t.Errorf("expected: %v, got: %s", expected, kv.Value)
		}
	}
	putStream, err := c.PutKVStream(context.TODO())
	for _, put := range []string{"1", "2", "3"} {
		if err := putStream.Send(&ls.KV{put, put}); err != nil {
			t.Fatalf("put stream: %v", err)
		}
	}
	opResult, err := putStream.CloseAndRecv()
	if opResult.ErrCode != 0 {
		t.Fatal("got non-zero OpResult from put stream")
	}

}

func TestTCPProxyForwarderHTTP2(t *testing.T) {
	f, _ := ioutil.TempFile("", "tempdb")
	f.Close()

	svr, pc, cleanup := grpcListenerClientCleanup()
	defer cleanup()

	ca, signed1 := testNewCAAndCert(t)

	s1 := ls.NewLocalServer(ls.NewTestHandler(201), signed1, ca)

	s1.StartHTTP2()
	defer s1.Stop()

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
	proxyTLSConf.NextProtos = []string{"h2"}
	l, _ := tls.Listen("tcp", "0.0.0.0:0", &proxyTLSConf)
	fwd.L = l // this sucks: refactor

	go fwd.Start()
	defer fwd.Stop()

	// match port in an address that looks like [::]:12345
	re := regexp.MustCompile(`[\[\]:]+(\d+)`)
	matches := re.FindStringSubmatch(l.Addr().String())
	proxyPort := matches[1]

	c := ls.NewHTTP2Client(ca)
	req, err := http.NewRequest("GET", fmt.Sprintf("https://server1:%s/", proxyPort), nil)
	if err != nil {
		t.Fatal(err)
	}
	b := makeBackend(server.Backend_HTTP2, "server1", s1.Lis.Addr().String(), signed1.Cert, signed1.PrivateKey)
	if _, err := pc.Put(context.TODO(), b); err != nil {
		t.Fatalf("could not add backend with grpc: %v", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		t.Errorf("expected 202 got %v", resp.StatusCode)
	}
}

func TestTCPProxyForwarderHTTP1(t *testing.T) {
	f, _ := ioutil.TempFile("", "tempdb")
	f.Close()

	svr, pc, cleanup := grpcListenerClientCleanup()
	defer cleanup()

	ca, signed1 := testNewCAAndCert(t)

	s1 := ls.NewLocalServer(ls.NewTestHandler(201), signed1, ca)

	s1.StartHTTP1()
	defer s1.Stop()

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
	proxyTLSConf.NextProtos = []string{"h2"}
	l, _ := tls.Listen("tcp", "0.0.0.0:0", &proxyTLSConf)
	fwd.L = l // this sucks: refactor

	go fwd.Start()
	defer fwd.Stop()

	// match port in an address that looks like [::]:12345
	re := regexp.MustCompile(`[\[\]:]+(\d+)`)
	matches := re.FindStringSubmatch(l.Addr().String())
	proxyPort := matches[1]

	c := ls.NewHTTP1Client(ca)
	req, err := http.NewRequest("GET", fmt.Sprintf("https://server1:%s/", proxyPort), nil)
	if err != nil {
		t.Fatal(err)
	}
	b := makeBackend(server.Backend_HTTP1, "server1", s1.Lis.Addr().String(), signed1.Cert, signed1.PrivateKey)
	if _, err := pc.Put(context.TODO(), b); err != nil {
		t.Fatalf("could not add backend with grpc: %v", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		t.Errorf("expected 202 got %v", resp.StatusCode)
	}
}

func TestExtractHostHeader(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"server1:37739", "server1"},
		{"server1", "server1"},
	}

	for _, c := range cases {
		if got := HostWithoutPort(c.input); got != c.expected {
			t.Errorf("input: %s expected: %s got: %s", c.input, c.expected, got)
		}
	}
}

func makeBackend(protocol server.Backend_Protocol, domain, addr string, cert, key []byte) *server.Backend {

	var b server.Backend
	b.Protocol = protocol
	b.Domain = domain
	b.Ips = []string{addr}

	var c server.X509Cert
	c.Cert = cert
	c.Key = key
	b.BackendCert = &c
	headers := make(map[string]string)
	headers["Host"] = domain
	b.MatchHeaders = headers

	return &b
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
