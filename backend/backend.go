package backend

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gitlab.com/DSASanFrancisco/co-chair/proto/server"
	"github.com/asdine/storm"
)

// Backend should be used to implement the server interface
// exposed by the generated server proto.
type Backend struct {
}

// Ensure struct implements interface
var _ server.BackendServer = (*Backend)(nil)

// Proxy is our server.ProxyServer implementation.
type Proxy struct {
	DB *storm.DB
}

// NewProxy is our proxy constructor.
func NewProxy(path string) (*Proxy, error) {
	db, err := storm.Open(path)
	if err != nil {
		return nil, err
	}
	return &Proxy{db}, nil
}

// assert that Proxy is a server.ProxyServer at compile time.
var _ server.ProxyServer = (*Proxy)(nil)

// State returns the state of the proxy. The number of backends returned is
// controlled by the domain field of the request. A blank domain returns all.
func (p *Proxy) State(_ context.Context, req *server.StateRequest) (*server.ProxyState, error) {
	var resp server.ProxyState
	var backends []*BackendData
	var err error
	if req.Domain == "" {
		err = p.DB.All(&backends)
	} else {
		err = p.DB.Find("Domain", req.Domain, &backends)
	}
	if err != nil {
		return nil, fmt.Errorf("db error: %v", err)
	}
	for _, b := range backends {
		resp.Backends = append(resp.Backends, b.AsBackendT())
	}

	return &resp, nil
}

// Put adds a backend to our pool of proxied Backends.
func (p *Proxy) Put(ctx context.Context, b *server.BackendT) (*server.OpResult, error) {
	var bd BackendData
	err := p.DB.One("Domain", b.Domain, &bd)

	if err != nil {
		if err == storm.ErrNotFound {
			// do nothing, always overwrite
		} else {
			return &server.OpResult{}, errors.New("")
		}
	}
	bd.Domain = b.Domain
	bd.IPs = combine(bd.IPs, b.Ips)

	err = p.DB.Save(&bd)
	if err != nil {
		return nil, fmt.Errorf("save: %v", err)
	}

	resp := &server.OpResult{200, "Ok"}

	return resp, nil
}

func combine(a, b []string) []string {
	// let's pre-allocate enough space
	both := make([]string, 0, len(a)+len(b))
	both = append(both, a...)
	both = append(both, b...)
	sort.Strings(both)
	var val string
	var res []string
	for _, x := range both {
		if strings.TrimSpace(x) == strings.TrimSpace(val) {
			continue
		}
		val = x
		res = append(res, strings.TrimSpace(x))
	}
	return res
}

// Remove ... TODO
func (p *Proxy) Remove(context.Context, *server.BackendT) (*server.OpResult, error) { return nil, nil }

// BackendData is our type for the storm ORM. We can define field-level
// constraints and indexes on struct tags.
type BackendData struct {
	ID     int    `storm:"id,increment"`
	Domain string `storm:"unique"`
	IPs    []string
}

// AsBackendT is a conversion method to a grpc-sendable type.
func (bd BackendData) AsBackendT() *server.BackendT {
	var b server.BackendT
	b.Domain = bd.Domain
	b.Ips = bd.IPs
	return &b
}

func (bd BackendData) PutIP(ip string) error {

	return nil
}
