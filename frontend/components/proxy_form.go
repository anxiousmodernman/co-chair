package components

import (
	"log"

	"github.com/anxiousmodernman/co-chair/frontend/api"
	"github.com/anxiousmodernman/co-chair/frontend/store"
	"github.com/anxiousmodernman/co-chair/frontend/styles"
	"github.com/anxiousmodernman/co-chair/proto/client"
	"github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
	"github.com/gopherjs/vecty/event"
	"github.com/gopherjs/vecty/prop"
)

// EditProxyForm ...
type EditProxyForm struct {
	vecty.Core
	Domain, IP, Protocol string
}

// Render ...
func (e *EditProxyForm) Render() vecty.ComponentOrHTML {

	// Get values from our fields
	cb1 := func(val string) { e.Domain = val }
	cb2 := func(val string) { e.IP = val }
	cb3 := func(val string) { e.Protocol = val }

	buttonStyle := styles.NewCSS("margin", "5px")
	click := event.Click(e.addProxy)
	// surrounding div takes the grid css
	return elem.Div(styles.ProxyForm().Yield(),
		&LabeledInput{Label: "domain", cb: cb1},
		&LabeledInput{Label: "ip:port", cb: cb2},
		&LabeledInput{Label: "health check"},
		&LabeledInput{Label: "protocol prefix", cb: cb3},
		elem.Button(
			buttonStyle.Yield(),
			vecty.Text("Add Proxy"), vecty.Markup(click)),
	)
}

func (e *EditProxyForm) addProxy(ev *vecty.Event) {
	b := client.Backend{}
	b.Domain = e.Domain
	b.Ips = []string{e.IP}
	b.Protocol = e.Protocol
	log.Println("put backend", b)
	api.PutBackend(store.S, api.Client, &b)
}

// LabeledInput ...
type LabeledInput struct {
	vecty.Core
	cb    func(string)
	Label string
	Val   string
}

// Render ...
func (l *LabeledInput) Render() vecty.ComponentOrHTML {

	c := styles.NewCSS(
		"display", "grid",
		"grid-template-rows", "33% 67%",
	)
	// surrounding div takes the grid css of parent. Our label and input
	// will be stacked vertically.
	return elem.Div(
		elem.Label(vecty.Text(l.Label),
			elem.Input(vecty.Markup(
				prop.Value(l.Val),
				event.Input(l.onInput)))),
		c.Yield(),
	)
}

func (l *LabeledInput) onInput(e *vecty.Event) {
	l.Val = e.Target.Get("value").String()
	l.cb(l.Val)
	vecty.Rerender(l)
}
