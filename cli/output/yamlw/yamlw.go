// Package yamlw implements output writers for YAML.
package yamlw

import (
	"bytes"
	"io"

	"github.com/fatih/color"
	goccy "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/lexer"
	"github.com/neilotoole/sq/libsq/core/errz"

	"github.com/goccy/go-yaml/printer"
	"github.com/neilotoole/sq/cli/output"
)

// writeYAML prints a YAML representation of v to out, using specs
// from pr.
func writeYAML(p printer.Printer, out io.Writer, v any) error {
	b, err := goccy.Marshal(v)
	if err != nil {
		return errz.Err(err)
	}

	tokens := lexer.Tokenize(string(b))

	_, err = out.Write([]byte(p.PrintTokens(tokens) + "\n"))
	return errz.Err(err)
}

func newPrinter(pr *output.Printing) printer.Printer {
	var p printer.Printer
	p.LineNumber = false
	p.Bool = func() *printer.Property {
		return &printer.Property{
			Prefix: formatColor(pr.Bool),
			Suffix: reset,
		}
	}
	p.Number = func() *printer.Property {
		return &printer.Property{
			Prefix: formatColor(pr.Number),
			Suffix: reset,
		}
	}
	p.MapKey = func() *printer.Property {
		return &printer.Property{
			Prefix: formatColor(pr.Key),
			Suffix: reset,
		}
	}
	p.Anchor = func() *printer.Property {
		return &printer.Property{
			Prefix: formatColor(pr.Faint),
			Suffix: reset,
		}
	}
	p.Alias = func() *printer.Property {
		return &printer.Property{
			Prefix: formatColor(pr.Faint),
			Suffix: reset,
		}
	}
	p.String = func() *printer.Property {
		return &printer.Property{
			Prefix: formatColor(pr.String),
			Suffix: reset,
		}
	}
	return p
}

const reset = "\x1b[0m"

// formatColor is a hack to extract the escape chars from
// a color.
func formatColor(c *color.Color) string {
	if c == nil {
		return ""
	}

	// Make a copy because the pkg-level color.NoColor could be false.
	c2 := *c
	c2.EnableColor()

	b := []byte(c2.Sprint(" "))
	i := bytes.IndexByte(b, ' ')
	if i <= 0 {
		// Shouldn't happen
		return ""
	}

	return string(b[:i])
}
