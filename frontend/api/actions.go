// Package api exports functions that update a store.
package api

import (
	"context"
	"log"

	"github.com/anxiousmodernman/co-chair/frontend/store"
	"github.com/anxiousmodernman/co-chair/proto/client"
)

// Client is a global handle to our grpc-over-websockets client.
var Client client.ProxyClient

// ProxyState calls the web.State endpoint, extracts the list of backends,
// and sets them on the store.
func ProxyState(s *store.Store, c client.ProxyClient) {

	go func() {
		var req client.StateRequest
		resp, err := c.State(context.TODO(), &req)
		if err != nil {
			log.Printf("api: %v\n", err)
			return
		}
		s.Put("proxy.list", resp.Backends)
	}()

}

func PutBackend(s *store.Store, c client.ProxyClient, b *client.Backend) {
	ctx := context.TODO()
	go func() {
		_, err := c.Put(ctx, b)
		if err != nil {
			log.Println("ERROR:", err)
			return
		}
		resp, err := c.State(ctx, &client.StateRequest{})
		if err != nil {
			log.Println("ERROR:", err)
			return
		}
		s.Put("proxy.list", resp.Backends)
	}()
}

func DeleteProxy(s *store.Store, c client.ProxyClient, domain string) {
	b := client.Backend{}
	b.Domain = domain

	go func() {
		_, err := c.Remove(context.TODO(), &b)
		if err != nil {
			log.Println("delete error:", err)
		}
		ProxyState(s, c)
	}()
}
