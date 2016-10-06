package out

import "github.com/fatih/color"

// TextTransformer is a function that can transform text.
type TextTransformer func(a ...interface{}) string

var Trans = struct {
	Error  TextTransformer
	Number TextTransformer
	Bold   TextTransformer
}{
	Error: func(a ...interface{}) string {
		return Color.Error.SprintFunc()(a...)
	},
	Number: func(a ...interface{}) string {
		return Color.Number.SprintFunc()(a...)
	},
	Bold: func(a ...interface{}) string {
		return color.New(color.Bold).SprintFunc()(a...)
	},
}

//func Colorize(val string, colors ...color.Attribute) string {
//	return color.New(colors...).SprintFunc()(val)
//
//	color.New().Print()
//}

var Color = struct {
	Header     *color.Color
	Error      *color.Color
	Success    *color.Color
	Datasource *color.Color
	Active     *color.Color
	Key        *color.Color
	String     *color.Color
	Binary     *color.Color
	Bool       *color.Color
	Number     *color.Color
	Null       *color.Color
}{
	Header:     color.New(color.FgHiBlue),
	Error:      color.New(color.FgRed, color.Bold),
	Success:    color.New(color.FgGreen, color.Bold),
	Datasource: color.New(color.FgCyan),
	Active:     color.New(color.Bold),
	Key:        color.New(color.FgBlue, color.Bold),
	String:     color.New(color.FgGreen, color.Bold),
	Binary:     color.New(color.FgCyan),
	Bool:       color.New(color.FgYellow, color.Bold),
	Number:     color.New(color.FgBlue, color.Bold),
	Null:       color.New(color.FgBlack, color.Bold),
}
