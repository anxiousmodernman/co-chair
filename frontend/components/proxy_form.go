package components

import (
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
	Cert, Key            []byte
}

// Render ...
func (e *EditProxyForm) Render() vecty.ComponentOrHTML {

	// Get values from our fields
	cb1 := func(val string) { e.Domain = val }
	cb2 := func(val string) { e.IP = val }
	// cb3 := func(val string) { e.Protocol = val }
	cb4 := func(val string) { e.Cert = []byte(val) }
	cb5 := func(val string) { e.Key = []byte(val) }

	buttonStyle := styles.NewCSS("margin", "5px")
	click := event.Click(e.addProxy)
	// surrounding div takes the grid css
	return elem.Div(styles.ProxyForm().Yield(),
		&LabeledInput{Label: "domain", cb: cb1},
		&LabeledInput{Label: "ip:port", cb: cb2},
		// &LabeledInput{Label: "protocol prefix", cb: cb3},
		&LabeledInput{Label: "cert", TextArea: true, cb: cb4},
		&LabeledInput{Label: "key", TextArea: true, cb: cb5},
		elem.Button(
			buttonStyle.Yield(),
			vecty.Text("Add Proxy"), vecty.Markup(click)),
	)
}

func (e *EditProxyForm) addProxy(ev *vecty.Event) {
	b := client.Backend{}
	b.Domain = e.Domain
	b.Ips = []string{e.IP}
	// TODO add enum here
	// b.Protocol = e.Protocol
	b.BackendCert = &client.X509Cert{
		Cert: e.Cert, Key: e.Key,
	}
	api.PutBackend(store.S, api.Client, &b)
}

// LabeledInput ...
type LabeledInput struct {
	vecty.Core
	cb    func(string)
	Label string
	Val   string
	// if true, this is a text area
	TextArea bool
}

// Render ...
func (l *LabeledInput) Render() vecty.ComponentOrHTML {

	c := styles.NewCSS(
		"display", "grid",
		"grid-template-rows", "33% 67%",
	)
	// surrounding div takes the grid css of parent
	input := func() *vecty.HTML {
		if l.TextArea {
			return elem.TextArea(vecty.Markup(
				prop.Value(l.Val),
				event.Input(l.onInput)))
		}
		return elem.Input(vecty.Markup(
			prop.Value(l.Val),
			event.Input(l.onInput)))
	}()

	return elem.Div(
		elem.Label(vecty.Text(l.Label),
			input),
		c.Yield(),
	)
}

func (l *LabeledInput) onInput(e *vecty.Event) {
	l.Val = e.Target.Get("value").String()
	l.cb(l.Val)
	vecty.Rerender(l)
}
