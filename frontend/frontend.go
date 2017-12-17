//go:generate gopherjs build frontend.go -o html/frontend.js
//go:generate bash -c "go run assets_generate.go"
package main

import (
	"context"
	"log"
	"strings"

	"honnef.co/go/js/dom"

	"github.com/gopherjs/gopherjs/js"
	vecty "github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
	"github.com/gopherjs/vecty/event"
	"github.com/gopherjs/vecty/prop"
	"gitlab.com/DSASanFrancisco/co-chair/frontend/store"
	"gitlab.com/DSASanFrancisco/co-chair/proto/client"
)

var apiClient client.ProxyClient

var state *store.Store

// Dims is a type for window dimensions
type Dims struct {
	Width, Height int64
}

var dims Dims

var document = dom.GetWindow().Document().(dom.HTMLDocument)

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
	Items []*BackendItem
}

// Render implements vecty.ComponentOrHTML
func (bl *BackendList) Render() vecty.ComponentOrHTML {

	buttonClicked := event.Click(func(e *vecty.Event) {
		var req client.StateRequest
		// we use a goroutine here to avoid error from gopherjs:
		// runtime error: cannot block in JavaScript callback, fix by wrapping code in goroutine
		go func() {
			resp, err := apiClient.State(context.TODO(), &req)
			if err != nil {
				log.Println("ERROR:", err)
				return
			}

			var listItems []*BackendItem
			for _, li := range resp.Backends {
				bi := &BackendItem{Domain: li.Domain}
				listItems = append(listItems, bi)
			}
			bl.Items = listItems
			vecty.Rerender(bl)
		}()
	})

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

	// MarkupOrChild can hold a MarkupList (our grid styling),
	// and lists of elem.Div
	for _, bi := range bl.Items {
		var box = NewCSS(
			"background-color", "#444",
			"color", "#fff",
			"border-radius", "5px",
			"padding", "20px",
			"font-size", "100%",
		)
		items = append(items,
			elem.Div(box.Yield(), vecty.Text(bi.Domain), vecty.Text(bi.IP)))
	}

	return elem.Div(
		elem.Div(items...),
		elem.Button(
			vecty.Text("Refresh"),
			vecty.Markup(buttonClicked),
		),
		&AddProxyForm{},
	)

}

// BackendItem ...
type BackendItem struct {
	Domain string
	IP     string
	vecty.Core
}

// Render implements vecty.ComponentOrHTML
func (bl *BackendItem) Render() vecty.ComponentOrHTML {
	return elem.ListItem(
		elem.Div(
			vecty.Text(bl.Domain),
			vecty.Text(bl.IP),
		),
	)
}

// AddProxyForm form is a small stateful component. IP and Domain are set on the
// fly when text in the input box changes. The contents of IP and Domain
// populate a request to the gRPC backend via websocket transport.
type AddProxyForm struct {
	vecty.Core
	IP     string
	Domain string
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
	ctx := context.Background()
	req := &client.Backend{}
	req.Domain = apf.Domain
	req.Ips = []string{apf.IP}
	go func() {
		_, err := apiClient.Put(ctx, req)
		if err != nil {
			log.Println("ERROR:", err)
		}
		vecty.Rerender(apf)
	}()
}

// Render implements vecty.ComponentOrHTML
func (apf *AddProxyForm) Render() vecty.ComponentOrHTML {
	return elem.Div(
		elem.Label(vecty.Text("domain")),
		elem.Input(vecty.Markup(prop.Value(apf.Domain), event.Input(apf.onDomainInput))),
		elem.Label(vecty.Text("ip")),
		elem.Input(vecty.Markup(prop.Value(apf.IP), event.Input(apf.onIPInput))),
		elem.Button(
			vecty.Text("Add"),
			vecty.Markup(event.Click(apf.onSubmit)),
		),
	)
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
