package libdiff

import (
	"github.com/fatih/color"
	"github.com/msoap/byline"
	"io"
)

// Printing encapsulates diff printing config.
type Printing struct {
	// Title is the color for diff title elements. The title is not actually part
	// of a standard diff. We use it when multiple diffs are printed back-to-back.
	Title *color.Color

	// Header is the color for diff header elements.
	//
	//  --- @diff/sakila_a.actor
	//  +++ @diff/sakila_b.actor
	//
	// A header should appear at the top of each diff.
	Header *color.Color

	// Section is the color for diff hunk section range elements.
	//
	//  @@ -8,9 +8,9 @@
	//
	// The above is a section.
	Section *color.Color

	// SectionComment is the color for (option) diff hunk section comments.
	//
	//  @@ -8,9 +8,9 @@ Here's some context.
	//
	// The text after the second @@ is a section comment.
	SectionComment *color.Color

	// Insertion is the color for diff plus "+" elements.
	Insertion *color.Color

	// Deletion is the color for diff minus "-" elements.
	Deletion *color.Color

	// Normal is the color for regular diff text.
	Normal *color.Color

	// monochrome is controlled by EnableColor.
	monochrome bool

	// ShowHeader indicates that a header (e.g. a header row) should
	// be printed where applicable.
	//
	// REVISIT: Printing.ShowHeader may not be needed.
	ShowHeader bool
}

// NewPrinting returns a Printing instance. Color is enabled by default.
func NewPrinting() *Printing {
	pr := &Printing{
		ShowHeader:     true,
		monochrome:     false,
		Title:          color.New(color.FgYellow), // FIXME: choose a better color
		Header:         color.New(color.Bold),
		Deletion:       color.New(color.FgRed),
		Normal:         color.New(color.Faint),
		Insertion:      color.New(color.FgGreen),
		Section:        color.New(color.FgCyan),
		SectionComment: color.New(color.FgCyan, color.Faint), // FIXME: make use of SectionComment
	}

	pr.EnableColor(true)
	return pr
}

// Clone returns a clone of pr.
func (pr *Printing) Clone() *Printing {
	pr2 := &Printing{
		monochrome: pr.monochrome,
		ShowHeader: pr.ShowHeader,
	}

	pr2.Title = toPtr(*pr.Title)
	pr2.Header = toPtr(*pr.Header)
	pr2.Section = toPtr(*pr.Section)
	pr2.SectionComment = toPtr(*pr.SectionComment)
	pr2.Deletion = toPtr(*pr.Deletion)
	pr2.Insertion = toPtr(*pr.Insertion)
	pr2.Normal = toPtr(*pr.Normal)

	return pr2
}

func (pr *Printing) colors() []*color.Color {
	return []*color.Color{
		pr.Title,
		pr.Header,
		pr.Section,
		pr.SectionComment,
		pr.Deletion,
		pr.Insertion,
		pr.Normal,
	}
}

// IsMonochrome returns true if in monochrome (no color) mode.
// Default is false (color enabled) for a new instance.
func (pr *Printing) IsMonochrome() bool {
	return pr.monochrome
}

// EnableColor enables or disables all colors.
func (pr *Printing) EnableColor(enable bool) {
	if enable {
		pr.monochrome = false

		for _, clr := range pr.colors() {
			clr.EnableColor()
		}
		return
	}

	pr.monochrome = true
	for _, clr := range pr.colors() {
		clr.DisableColor()
	}
}

// toPtr returns a pointer copy of value.
func toPtr[T any](x T) *T {
	return &x
}

func NewColorizer(pr *Printing, src io.Reader) io.Reader {
	lr := byline.NewReader(src)
	return lr
}

var (
	newline       = []byte{'\n'}
	prefixMinuses = []byte("---")
	prefixPluses  = []byte("+++")
	prefixSection = []byte("@@")
)

//func colorizeLine(pr *Printing, line []byte) []byte {
//	switch {
//	case len(line) == 0:
//		return line
//	case line[0] == '-':
//		return stringz.UnsafeBytes(pr.Deletion.Sprint(stringz.UnsafeString(line)))
//	case line[0] == '+':
//		return stringz.UnsafeBytes(pr.Insertion.Sprint(stringz.UnsafeString(line)))
//	case line[0] == ' ':
//		return stringz.UnsafeBytes(pr.Normal.Sprint(stringz.UnsafeString(line)))
//	case line[0] == '@':
//		return stringz.UnsafeBytes(pr.Section.Sprint(stringz.UnsafeString(line)))
//	case bytes.HasPrefix(line, prefixMinuses), bytes.HasPrefix(line, prefixPluses):
//		return stringz.UnsafeBytes(pr.Header.Sprint(stringz.UnsafeString(line)))
//	case bytes.HasPrefix(line, prefixSection):
//		return stringz.UnsafeBytes(pr.Section.Sprint(stringz.UnsafeString(line)))
//
//	default:
//		_, err = printNormal(w, line)
//	}
//}

//
//func NewColorizer(ctx context.Context, pr *Printing, src io.Reader) io.Reader {
//	return &colorizer{ctx: ctx, pr: pr, src: src, buf: &bytes.Buffer{}, sc: bufio.NewScanner(src)}
//}
//
//var _ io.Reader = (*colorizer)(nil)
//
//type colorizer struct {
//	ctx context.Context
//	pr  *Printing
//	src io.Reader
//	buf *bytes.Buffer
//	sc  *bufio.Scanner
//}
//
//func (c *colorizer) Read(p []byte) (n int, err error) {
//	if c.buf.Len() >= len(p) {
//		return c.buf.Read(p)
//	}
//
//	var line []byte
//	_ = line
//
//	for c.buf.Len() < len(p) && c.sc.Scan() {
//		line = c.sc.Bytes()
//		if len(line) == 0 {
//			err = c.buf.WriteByte('\n')
//			if err != nil {
//				break
//			}
//			continue
//		}
//
//		// do colorization here
//		_, err = c.buf.Write(line)
//		if err != nil {
//			break
//		}
//		err = c.buf.WriteByte('\n')
//		if err != nil {
//			break
//		}
//	}
//
//	if err == nil {
//		return c.buf.Read(p)
//	}
//
//	// We've got an error situation here.
//
//	return 0, nil
//}
//
//func (c *colorizer) readPassthrough(p []byte) (n int, err error) {
//	for {
//		if c.buf.Len() >= len(p) {
//			return c.buf.Read(p)
//		}
//
//		n, err = c.src.Read(p)
//		if n > 0 {
//			c.buf.Write(p[:n])
//		}
//
//		if err != nil {
//			n, _ = c.buf.Read(p)
//			if n == len(p) {
//				return n, nil
//			}
//			return n, err
//		}
//	}
//}
