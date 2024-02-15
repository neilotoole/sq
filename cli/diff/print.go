package diff

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/colorz"
	"github.com/neilotoole/sq/libsq/core/errz"
)

var (
	newline       = []byte{'\n'}
	prefixMinuses = []byte("---")
	prefixPluses  = []byte("+++")
	prefixSection = []byte("@@")
)

// Print prints dif to w. If pr is nil, printing is monochrome.
func Print(ctx context.Context, w io.Writer, pr *output.Printing, header string, dif io.Reader) error {
	if dif == nil {
		return nil
	}

	var err error
	if pr == nil || pr.IsMonochrome() {
		// Monochrome
		if header != "" {
			if _, err = fmt.Fprintln(w, header); err != nil {
				return errz.Err(err)
			}
		}
		_, err = fmt.Fprintln(w, dif)
		return errz.Err(err)
	}

	// Else, output is colorized.
	if header != "" {
		if _, err = pr.DiffHeader.Fprintln(w, header); err != nil {
			return errz.Err(err)
		}
	}

	var (
		printHeader  = colorz.NewPrinter(pr.DiffHeader).Line
		printSection = colorz.NewPrinter(pr.DiffSection).Line
		printMinus   = colorz.NewPrinter(pr.DiffMinus).Line
		printPlus    = colorz.NewPrinter(pr.DiffPlus).Line
		printNormal  = colorz.NewPrinter(pr.DiffNormal).Line
	)

	sc := bufio.NewScanner(dif)
	var line []byte
	for i := 0; sc.Scan(); i++ {
		if ctx.Err() != nil {
			return errz.Err(ctx.Err())
		}

		line = sc.Bytes()
		if len(line) == 0 {
			// I'm not sure if this happens, but if it does, print an empty line.
			if _, err = w.Write(newline); err != nil {
				return errz.Err(err)
			}
		}

		switch {
		case i == 0 && bytes.HasPrefix(line, prefixMinuses):
			_, err = printHeader(w, line)
		case i == 1 && bytes.HasPrefix(line, prefixPluses):
			_, err = printHeader(w, line)
		case bytes.HasPrefix(line, prefixSection):
			_, err = printSection(w, line)
		case line[0] == '-':
			_, err = printMinus(w, line)
		case line[0] == '+':
			_, err = printPlus(w, line)
		default:
			_, err = printNormal(w, line)
		}

		if err != nil {
			return errz.Err(err)
		}
	}

	return errz.Err(sc.Err())
}
