package components

import (
	"github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
	"github.com/gopherjs/vecty/event"
	"gitlab.com/DSASanFrancisco/co-chair/frontend/store"
	"gitlab.com/DSASanFrancisco/co-chair/frontend/styles"
	"gitlab.com/DSASanFrancisco/co-chair/proto/client"
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
	store.S.Subscribe("proxy.form.active", rerender)
}

// Unmount ...
func (bl *BackendList) Unmount() {
	for _, id := range bl.callbacks {
		store.S.Unsubscribe(id)
	}
}

// Render implements vecty.ComponentOrHTML
func (bl *BackendList) Render() vecty.ComponentOrHTML {

	// items will become a list of backends blocked out in a grid. Backends is
	// a heterogeneous list of styles and Divs
	var items []vecty.MarkupOrChild

	// Show either, "+" or new form
	formActive := store.S.Get("proxy.form.active").(bool)
	// Add another component for the "+" sign; A UI component to add new proxies.
	if formActive {
		// show awesome form
		items = append(items, &EditProxyForm{})
	} else {
		// show proxy list
		if backends, ok := store.S.Get("proxy.list").([]*client.Backend); ok {
			for _, b := range backends {
				bi := BackendItem{Domain: b.Domain}
				items = append(items, &bi)
			}
			items = append(items, &BackendItemPlusSign{i: len(items) + 1})
		}
	}
	items = append(items, styles.BackendList().Yield())

	return elem.Div(
		items...,
	)
}

// BackendItem is one of our blocks on the grid of live proxies.
type BackendItem struct {
	Domain string
	IP     string
	vecty.Core
}

// Render implements vecty.ComponentOrHTML
func (bl *BackendItem) Render() vecty.ComponentOrHTML {
	var box = styles.NewCSS(
		"list-style-type", "none",
		"background-color", "#444",
		"color", "#fff",
		"border-radius", "5px",
		"padding", "20px",
		"font-size", "100%",
	)
	return elem.Div(box.Yield(), vecty.Text(bl.Domain), vecty.Text(bl.IP))
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
