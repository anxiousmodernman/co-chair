package components

import (
	"github.com/gopherjs/vecty"
	"github.com/gopherjs/vecty/elem"
)

type EditProxyForm struct {
	vecty.Core
}

func (e *EditProxyForm) Render() vecty.ComponentOrHTML {

	return elem.Div(
		elem.Div(), // domain
		elem.Div(), // ip:port
		elem.Div(), // health
		elem.Div(), // save/cancel buttons
	)
}
