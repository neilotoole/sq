package libdiff

import (
	"github.com/fatih/color"
)

// Printing encapsulates diff printing config.
type Printing struct {
	// Title is the color for diff title elements.
	Title *color.Color

	// Header is the color for diff header elements.
	Header *color.Color

	// Section is the color for diff section elements.
	Section *color.Color

	// Plus is the color for diff plus "+" elements.
	Plus *color.Color

	// Minus is the color for diff minus "-" elements.
	Minus *color.Color

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
		ShowHeader: true,
		monochrome: false,
		Title:      color.New(color.FgYellow), // FIXME: choose a better color
		Header:     color.New(color.Bold),
		Minus:      color.New(color.FgRed),
		Normal:     color.New(color.Faint),
		Plus:       color.New(color.FgGreen),
		Section:    color.New(color.FgCyan),
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
	pr2.Minus = toPtr(*pr.Minus)
	pr2.Plus = toPtr(*pr.Plus)
	pr2.Normal = toPtr(*pr.Normal)

	return pr2
}

func (pr *Printing) colors() []*color.Color {
	return []*color.Color{
		pr.Title, pr.Header, pr.Section, pr.Minus, pr.Plus, pr.Normal,
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
