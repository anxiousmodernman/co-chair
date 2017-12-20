package backend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/asdine/storm"
	"gitlab.com/DSASanFrancisco/co-chair/proto/server"
)

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
		return nil, fmt.Errorf("domain: %s; db error: %v", req.Domain, err)
	}
	for _, b := range backends {
		fmt.Println("backend:", *b)
		resp.Backends = append(resp.Backends, b.AsBackend())
	}

	return &resp, nil
}

// Put adds a backend to our pool of proxied Backends.
func (p *Proxy) Put(ctx context.Context, b *server.Backend) (*server.OpResult, error) {
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

// PutKVStream lets us stream key-value pairs into our db.
func (p *Proxy) PutKVStream(stream server.Proxy_PutKVStreamServer) error {

	for {
		kv, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if err := p.DB.SetBytes("streams", kv.Key, kv.Value); err != nil {
			return fmt.Errorf("SetBytes: %v", err)
		}
	}
	return nil
}

// GetKVStream scans a keyspace.
func (p *Proxy) GetKVStream(key *server.Key, stream server.Proxy_GetKVStreamServer) error {

	tx, err := p.DB.Bolt.Begin(false)
	if err != nil {
		return fmt.Errorf("db error: %v", err)
	}
	c := tx.Bucket([]byte("streams")).Cursor()

	// Our time range spans the 90's decade.
	// RFC3339
	//min := []byte("1990-01-01T00:00:00Z")
	//max := []byte("2000-01-01T00:00:00Z")

	// Iterate over the 90's.
	for k, v := c.Seek(key.Prefix); k != nil || false; /* could do something besides "false" */ k, v = c.Next() {
		fmt.Printf("%s: %s\n", k, v)
		kv := server.KV{Key: k, Value: v}
		err := stream.Send(&kv)
		if err != nil {
			return fmt.Errorf("send: %v", err)
		}
	}
	return nil
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
func (p *Proxy) Remove(_ context.Context, b *server.Backend) (*server.OpResult, error) {
	// match on domain name exactly
	var bd BackendData
	if err := p.DB.One("Domain", b.Domain, &bd); err != nil {
		return nil, err
	}
	if err := p.DB.DeleteStruct(&bd); err != nil {
		return nil, err
	}

	res := &server.OpResult{Code: 200, Status: fmt.Sprintf("removed: %s", bd.Domain)}
	return res, nil
}

// BackendData is our type for the storm ORM. We can define field-level
// constraints and indexes on struct tags. It is unfortunate that we
// need an intermediary type, but it seems better than going in and
// adding storm struct tags to generated code.
//
// See issue: https://github.com/golang/protobuf/issues/52
type BackendData struct {
	ID     int    `storm:"id,increment"`
	Domain string `storm:"unique"`
	IPs    []string
	// An optional endpoint we can call, expecting HTTP 200
	HealthCheck string
}

// AsBackend is a conversion method to a grpc-sendable type.
func (bd BackendData) AsBackend() *server.Backend {
	var b server.Backend
	b.Domain = bd.Domain
	b.Ips = bd.IPs
	return &b
}

func (bd BackendData) PutIP(ip string) error {

	return nil
}
