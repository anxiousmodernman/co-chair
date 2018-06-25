package backend

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/anxiousmodernman/co-chair/proto/server"
)

func TestCombine(t *testing.T) {

	assert := func(l []string, r ...string) {
		for i, x := range l {
			if x != r[i] {
				t.Errorf("expected %s got %s", x, r[i])
			}
		}
	}

	a := []string{"10.2.1.20", "10.2.1.21"}
	b := []string{"10.2.1.56", "10.2.1.21"}

	c := combine(a, b)
	t.Logf("combined: %v", c)

	assert(c, "10.2.1.20", "10.2.1.21", "10.2.1.56")
}

func TestProxy(t *testing.T) {
	p, cleanup, err := NewTestProxyCleanup()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	var ctx = context.TODO()

	// test proxy stuff
	var b server.Backend
	b.Domain = "harrington.io"
	b.Ips = []string{"127.0.0.2"}
	p.Put(ctx, &b)

	var sr server.StateRequest
	resp, err := p.State(ctx, &sr)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Backends) != 1 {
		t.Errorf("expected exactly one backend")
	}

	if _, err := p.Remove(ctx, &b); err != nil {
		t.Error(err)
	}
	resp, err = p.State(ctx, &sr)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("response: %v", resp)
	if len(resp.Backends) != 0 {
		t.Errorf("expected zero backends")
	}

}

// NewProxyTestCleanup returns a Proxy with a cleanup function, or an error.
func NewTestProxyCleanup() (*Proxy, func(), error) {
	dir, err := ioutil.TempDir("", "testdb")
	if err != nil {
		return nil, nil, err
	}

	p, err := NewProxy(filepath.Join(dir, "co-chair-test.db"))
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() { os.RemoveAll(dir) }

	return p, cleanup, nil
}
