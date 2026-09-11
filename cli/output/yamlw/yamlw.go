// Package yamlw implements output writers for YAML.
package yamlw

import (
	"bytes"
	"io"
	"strconv"

	"github.com/fatih/color"
	goccy "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/printer"
	"github.com/shopspring/decimal"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// newDecimalMarshaler returns a goccy encode option that renders decimal values.
// In number mode the value is emitted as a bare YAML number; otherwise it is
// emitted as a quoted string (precision-safe). goccy re-parses the returned
// bytes as a YAML document, so quote-wrapping yields a string node. See #846.
func newDecimalMarshaler(asNumber bool) goccy.EncodeOption {
	return goccy.CustomMarshaler[decimal.Decimal](func(d decimal.Decimal) ([]byte, error) {
		s := stringz.FormatDecimal(d)
		if asNumber {
			return []byte(s), nil
		}
		return []byte(strconv.Quote(s)), nil
	})
}

// MarshalToString renders v to a string.
func MarshalToString(pr *output.Printing, v any) (string, error) {
	p := newPrinter(pr)
	buf := &bytes.Buffer{}
	if err := writeYAML(buf, p, v); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// writeYAML prints a YAML representation of v to out, using the colorization
// and formatting from printer p. It always marshals decimal values as quoted
// strings; only the record writer (recordwriter.go) needs option-aware decimal
// rendering.
func writeYAML(out io.Writer, p printer.Printer, v any) error {
	b, err := goccy.MarshalWithOptions(v, newDecimalMarshaler(false))
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
	if pr.IsMonochrome() {
		return p
	}

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
