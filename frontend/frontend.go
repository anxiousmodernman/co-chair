//go:generate gopherjs build frontend.go -o html/frontend.js
//go:generate bash -c "go run assets_generate.go"
package main

import (
	"strings"

	"honnef.co/go/js/dom"

	"github.com/gopherjs/gopherjs/js"
	vecty "github.com/gopherjs/vecty"
	"gitlab.com/DSASanFrancisco/co-chair/frontend/api"
	"gitlab.com/DSASanFrancisco/co-chair/frontend/components"
	"gitlab.com/DSASanFrancisco/co-chair/frontend/store"
	"gitlab.com/DSASanFrancisco/co-chair/proto/client"
)

var (
	document = dom.GetWindow().Document().(dom.HTMLDocument)
)

// InitStore initializes the state for our app.
func InitStore(s *store.Store, c client.ProxyClient) {
	// Note: we have to keep track of paths explicitly.
	s.Put("proxy.form.active", false)
	s.Put("proxy.list", make([]*client.Backend, 0))
	// Populate the store with api calls
	api.ProxyState(s, c)
}

// no-op main
func main() {}

// Ensure our setup() gets called as soon as the DOM has loaded
func init() {
	document.AddEventListener("DOMContentLoaded", false, func(_ dom.Event) {
		go setup()
	})
}

func setup() {
	p := &components.Page{}
	w := js.Global.Get("window")
	w = w.Call("addEventListener", "resize", func(e vecty.Event) {
		vecty.Rerender(p)
	})

	serverAddr := strings.TrimSuffix(document.BaseURI(), "/")
	api.Client = client.NewProxyClient(serverAddr)
	InitStore(store.S, api.Client)

	vecty.RenderBody(p)
}
