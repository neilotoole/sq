package diffdoc

import (
	"bufio"
	"bytes"
	"context"
	"io"

	"github.com/neilotoole/sq/libsq/core/ioz/scannerz"

	"github.com/neilotoole/sq/libsq/core/colorz"
	"github.com/neilotoole/sq/libsq/core/errz"
)

type colorizer struct {
	sc      *bufio.Scanner
	buf     *bytes.Buffer
	clrs    codes
	hasData bool
}

func NewColorizer(ctx context.Context, clrs *Colors, src io.Reader) io.Reader {
	if src == nil || clrs == nil || clrs.IsMonochrome() {
		return src
	}

	c := &colorizer{
		sc:      scannerz.NewScanner(ctx, src),
		buf:     &bytes.Buffer{},
		hasData: true,
		clrs:    *clrs.codes(),
	}

	c.buf.Grow(512)
	return c
}

const newline = '\n'

func (c *colorizer) Read(p []byte) (n int, err error) {
	if c.buf.Len() >= len(p) {
		return c.buf.Read(p)
	}

	var line []byte
	var length int
	var b0 byte

	for c.buf.Len() < len(p) && c.sc.Scan() {
		line = c.sc.Bytes()
		length = len(line)
		if length == 0 {
			_ = c.buf.WriteByte(newline)
			continue
		}

		b0 = line[0]
		if length == 0 {
			switch b0 {
			case '-':
				c.clrs.deletion.WritelnByte(c.buf, '-')
			case '+':
				c.clrs.deletion.WritelnByte(c.buf, '+')
			case ' ':
				_ = c.buf.WriteByte(' ')
				_ = c.buf.WriteByte(newline)
			default:
				// This would be slightly weird, but it must be a single-char
				// command title.
				c.clrs.command.WritelnByte(c.buf, b0)
			}
			continue
		}

		if length >= 4 {
			// Header lines are prefixed with "--- " or "+++ ".
			if b0 == '-' && line[1] == '-' && line[2] == '-' && line[3] == ' ' {
				c.clrs.header.Writeln(c.buf, line)
				continue
			}

			if b0 == '+' && line[1] == '+' && line[2] == '+' && line[3] == ' ' {
				c.clrs.header.Writeln(c.buf, line)
				continue
			}

			if b0 == '@' && line[1] == '@' && line[2] == ' ' {
				// It's possible that there's commentary after the second @@
				//
				//  @@ -4,7 +4,7 @@ Here is some additional section commentary
				//
				// That commentary should be printed in a different color, so we
				// need to search for it.

				var i int
				for i = 3; i < length; i++ {
					if line[i] == '@' && line[i-1] == '@' {
						break
					}
				}
				if i == length-1 {
					// No commentary after the second @@
					c.clrs.section.Writeln(c.buf, line)
					continue
				}

				c.clrs.section.Write(c.buf, line[:i+1])
				// There's additional commentary after the second @@
				c.clrs.sectionComment.Writeln(c.buf, line[i+1:])
				continue
			}
		}

		switch b0 {
		case '-':
			c.clrs.deletion.Writeln(c.buf, line)
		case '+':
			c.clrs.insertion.Writeln(c.buf, line)
		case ' ':
			c.clrs.context.Writeln(c.buf, line)
		default:
			c.clrs.command.Writeln(c.buf, line)
		}
	}

	if err = c.sc.Err(); err != nil {
		n, _ = c.buf.Read(p)
		return n, err
	}

	return c.buf.Read(p)
}

var prefixSection = []byte("@@")

// ColorizeHunks prints a colorized diff hunks to w. The reader must not include
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
func ColorizeHunks(ctx context.Context, w io.Writer, clrs *Colors, hunks io.Reader) error {
	if hunks == nil {
		return nil
	}

	var err error
	if clrs == nil || clrs.IsMonochrome() {
		_, err = io.Copy(w, hunks)
		return errz.Err(err)
	}

	var (
		printSection = colorz.NewPrinter(clrs.Section).Line
		printMinus   = colorz.NewPrinter(clrs.Deletion).Line
		printPlus    = colorz.NewPrinter(clrs.Insertion).Line
		printNormal  = colorz.NewPrinter(clrs.Context).Line
	)

	sc := scannerz.NewScanner(ctx, hunks)
	var line []byte
	for sc.Scan() {
		if ctx.Err() != nil {
			return errz.Err(ctx.Err())
		}

		line = sc.Bytes()
		switch {
		case len(line) == 0:
			// I'm not sure if this happens, but if it does, print an empty line.
			_, err = w.Write([]byte{newline})
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
