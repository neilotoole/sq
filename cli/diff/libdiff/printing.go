package libdiff

import (
	"github.com/fatih/color"
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

	// Inserted is the color for diff plus "+" elements.
	Inserted *color.Color

	// Deleted is the color for diff minus "-" elements.
	Deleted *color.Color

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
		Deleted:        color.New(color.FgRed),
		Normal:         color.New(color.Faint),
		Inserted:       color.New(color.FgGreen),
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
	pr2.Deleted = toPtr(*pr.Deleted)
	pr2.Inserted = toPtr(*pr.Inserted)
	pr2.Normal = toPtr(*pr.Normal)

	return pr2
}

func (pr *Printing) colors() []*color.Color {
	return []*color.Color{
		pr.Title,
		pr.Header,
		pr.Section,
		pr.SectionComment,
		pr.Deleted,
		pr.Inserted,
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
