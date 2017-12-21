package components

import (
	"github.com/gopherjs/gopherjs/js"
	"github.com/gopherjs/vecty"
)

var (
	dims       Dimensions
	breakpoint int64 = 600
)

func init() {
	dims.Width = js.Global.Get("window").Get("innerWidth").Int64()
	dims.Height = js.Global.Get("window").Get("innerHeight").Int64()
}

// RerenderOnResize sets a callback called when innerWidth or innerHeigh changes.
func RerenderOnResize(c vecty.Component) {
	w := js.Global.Get("window")
	w = w.Call("addEventListener", "resize", func(e vecty.Event) {
		dims.Width = js.Global.Get("window").Get("innerWidth").Int64()
		dims.Height = js.Global.Get("window").Get("innerHeight").Int64()
		vecty.Rerender(c)
	})
}

// Dimensions is a type for window dimensions.
type Dimensions struct {
	Width, Height int64
}

func mobile() bool  { return dims.Width <= breakpoint }
func desktop() bool { return dims.Width > breakpoint }
