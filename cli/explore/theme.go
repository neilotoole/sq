package explore

import "github.com/charmbracelet/lipgloss"

// theme holds the lipgloss styles the UI uses. Constructing it once
// per program run lets the test suite pass in a no-color theme.
//
//nolint:unused // consumed in Phase 1.3.
type theme struct {
	Title     lipgloss.Style
	Pane      lipgloss.Style
	PaneFocus lipgloss.Style
	Item      lipgloss.Style
	ItemSel   lipgloss.Style
	ItemActiv lipgloss.Style // active source — bold
	Faint     lipgloss.Style
	Error     lipgloss.Style
	Help      lipgloss.Style
}

// newTheme returns a theme. If noColor is true, every style has its
// foreground/background stripped — useful for tests and for
// --no-color / NO_COLOR scenarios.
//
//nolint:unused // consumed in Phase 1.3.
func newTheme(noColor bool) theme {
	if noColor {
		return theme{} // every style is the zero-value (no styling).
	}
	return theme{
		Title:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")),
		Pane:      lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1),
		PaneFocus: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("99")).Padding(0, 1),
		Item:      lipgloss.NewStyle(),
		ItemSel:   lipgloss.NewStyle().Reverse(true),
		ItemActiv: lipgloss.NewStyle().Bold(true),
		Faint:     lipgloss.NewStyle().Faint(true),
		Error:     lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		Help:      lipgloss.NewStyle().Faint(true),
	}
}
