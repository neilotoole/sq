package explore

import "github.com/charmbracelet/lipgloss"

// ANSI color indices, chosen to mirror the palette in [output.Printing]
// so the TUI matches sq's existing text output.
const (
	colorBlue   = "4"  // matches color.FgBlue   (Header / Handle).
	colorGreen  = "2"  // matches color.FgGreen  (Active / Location / String).
	colorCyan   = "6"  // matches color.FgCyan   (Number).
	colorRed    = "1"  // matches color.FgRed    (Error).
	colorHiBlue = "12" // matches color.FgHiBlue (Hilite).
	colorGray   = "8"  // dim foreground for unfocused pane border.
)

// theme holds the lipgloss styles the UI uses. Constructing it once
// per program run lets the test suite pass in a no-color theme.
type theme struct {
	// Title styles pane titles ("Sources", "Schema (…)") — blue + bold,
	// like sq's Header.
	Title lipgloss.Style
	// TitleFocus styles the focused pane's title (hi-blue, like sq's
	// Hilite). Panes are borderless, so the title carries the focus
	// signal (the inspector, which has no title, uses DividerFocus).
	TitleFocus lipgloss.Style
	// Pane is the borderless body style shared by all panes; panes are
	// separated by Divider rules, not box borders.
	Pane lipgloss.Style
	// Item is the default list-item style.
	Item lipgloss.Style
	// ItemSel styles the selected row in the focused pane (reverse).
	ItemSel lipgloss.Style
	// ItemCursor styles the selected row in an unfocused pane so the
	// cursor position is still visible (underline + faint).
	ItemCursor lipgloss.Style
	// ItemActiv styles the active source — green + bold, like sq's Active.
	ItemActiv lipgloss.Style
	// Handle is a source handle — blue, like sq's Handle.
	Handle lipgloss.Style
	// Number is numeric values — cyan, like sq's Number.
	Number lipgloss.Style
	// Location is for source location URLs / paths — green, like sq's Location.
	Location lipgloss.Style
	// Divider styles the rules separating panes: full-height "│"
	// columns in horizontal layouts, a "─" row in the stacked layout.
	Divider lipgloss.Style
	// DividerFocus styles the divider when the inspector has focus. It
	// stands in for the focused border the other panes use.
	DividerFocus lipgloss.Style
	// Faint de-emphasizes text — like sq's Faint.
	Faint lipgloss.Style
	// Error styles error messages — red + bold.
	Error lipgloss.Style
	// Help styles the always-visible top help line.
	Help lipgloss.Style
}

// newTheme returns a theme. If noColor is true, every style has its
// foreground/background stripped — useful for tests and for
// --no-color / NO_COLOR scenarios.
func newTheme(noColor bool) theme {
	if noColor {
		return theme{} // every style is the zero-value (no styling).
	}
	return theme{
		Title:        lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorBlue)),
		TitleFocus:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorHiBlue)),
		Pane:         lipgloss.NewStyle().Padding(0, 1),
		Item:         lipgloss.NewStyle(),
		ItemSel:      lipgloss.NewStyle().Reverse(true),
		ItemCursor:   lipgloss.NewStyle().Underline(true).Faint(true),
		ItemActiv:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorGreen)),
		Handle:       lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlue)),
		Number:       lipgloss.NewStyle().Foreground(lipgloss.Color(colorCyan)),
		Location:     lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen)),
		Divider:      lipgloss.NewStyle().Foreground(lipgloss.Color(colorGray)),
		DividerFocus: lipgloss.NewStyle().Foreground(lipgloss.Color(colorHiBlue)),
		Faint:        lipgloss.NewStyle().Faint(true),
		Error:        lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorRed)),
		Help:         lipgloss.NewStyle().Faint(true),
	}
}
