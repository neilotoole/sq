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
	Title     lipgloss.Style // pane titles ("Sources", "Schema (…)") — blue + bold, like sq's Header.
	Pane      lipgloss.Style // unfocused pane border.
	PaneFocus lipgloss.Style // focused pane border (hi-blue, like sq's Hilite).
	Item      lipgloss.Style // default list item.
	ItemSel   lipgloss.Style // selected list item (reverse video for visibility regardless of palette).
	ItemActiv lipgloss.Style // active source — green + bold, like sq's Active.
	Handle    lipgloss.Style // source handle — blue, like sq's Handle.
	Number    lipgloss.Style // numeric values — cyan, like sq's Number.
	Location  lipgloss.Style // location URLs / paths — green, like sq's Location.
	Faint     lipgloss.Style // de-emphasized text — faint, like sq's Faint.
	Error     lipgloss.Style // error messages — red + bold.
	Help      lipgloss.Style // the always-visible top help line.
}

// newTheme returns a theme. If noColor is true, every style has its
// foreground/background stripped — useful for tests and for
// --no-color / NO_COLOR scenarios.
func newTheme(noColor bool) theme {
	if noColor {
		return theme{} // every style is the zero-value (no styling).
	}
	return theme{
		Title: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorBlue)),
		Pane: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(colorGray)).
			Padding(0, 1),
		PaneFocus: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorHiBlue)).
			Padding(0, 1),
		Item:      lipgloss.NewStyle(),
		ItemSel:   lipgloss.NewStyle().Reverse(true),
		ItemActiv: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorGreen)),
		Handle:    lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlue)),
		Number:    lipgloss.NewStyle().Foreground(lipgloss.Color(colorCyan)),
		Location:  lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen)),
		Faint:     lipgloss.NewStyle().Faint(true),
		Error:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorRed)),
		Help:      lipgloss.NewStyle().Faint(true),
	}
}
