package diffdoc

import (
	"github.com/fatih/color"

	"github.com/neilotoole/sq/libsq/core/colorz"
)

// Colors encapsulates diff color printing config.
type Colors struct {
	// CmdTitle is the color for the diff command text. That is, the text of the
	// command that effectively triggered this diff. For example:
	//
	//  diff -U3 -r ./a/hiawatha.txt ./b/hiawatha.txt
	//
	// The command text is typically only displayed when multiple diffs are
	// printed back-to-back.
	CmdTitle *color.Color

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

	// SectionComment is the color for (optional) diff hunk section comments.
	//
	//  @@ -8,9 +8,9 @@ Here's some context.
	//
	// The text after the second @@ is a section comment.
	SectionComment *color.Color

	// Insertion is the color for diff insertion "+" elements.
	Insertion *color.Color

	// Deletion is the color for diff deletion "-" elements.
	Deletion *color.Color

	// Context is the color for context lines, i.e. the lines above and below
	// the insertions and deletions.
	Context *color.Color

	// monochrome is controlled by EnableColor.
	monochrome bool

	// ShowHeader indicates that a header (e.g. a header row) should
	// be printed where applicable.
	//
	// REVISIT: Colors.ShowHeader may not be needed.
	ShowHeader bool
}

// NewColors returns a Colors instance. Coloring is enabled by default.
func NewColors() *Colors {
	c := &Colors{
		ShowHeader:     true,
		monochrome:     false,
		CmdTitle:       color.New(color.FgBlue),
		Header:         color.New(color.Bold),
		Deletion:       color.New(color.FgRed),
		Context:        color.New(color.Faint),
		Insertion:      color.New(color.FgGreen),
		Section:        color.New(color.FgHiMagenta, color.Bold),
		SectionComment: color.New(color.FgBlue),
	}

	c.EnableColor(true)
	return c
}

// Clone returns a clone of c.
func (c *Colors) Clone() *Colors {
	c2 := &Colors{
		monochrome: c.monochrome,
		ShowHeader: c.ShowHeader,
	}

	c2.CmdTitle = toPtr(*c.CmdTitle)
	c2.Header = toPtr(*c.Header)
	c2.Section = toPtr(*c.Section)
	c2.SectionComment = toPtr(*c.SectionComment)
	c2.Deletion = toPtr(*c.Deletion)
	c2.Insertion = toPtr(*c.Insertion)
	c2.Context = toPtr(*c.Context)

	return c2
}

func (c *Colors) colors() []*color.Color {
	return []*color.Color{
		c.CmdTitle,
		c.Header,
		c.Section,
		c.SectionComment,
		c.Deletion,
		c.Insertion,
		c.Context,
	}
}

// IsMonochrome returns true if in monochrome (no color) mode.
// Default is false (color enabled) for a new instance.
func (c *Colors) IsMonochrome() bool {
	return c.monochrome
}

// EnableColor enables or disables all colors.
func (c *Colors) EnableColor(enable bool) {
	if enable {
		c.monochrome = false

		for _, clr := range c.colors() {
			clr.EnableColor()
		}
		return
	}

	c.monochrome = true
	for _, clr := range c.colors() {
		clr.DisableColor()
	}
}

func (c *Colors) codes() *codes {
	return &codes{
		command:        colorz.ExtractSeqs(c.CmdTitle),
		header:         colorz.ExtractSeqs(c.Header),
		section:        colorz.ExtractSeqs(c.Section),
		sectionComment: colorz.ExtractSeqs(c.SectionComment),
		insertion:      colorz.ExtractSeqs(c.Insertion),
		deletion:       colorz.ExtractSeqs(c.Deletion),
		context:        colorz.ExtractSeqs(c.Context),
	}
}

type codes struct {
	command        colorz.Seqs
	header         colorz.Seqs
	section        colorz.Seqs
	sectionComment colorz.Seqs
	insertion      colorz.Seqs
	deletion       colorz.Seqs
	context        colorz.Seqs
}

// toPtr returns a pointer copy of value.
func toPtr[T any](x T) *T {
	return &x
}
