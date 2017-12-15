package backend

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"gitlab.com/DSASanFrancisco/co-chair/proto/server"
	"google.golang.org/grpc"
)

func TestProxyForwarder(t *testing.T) {

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
			"server1",
			[]string{server1.Addr},
		},
		{
			"server2",
			[]string{server2.Addr},
		},
		{
			"server3",
			[]string{server3.Addr},
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

			c := http.DefaultClient
			url := fmt.Sprintf("http://%s:42069", tc.name)
			req, _ := http.NewRequest("GET", url, nil)
			resp, err := c.Do(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			if resp.StatusCode != tc.code {
				t.Errorf("expected %v got %v", tc.code, resp.StatusCode)
			}
		})
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

type FakeServer struct {
	lis  net.Listener
	srv  *http.Server
	Addr string
}

func (f *FakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//w.WriteHeader(f.code)
	return
}
func (f *FakeServer) Start() {
	go func() {
		f.srv.Serve(f.lis)
	}()
}

func (f *FakeServer) Stop() {
	f.srv.Shutdown(context.TODO())
}
