package backend

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/vulcand/oxy/forward"
	"github.com/vulcand/oxy/testutils"
	"gitlab.com/DSASanFrancisco/co-chair/proto/server"
	"google.golang.org/grpc"
)

// ProxyForwarder is our type that actually handles connections from the
// internet that want to proxy to services behind co-chair.
type ProxyForwarder struct {
	fwd    *forward.Forwarder
	logger *logrus.Logger
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
	splitted := strings.Split(dom, ":")
	fmt.Println("splitted:", splitted)

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
				r.URL = testutils.ParseURI("http://" + upstreams.Backends[0].Ips[0])
				pf.logger.Infof("proxying %s -> %s", dom, r.Host)
				pf.fwd.ServeHTTP(w, r)
				// we MUST return early
				return
			}
		}
	}
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(fmt.Sprintf("upstream %s not found", dom)))
	return
}

func orElse(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
