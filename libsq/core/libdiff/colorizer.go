package libdiff

import (
	"bufio"
	"bytes"
	"io"
)

//var (
//	prefixMinuses = []byte("---")
//	prefixPluses  = []byte("+++")
//	prefixSection = []byte("@@")
//)

type colorizer struct {
	sc      *bufio.Scanner
	buf     *bytes.Buffer
	clrs    codes
	hasData bool
}

func NewColorizer(clrs *Colors, src io.Reader) io.Reader {
	if src == nil || clrs == nil || clrs.IsMonochrome() {
		return src
	}

	c := &colorizer{
		sc:      bufio.NewScanner(src),
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
