package components

import (
	"github.com/anxiousmodernman/co-chair/frontend/api"
	"github.com/anxiousmodernman/co-chair/frontend/store"
	"github.com/anxiousmodernman/co-chair/frontend/styles"
	"github.com/anxiousmodernman/co-chair/proto/client"
	"github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
	"github.com/gopherjs/vecty/event"
)

// BackendList shows our configured proxy's state.
type BackendList struct {
	vecty.Core
	// This might be eliminated in favor of using the store.
	//Items []*BackendItem
	// a slice of callback ids to de-register in our Unmount method.
	callbacks []int
}

// Mount ...
func (bl *BackendList) Mount() {
	rerender := func() { vecty.Rerender(bl) }
	store.S.Subscribe("proxy.list", rerender)
}

// Unmount ...
func (bl *BackendList) Unmount() {
	for _, id := range bl.callbacks {
		store.S.Unsubscribe(id)
	}
}

// Render implements vecty.ComponentOrHTML
func (bl *BackendList) Render() vecty.ComponentOrHTML {

	var items []vecty.MarkupOrChild
	if backends, ok := store.S.Get("proxy.list").([]*client.Backend); ok {
		for _, b := range backends {
			if len(b.Ips) == 0 {
				continue
			}
			bi := BackendItem{Domain: b.Domain, IP: b.Ips[0]} // only single ip for now
			items = append(items, &bi)
		}
	}
	items = append(items, styles.ProxyList().Yield())
	return elem.Div(items...)
}

// BackendItem is one of our blocks on the grid of live proxies.
type BackendItem struct {
	Domain   string
	IP       string
	Protocol string
	vecty.Core
}

// Render implements vecty.ComponentOrHTML
func (bi *BackendItem) Render() vecty.ComponentOrHTML {
	box := styles.NewCSS(
		"background-color", "#444",
		"color", "#fff",
		"border-radius", "5px",
		"padding", "20px",
		"font-size", "100%",
		"display", "grid",
	)

	ip := styles.NewCSS("font-size", "75%")
	click := vecty.Markup(event.Click(bi.deleteProxy))
	msg := bi.Protocol + bi.IP

	return elem.Div(box.Yield(), elem.Div(vecty.Text(bi.Domain)),
		elem.Div(ip.Yield(), vecty.Text(msg)),
		elem.Div(),
		elem.Button(vecty.Text("delete"), click),
	)
}

func (bi *BackendItem) deleteProxy(e *vecty.Event) {
	d := bi.Domain
	api.DeleteProxy(store.S, api.Client, d)

}

// BackendItemPlusSign ...
type BackendItemPlusSign struct {
	i int
	vecty.Core
}

// Render implements vecty.ComponentOrHTML
func (plus *BackendItemPlusSign) Render() vecty.ComponentOrHTML {
	open := event.Click(func(e *vecty.Event) {
		store.S.Put("proxy.form.active", true)
	})

	var box *styles.CSS
	box = styles.NewCSS(
		"list-style-type", "none",
		"background-color", "#444",
		"color", "#fff",
		"border-radius", "5px",
		"padding", "20px",
		"font-size", "200%",
		"text-align", "center",
	)

	var div *vecty.HTML
	div = elem.Div(box.Yield(), vecty.Text("+"), vecty.Markup(open))

	return div
}
