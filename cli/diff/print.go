package diff

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/colorz"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"io"
	"strings"
)

// Print prints dif to w. If pr is nil, printing is in monochrome.
func Print(ctx context.Context, w io.Writer, pr *output.Printing, header, dif string) error {
	if dif == "" {
		return nil
	}

	if pr == nil || pr.IsMonochrome() {
		if header != "" {
			dif = header + "\n" + dif
		}
		_, err := fmt.Fprintln(w, dif)
		return errz.Err(err)
	}

	bar := progress.FromContext(ctx).
		NewUnitCounter("Preparing diff output", "line", progress.OptMemUsage)

	// FIXME: Should stream diff line colorization, not buffer it all up.
	after := stringz.VisitLines(dif, func(i int, line string) string {
		if i == 0 && strings.HasPrefix(line, "---") {
			return pr.DiffHeader.Sprint(line)
		}
		if i == 1 && strings.HasPrefix(line, "+++") {
			return pr.DiffHeader.Sprint(line)
		}

		if strings.HasPrefix(line, "@@") {
			return pr.DiffSection.Sprint(line)
		}

		if strings.HasPrefix(line, "-") {
			return pr.DiffMinus.Sprint(line)
		}

		if strings.HasPrefix(line, "+") {
			return pr.DiffPlus.Sprint(line)
		}

		bar.Incr(1)
		return pr.DiffNormal.Sprint(line)
	})

	if header != "" {
		after = pr.DiffHeader.Sprint(header) + "\n" + after
	}

	bar.Stop()
	_, err := fmt.Fprintln(w, after)
	return errz.Err(err)
}

var (
	newline       = []byte{'\n'}
	prefixMinuses = []byte("---")
	prefixPluses  = []byte("+++")
	prefixSection = []byte("@@")
)

// Print2 prints dif to w. If pr is nil, printing is in monochrome.
func Print2(ctx context.Context, w io.Writer, pr *output.Printing, header string, dif io.Reader) error {
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
		printHeader  = colorz.NewPrinter(pr.DiffHeader)
		printSection = colorz.NewPrinter(pr.DiffSection)
		printMinus   = colorz.NewPrinter(pr.DiffMinus)
		printPlus    = colorz.NewPrinter(pr.DiffPlus)
		printNormal  = colorz.NewPrinter(pr.DiffNormal)
	)

	sc := bufio.NewScanner(dif)
	var line []byte
	for i := 0; sc.Scan(); i++ {
		select {
		case <-ctx.Done():
			return errz.Err(ctx.Err())
		default:
		}

		line = sc.Bytes()
		if len(line) == 0 {
			if _, err = w.Write(newline); err != nil {
				return errz.Err(err)
			}
		}

		switch {
		case i == 0 && bytes.HasPrefix(line, prefixMinuses):
			_, err = printHeader.Line(w, line)
		case i == 1 && bytes.HasPrefix(line, prefixPluses):
			_, err = printHeader.Line(w, line)
		case bytes.HasPrefix(line, prefixSection):
			_, err = printSection.Line(w, line)
		case line[0] == '-':
			_, err = printMinus.Line(w, line)
		case line[0] == '+':
			_, err = printPlus.Line(w, line)
		default:
			_, err = printNormal.Line(w, line)
		}

		if err != nil {
			return errz.Err(err)
		}
	}

	return errz.Err(sc.Err())
}
