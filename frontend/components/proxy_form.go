package components

import (
	"github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
	"github.com/gopherjs/vecty/event"
	"github.com/gopherjs/vecty/prop"
	"gitlab.com/DSASanFrancisco/co-chair/frontend/styles"
)

// EditProxyForm ...
type EditProxyForm struct {
	vecty.Core
}

// Render ...
func (e *EditProxyForm) Render() vecty.ComponentOrHTML {

	// surrounding div takes the grid css
	return elem.Div(styles.ProxyForm().Yield(),
		&LabeledInput{Label: "domain"},
		&LabeledInput{Label: "ip:port"},
		&LabeledInput{Label: "health check"},
		&SaveCancel{},
	)
}

// LabeledInput ...
type LabeledInput struct {
	vecty.Core
	Label string
	Val   string
}

// Render ...
func (l *LabeledInput) Render() vecty.ComponentOrHTML {

	c := styles.NewCSS(
		"align", "left",
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
	vecty.Rerender(l)
}

type SaveCancel struct {
	vecty.Core
}

func (s *SaveCancel) Render() vecty.ComponentOrHTML {
	c := styles.NewCSS("display", "grid",
		"align", "left",
		"grid-template-columns", "50% 50%", "padding", "5px")

	m := styles.NewCSS("margin", "5px")
	return elem.Div(c.Yield(),
		elem.Button(m.Yield(), vecty.Text("Save")),
		elem.Button(m.Yield(), vecty.Text("Cancel")),
		elem.Div(),
	)
}

func (s *SaveCancel) onCancel(e *vecty.Event) {

}
