//go:generate gopherjs build frontend.go -o html/frontend.js
//go:generate bash -c "go run assets_generate.go"
package main

import (
	"log"
	"strings"

	"honnef.co/go/js/dom"

	"github.com/gopherjs/gopherjs/js"
	vecty "github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
	"github.com/gopherjs/vecty/event"
	"github.com/gopherjs/vecty/prop"
	"gitlab.com/DSASanFrancisco/co-chair/frontend/api"
	"gitlab.com/DSASanFrancisco/co-chair/frontend/store"
	"gitlab.com/DSASanFrancisco/co-chair/proto/client"
)

var (
	apiClient client.ProxyClient
	state     *store.Store
	dims      Dims
	document  = dom.GetWindow().Document().(dom.HTMLDocument)
)

// InitStore initializes the state for our app.
func InitStore(s *store.Store, c client.ProxyClient) {
	// Note: we have to keep track of paths explicitly.
	s.Put("proxy.form.active", false)
	s.Put("proxy.list", make([]*client.Backend, 0))
	// Populate the store with api calls
	api.ProxyState(s, c)
}

// Dims is a type for window dimensions
type Dims struct {
	Width, Height int64
}

// no-op main
func main() {}

// Ensure our setup() gets called as soon as the DOM has loaded
func init() {
	dims.Width = js.Global.Get("window").Get("innerWidth").Int64()
	dims.Height = js.Global.Get("window").Get("innerHeight").Int64()
	document.AddEventListener("DOMContentLoaded", false, func(_ dom.Event) {
		go setup()
	})
}

func setup() {
	state = store.NewStore()
	p := &Page{}

	w := js.Global.Get("window")
	w = w.Call("addEventListener", "resize", func(e vecty.Event) {
		// TODO: use debounce func here?
		dims.Width = js.Global.Get("window").Get("innerWidth").Int64()
		dims.Height = js.Global.Get("window").Get("innerHeight").Int64()
		vecty.Rerender(p)
	})

	serverAddr := strings.TrimSuffix(document.BaseURI(), "/")
	apiClient = client.NewProxyClient(serverAddr)
	InitStore(state, apiClient)

	vecty.RenderBody(p)
}

// Page returns a Body element suitable for vecty.RenderBody
type Page struct {
	vecty.Core
}

// Render implements vecty.Component for Page.
func (p *Page) Render() vecty.ComponentOrHTML {
	return elem.Body(
		NewCSS("margin", "0", "padding", "0", "background", "#ccc").Yield(),
		elem.Header(
			&NavComponent{},
		),
		&BackendList{},
	)
}

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
	state.Subscribe("proxy.list", rerender)
	state.Subscribe("proxy.form.active", rerender)
}

// Unmount ...
func (bl *BackendList) Unmount() {
	for _, id := range bl.callbacks {
		state.Unsubscribe(id)
	}
}

// Render implements vecty.ComponentOrHTML
func (bl *BackendList) Render() vecty.ComponentOrHTML {

	// items will become a list of backends blocked out in a grid. Backends is
	// a heterogeneous list of styles and Divs
	var items []vecty.MarkupOrChild

	var grid CSS
	grid.And(
		"grid-gap", "10px",
		"display", "grid",
		"margin", "10px",
	).AndIf(dims.Width > 600,
		"width", "800px",
		"grid-template-columns", "repeat(4, 200px)",
	).AndIf(dims.Width <= 600,
		"width", "100%",
		"grid-template-columns", "repeat(1, 80%)",
	)
	items = append(items, grid.Yield())

	// Show either, "+" or new form
	formActive := state.Get("proxy.form.active").(bool)
	// Add another component for the "+" sign; A UI component to add new proxies.
	if formActive {
		// show awesome form
		items = append(items, &AddProxyForm{})
	} else {
		// show proxy list
		if backends, ok := state.Get("proxy.list").([]*client.Backend); ok {
			for _, b := range backends {
				bi := BackendItem{Domain: b.Domain}
				items = append(items, &bi)
			}
			items = append(items, &BackendItemPlusSign{i: len(items) + 1})
		}
	}

	return elem.Div(
		elem.Div(items...),
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
	var box = NewCSS(
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
		state.Put("proxy.form.active", true)
	})

	var box *CSS
	box = NewCSS(
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

// AddProxyForm form is a small stateful component. IP and Domain are set on the
// fly when text in the input box changes. The contents of IP and Domain
// populate a request to the gRPC backend via websocket transport.
type AddProxyForm struct {
	vecty.Core
	IP     string
	Domain string
}

// Render implements vecty.ComponentOrHTML
func (apf *AddProxyForm) Render() vecty.ComponentOrHTML {

	var form *CSS
	form = NewCSS(
		"align", "right",
		"list-style-type", "none",
		"background-color", "#444",
		"color", "#fff",
		"border-radius", "5px",
		"padding", "15px",
		"display", "grid",
	).AndIf(dims.Width < 600,
		"grid-template-columns", "repeat(1, 80%)",
	).AndIf(dims.Width >= 600,
		"grid-template-columns", "40% 40% 20%",
		"grid-column-start", "1",
		"grid-column-end", "5",
		"height", "250px",
	)

	return elem.Div(form.Yield(),
		elem.Div(elem.Label(vecty.Text("domain")),
			elem.Input(vecty.Markup(prop.Value(apf.Domain), event.Input(apf.onDomainInput)))),
		elem.Div(elem.Label(vecty.Text("ip")),
			elem.Input(vecty.Markup(prop.Value(apf.IP), event.Input(apf.onIPInput)))),
		elem.Div(
			elem.Button(
				vecty.Text("Add"),
				vecty.Markup(event.Click(apf.onSubmit)),
			),
		),
	)
}

// onIPInput modifies our internal state. We maintain internal state to collect
// data before we send it to the server.
func (apf *AddProxyForm) onIPInput(e *vecty.Event) {
	apf.IP = e.Target.Get("value").String()
	vecty.Rerender(apf)
}

func (apf *AddProxyForm) onDomainInput(e *vecty.Event) {
	apf.Domain = e.Target.Get("value").String()
	vecty.Rerender(apf)
}

func (apf *AddProxyForm) onSubmit(e *vecty.Event) {
	req := &client.Backend{}
	req.Domain = apf.Domain
	req.Ips = []string{apf.IP}
	log.Println("put request", req)
	api.PutBackend(state, apiClient, req)
}

// NavComponent ...
type NavComponent struct {
	vecty.Core
	Items []*NavItem
}

// Render implements vecty.ComponentOrHTML
func (n *NavComponent) Render() vecty.ComponentOrHTML {

	var ulstyle CSS
	ulstyle.And("list-style", "none",
		"background-color", "#444",
	).AndIf(dims.Width > 600,
		"margin", "auto",
		"width", "100%",
		"overflow", "auto",
	).AndIf(dims.Width <= 600,
		"text-align", "center",
		"padding", "0",
		"margin", "0",
	)

	return elem.Div(
		elem.UnorderedList(
			&NavItem{Name: "proxy"},
			&NavItem{Name: "containers"},
			&NavItem{Name: "streams"},
			ulstyle.Yield(),
		),
	)
}

// NavItem represents an item in the top nav bar.
type NavItem struct {
	vecty.Core
	hovered bool
	Name    string
}

// Render implements vecty.ComponentOrHTML
func (ni *NavItem) Render() vecty.ComponentOrHTML {

	mo := event.MouseEnter(func(e *vecty.Event) {
		ni.hovered = true
		vecty.Rerender(ni)
	})
	ml := event.MouseLeave(func(e *vecty.Event) {
		ni.hovered = false
		vecty.Rerender(ni)
	})

	var listyle CSS
	listyle.And(
		"font-family", "'Oswald', sans-serif",
	).AndIf(dims.Width > 600,
		"font-size", "1.4em",
		"line-height", "50px",
		"height", "50px",
		"width", "120px",
		"float", "left",
	).AndIf(dims.Width <= 600,
		"font-size", "1.2em",
		"line-height", "40px",
		"height", "40px",
		"border-bottom", "1px solid #888",
	)

	var astyle CSS
	astyle.And("text-decoration", "none",
		"display", "block",
		"transition", ".2s background-color",
		"color", "rgb(222, 222, 216)",
	).AndIf(ni.hovered,
		"background-color", "rgb(135, 133, 133)",
	).AndIf(!ni.hovered,
		"background-color", "rgb(89, 89, 89)")

	return elem.ListItem(
		elem.Anchor(
			astyle.Yield(),
			// vecty.Markup(vecty.Attribute("href", "#")), // TODO
			vecty.Text(ni.Name),
		),
		vecty.Markup(mo, ml),
		listyle.Yield(),
	)
}

// CSS is our container type for multiple CSS styles. You cannot pass this to
// an element directly, but must rather call its Yield method.
type CSS struct {
	Styles []vecty.Applyer
}

// Yield takes the CSS accumulated styles and creates a vecty.MarkupList, which
// can be passed to an element.
//
// Example:
//   var c CSS
//   e := elem.Div(c.And("color", "red").Yield())
func (c *CSS) Yield() vecty.MarkupList {
	return vecty.Markup(c.Styles...)

}

func (c *CSS) multiApplyer(s ...string) {
	if len(s)%2 != 0 {
		panic("len must be multiple of 2")
	}

	var i = 0
	for i < len(s) {
		c.Styles = append(c.Styles, vecty.Style(s[i], s[i+1]))
		i = i + 2
	}
}

// And adds the styles no matter what. The number of styles must be a multiple of 2.
func (c *CSS) And(styles ...string) *CSS {
	c.multiApplyer(styles...)
	return c
}

// AndIf adds the styles if b is true. The number of styles must be a multiple of 2.
func (c *CSS) AndIf(b bool, styles ...string) *CSS {
	if b {
		c.multiApplyer(styles...)
	}
	return c
}

// NewCSS gives us a CSS from a list of property-value pairs. The number of styles
// must be a multiple of 2.
func NewCSS(styles ...string) *CSS {
	var c CSS
	c.multiApplyer(styles...)
	return &c
}
