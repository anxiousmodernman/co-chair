package components

import (
	"github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
	"github.com/gopherjs/vecty/event"
	"gitlab.com/DSASanFrancisco/co-chair/frontend/styles"
)

// Page returns a Body element suitable for vecty.RenderBody
type Page struct {
	vecty.Core
}

// Render implements vecty.Component for Page.
func (p *Page) Render() vecty.ComponentOrHTML {
	return elem.Body(
		styles.NewCSS("margin", "0", "padding", "0", "background", "#ccc").Yield(),
		elem.Header(
			&NavComponent{},
		),
		&BackendList{},
	)
}

// NavComponent ...
type NavComponent struct {
	vecty.Core
	Items []*NavItem
}

// Render implements vecty.ComponentOrHTML
func (n *NavComponent) Render() vecty.ComponentOrHTML {
	return elem.Div(
		elem.UnorderedList(
			&NavItem{Name: "proxy"},
			&NavItem{Name: "containers"},
			&NavItem{Name: "streams"},
			styles.NavBar().Yield(),
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

	return elem.ListItem(
		elem.Anchor(
			styles.NavAnchor(ni.hovered).Yield(),
			// vecty.Markup(vecty.Attribute("href", "#")), // TODO
			vecty.Text(ni.Name),
		),
		vecty.Markup(mo, ml),
		styles.NavItem().Yield(),
	)
}
