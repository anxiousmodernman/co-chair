package styles

import (
	"log"

	"github.com/gopherjs/gopherjs/js"
	"github.com/gopherjs/vecty"
)

var window = js.Global.Get("window")

// Breakpoint is a screen width in pixels.
type Breakpoint int64

// CSS is our container type for multiple CSS styles. You cannot pass this to
// an element directly, but must rather call its Yield method.
type CSS struct {
	raw map[string]string
}

// NewCSS gives us a CSS from a list of property-value pairs. The number of styles
// must be a multiple of 2.
func NewCSS(styles ...string) *CSS {
	var c CSS
	c.raw = make(map[string]string)
	c.multiApplyer(styles...)
	return &c
}

// Yield takes the CSS accumulated styles and creates a vecty.MarkupList, which
// can be passed to an element.
//
// Example:
//   var c CSS
//   e := elem.Div(c.And("color", "red").Yield())
func (c *CSS) Yield() vecty.MarkupList {
	var s []vecty.Applyer
	for k, v := range c.raw {
		s = append(s, vecty.Style(k, v))
	}
	return vecty.Markup(s...)

}

func (c *CSS) multiApplyer(s ...string) {
	if len(s)%2 != 0 {
		panic("len must be multiple of 2")
	}

	var i = 0
	for i < len(s) {
		c.raw[s[i]] = s[i+1]
		i = i + 2
	}
}

func (c *CSS) Combine(moreSpecific *CSS) *CSS {

	combined := make(map[string]string)
	for k, v := range c.raw {
		combined[k] = v
	}
	// We might overwrite some keys here, and that's our intention.
	for k, v := range moreSpecific.raw {
		combined[k] = v
	}
	return &CSS{raw: combined}
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

type MediaQuery struct {
	breakpoints []Breakpoint
	h, w        int64
	win         *js.Object
}

func NewMediaQuery(breakpoints []Breakpoint) *MediaQuery {
	var m MediaQuery
	m.breakpoints = breakpoints
	m.w = js.Global.Get("window").Get("innerWidth").Int64()
	m.h = js.Global.Get("window").Get("innerHeight").Int64()
	// We don't need to concern ourselves with vecty.Rerender here, do we? The
	// Page component can have it's own listener instantiated in frontend.go
	window = window.Call("addEventListener", "resize", func(e vecty.Event) {
		m.w = js.Global.Get("window").Get("innerWidth").Int64()
		m.h = js.Global.Get("window").Get("innerHeight").Int64()
	})
	return &m
}

// Query yields a *CSS that is the common param combined with exactly one of the
// variadic selectables.
func (m *MediaQuery) Query(common *CSS, selectables ...*CSS) *CSS {
	if len(m.breakpoints) == 0 {
		log.Println("common returned")
		return common
	}
	// If we have 1 breakpoint, we need 2 *CSS. If we have 2 breakpoints, we
	// need 3 *CSS, and so on.
	if len(selectables)-1 != len(m.breakpoints) {
		panic("len(selectable) != len(breakpoints)-1")
	}
	return common.Combine(selectables[m.index()])
}

func (m *MediaQuery) index() int {
	for i, bp := range m.breakpoints {
		if Breakpoint(m.w) > bp {
			return i + 1
		}
	}
	return 0
}
