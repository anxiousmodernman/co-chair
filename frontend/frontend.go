//go:generate gopherjs build frontend.go -m -o html/frontend.js
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
	"github.com/gopherjs/vecty/style"
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
		vecty.Markup(
			style.Margin(style.Px(0)),
			vecty.Style("padding", "0"),
			vecty.Style("background", "#ccc"),
		),
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
		ctx := context.Background()
		req := &client.StateRequest{}
		// we use a goroutine here to avoid error from gopherjs:
		// runtime error: cannot block in JavaScript callback, fix by wrapping code in goroutine
		go func() {
			resp, err := apiClient.State(ctx, req)
			if err != nil {
				log.Println("ERROR:", err)
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

	var items []vecty.MarkupOrChild
	for _, bi := range bl.Items {
		items = append(items, elem.ListItem(
			elem.Div(vecty.Text(bi.Domain), vecty.Text(bi.IP)),
		))
	}

	return elem.Div(
		elem.UnorderedList(items...),
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

	var ulstyle vecty.MarkupList

	if dims.Width > 600 {
		ulstyle = vecty.Markup(
			vecty.Style("list-style", "none"),
			vecty.Style("background-color", "#444"),
			vecty.Style("margin", "auto"),
			vecty.Style("width", "100%"),
			vecty.Style("overflow", "auto"),
		)
	} else {
		ulstyle = vecty.Markup(
			vecty.Style("list-style", "none"),
			vecty.Style("background-color", "#444"),
			vecty.Style("text-align", "center"),
			vecty.Style("padding", "0"),
			vecty.Style("margin", "0"),
		)
	}

	return elem.Div(
		elem.UnorderedList(
			&NavItem{Name: "backends"},
			&NavItem{Name: "perf"},
			&NavItem{Name: "about"},
			ulstyle,
		),
	)
}

// NavItem ...
type NavItem struct {
	vecty.Core
	hovered bool
	Name    string
}

// Render implements vecty.ComponentOrHTML
func (ni *NavItem) Render() vecty.ComponentOrHTML {
	var listyle vecty.MarkupList

	if dims.Width > 600 {
		listyle = vecty.Markup(
			vecty.Style("font-family", "'Oswald', sans-serif"),
			vecty.Style("font-size", "1.4em"),
			vecty.Style("line-height", "50px"),
			vecty.Style("height", "50px"),
			vecty.Style("width", "120px"),
			vecty.Style("float", "left"),
		)
	} else {
		listyle = vecty.Markup(
			vecty.Style("font-family", "'Oswald', sans-serif"),
			vecty.Style("font-size", "1.2em"),
			vecty.Style("line-height", "40px"),
			vecty.Style("height", "40px"),
			vecty.Style("border-bottom", "1px solid #888"),
		)
	}

	var colr = ifElse(ni.hovered, "rgb(222, 222, 216)", "rgb(222, 222, 216)")
	var bckgrnd = ifElse(ni.hovered, "rgb(135, 133, 133)", "rgb(89, 89, 89)")

	var astyle vecty.MarkupList
	astyle = vecty.Markup(
		vecty.Style("text-decoration", "none"),
		vecty.Style("color", colr),
		vecty.Style("background-color", bckgrnd),
		vecty.Style("display", "block"),
		vecty.Style("transition", ".2s background-color"),
	)
	mo := event.MouseEnter(func(e *vecty.Event) {
		ni.hovered = true
		vecty.Rerender(ni)
	})
	ml := event.MouseLeave(func(e *vecty.Event) {
		ni.hovered = false
		vecty.Rerender(ni)
	})

	return elem.ListItem(
		elem.Anchor(
			astyle,
			vecty.Markup(vecty.Attribute("href", "#")),
			vecty.Text(ni.Name),
		),
		vecty.Markup(mo),
		vecty.Markup(ml),
		listyle,
	)
}

// if b, then i else e.
func ifElse(b bool, i, e string) string {
	if b {
		return i
	}
	return e
}

// MediaQuery ...
type MediaQuery struct {
	// Between
	Common []vecty.ComponentOrHTML
	Ranged []*MediaQueryStyle
}

// AddCommon ...
func (mq *MediaQuery) AddCommon(styles ...vecty.ComponentOrHTML) *MediaQuery {
	// for _, s := range styles {
	// 	mq.Common = append(mq.Common, s)
	// }
	mq.Common = append(mq.Common, styles...)
	return mq
}

func (mq *MediaQuery) AddRanged(min, max int, styles ...vecty.ComponentOrHTML) *MediaQuery {

	mq.Ranged = append(mq.Ranged, &MediaQueryStyle{Min: min, Max: max, Styles: styles})
	return mq
}

func (mq *MediaQuery) Apply() []vecty.List {
	var ret []vecty.ComponentOrHTML
	for _, c := range mq.Common {
		ret = append(ret, c)
	}

	return nil
}

type MediaQueryStyle struct {
	Min, Max int
	Styles   []vecty.ComponentOrHTML
}

// Range ...
type Range struct {
	Min, Max int
}

// TODO fix this
func (r *Range) Within(val int) bool {
	if val >= r.Min && val <= r.Max {
		return true
	}
	return false
}

// if between 0-600
// if gt 600

/*
  .nav li {
    width: 120px;
    border-bottom: none;
    height: 50px;
    line-height: 50px;
    font-size: 1.4em;
  }


  .nav li {
    display: inline-block;
    margin-right: -4px;
  }

  .nav li {
    float: left;
  }
  .nav ul {
    overflow: auto;
    width: 600px;
    margin: 0 auto;
  }
  .nav {
    background-color: #444;
  }
*/
