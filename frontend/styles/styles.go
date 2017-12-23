package styles

var (
	//dims       Dimensions
	breakpoint int64 = 600
	mq         *MediaQuery
)

func init() {
	mq = NewMediaQuery([]Breakpoint{600})
}

func NavBar() *CSS {

	common := NewCSS(
		"list-style", "none",
		"background-color", "#444",
		// just use grid for everything, okay i get it now
		"display", "grid",
	)
	mobile := NewCSS(
		"margin", "auto",
		"width", "100%",
		"overflow", "auto",
	)
	desktop := NewCSS(
		"text-align", "center",
		"padding-left", "50px",
		"padding-right", "50px",
		"margin", "0",
		// nav nav nav -----  empty space  ----- logout
		"grid-template-columns", "120px 120px 120px auto 100px",
	)

	return mq.Query(common, mobile, desktop)
}

func NavItem() *CSS {
	common := NewCSS(
		"font-family", "'Oswald', sans-serif",
	)
	mobile := NewCSS(
		"font-size", "1.4em",
		"line-height", "50px",
		"height", "50px",
	)
	desktop := NewCSS(
		"width", "100%",
		// "float", "left",
		"font-size", "1.2em",
		"line-height", "40px",
		"height", "40px",
		"border-bottom", "1px solid #888",
	)

	return mq.Query(common, mobile, desktop)
}

func NavAnchor(hovered bool) *CSS {
	var color string
	if hovered {
		color = "rgb(135, 133, 133)"
	} else {
		color = "rgb(89, 89, 89)"
	}
	common := NewCSS(
		"text-decoration", "none",
		"display", "block",
		"transition", ".2s background-color",
		"color", "rgb(222, 222, 216)",
		"background-color", color,
	)

	return common
}

func ProxyForm() *CSS {

	common := NewCSS(
		"display", "grid",
		"background-color", "#444",
		"border-radius", "5px",
		"padding", "15px",
		"margin-left", "auto",
		"margin-right", "auto",
		"margin-top", "20px",
	)

	mobile := NewCSS()
	desktop := NewCSS(
		"width", "800px",
		"grid-template-columns", "50% 50%",
	)
	c := mq.Query(common, mobile, desktop)
	return c
}

func ProxyList() *CSS {
	common := NewCSS(
		"grid-gap", "10px",
		"display", "grid",
		"margin", "10px",
	)
	mobile := NewCSS(
		"width", "100%",
		"grid-template-columns", "repeat(1, 80%)",
	)
	desktop := NewCSS(
		"width", "800px",
		"margin-left", "auto",
		"margin-right", "auto",
		"margin-top", "20px",
		"grid-template-columns", "repeat(4, 200px)",
	)
	return mq.Query(common, mobile, desktop)
}
