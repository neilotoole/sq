package explore

import "github.com/charmbracelet/bubbles/key"

// keyMap holds every key binding the TUI uses. It is the single source
// of truth for both Update routing and the help footer.
type keyMap struct {
	Up        key.Binding
	Down      key.Binding
	Left      key.Binding
	Right     key.Binding
	Top       key.Binding
	Bottom    key.Binding
	Enter     key.Binding
	Space     key.Binding
	Tab       key.Binding
	ShiftTab  key.Binding
	Filter    key.Binding
	ClearFilt key.Binding
	Preview   key.Binding
	Refresh   key.Binding
	Copy      key.Binding
	Help      key.Binding
	Quit      key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Up:        key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
		Down:      key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
		Left:      key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("←/h", "pane left")),
		Right:     key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("→/l", "pane right / expand")),
		Top:       key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
		Bottom:    key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		Enter:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Space:     key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "expand/collapse")),
		Tab:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next pane")),
		ShiftTab:  key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev pane")),
		Filter:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		ClearFilt: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear filter")),
		Preview:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "preview rows")),
		Refresh:   key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "refresh metadata")),
		Copy:      key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy handle")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}
