package diff

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/neilotoole/sq/libsq/core/colorz"
	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/core/errz"
)

var (
	newline       = []byte{'\n'}
	prefixMinuses = []byte("---")
	prefixPluses  = []byte("+++")
	prefixSection = []byte("@@")
)

// Print prints dif to w. If pr is nil, printing is monochrome.
func Print(ctx context.Context, w io.Writer, pr *diffdoc.Colors, header string, dif io.Reader) error {
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
		if _, err = pr.Header.Fprintln(w, header); err != nil {
			return errz.Err(err)
		}
	}

	var (
		printHeader  = colorz.NewPrinter(pr.Header).Line
		printSection = colorz.NewPrinter(pr.Section).Line
		printMinus   = colorz.NewPrinter(pr.Deletion).Line
		printPlus    = colorz.NewPrinter(pr.Insertion).Line
		printNormal  = colorz.NewPrinter(pr.Context).Line
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

// colorizeHunks prints a colorized diff hunks to w. The reader must not include
// the diff header. That is, it should not include:
//
//	sq diff --data @diff/sakila_a.actor @diff/sakila_b.actor
//	--- @diff/sakila_a.actor
//	+++ @diff/sakila_b.actor
//
// Instead, hunks should contain one or more hunks, e.g.
//
//	@@ -2,3 +2,3 @@
//	 1         PENELOPE    GUINESS    2020-06-11T02:50:54Z
//	-2         NICK        WAHLBERG   2020-06-11T02:50:54Z
//	+2         NICK        BERGER     2020-06-11T02:50:54Z
//	 3         ED          CHASE      2020-06-11T02:50:54Z
//	@@ -12,3 +12,3 @@
//	 11        ZERO        CAGE       2020-06-11T02:50:54Z
//
// If pr is nil, printing is monochrome.
func colorizeHunks(ctx context.Context, w io.Writer, pr *diffdoc.Colors, hunks io.Reader) error {
	if hunks == nil {
		return nil
	}

	var err error
	if pr == nil || pr.IsMonochrome() {
		_, err = io.Copy(w, hunks)
		return errz.Err(err)
	}

	var (
		printSection = colorz.NewPrinter(pr.Section).Line
		printMinus   = colorz.NewPrinter(pr.Deletion).Line
		printPlus    = colorz.NewPrinter(pr.Insertion).Line
		printNormal  = colorz.NewPrinter(pr.Context).Line
	)

	sc := bufio.NewScanner(hunks)
	var line []byte
	for sc.Scan() {
		if ctx.Err() != nil {
			return errz.Err(ctx.Err())
		}

		line = sc.Bytes()
		switch {
		case len(line) == 0:
			// I'm not sure if this happens, but if it does, print an empty line.
			_, err = w.Write(newline)
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
