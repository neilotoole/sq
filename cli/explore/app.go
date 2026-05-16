package explore

import (
	"errors"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/neilotoole/sq/libsq/source"
)

// Config is the input to NewModel — everything the TUI needs that
// can't be derived after construction.
type Config struct {
	// FocusedSrc is the source the explorer starts on. Required.
	FocusedSrc *source.Source

	// FocusedTable, if non-empty, opens the named table in the
	// detail pane on launch.
	FocusedTable string

	// Sources are all known sources (typically coll.Sources()).
	Sources []*source.Source

	// PreviewRows is the cap for preview rows. Zero means default (100).
	PreviewRows int

	// EmitHandle, if true, writes the last-focused handle to stdout
	// on quit. The wrapping command performs the write; the model
	// just records the handle in the quitMsg.
	EmitHandle bool

	// NoColor disables every lipgloss style — also implied by NO_COLOR.
	NoColor bool
}

// Model is the root tea.Model. It composes the three panes and routes
// keystrokes to the focused one. Panes are added in later phases; v1
// keeps fields here so subsequent tasks can extend without re-defining
// the struct.
type Model struct {
	focusedSrc *source.Source
	//nolint:unused // populated by later phases when async loads fail.
	lastErr     error
	focusedTbl  string
	finalHandle string
	keys        keyMap
	theme       theme
	cfg         Config
	width       int
	height      int
	quitting    bool
}

// NewModel constructs the root model. Returns an error only if the
// config is invalid (missing FocusedSrc).
func NewModel(cfg Config) (*Model, error) {
	if cfg.FocusedSrc == nil {
		return nil, errors.New("explore: Config.FocusedSrc is required")
	}
	if cfg.PreviewRows == 0 {
		cfg.PreviewRows = 100
	}
	return &Model{
		cfg:        cfg,
		keys:       defaultKeys(),
		theme:      newTheme(cfg.NoColor),
		focusedSrc: cfg.FocusedSrc,
		focusedTbl: cfg.FocusedTable,
	}, nil
}

// FinalHandle returns the address that was focused at quit time,
// formatted as "@src" or "@src.table". Empty until Update returns
// tea.Quit. Callers consult this after tea.Program.Run.
func (m *Model) FinalHandle() string { return m.finalHandle }

// Init satisfies tea.Model. v1 returns nil; later phases dispatch the
// initial fetches here.
func (m *Model) Init() tea.Cmd { return nil }

// Update routes messages. The v1 model handles only the quit and
// window-size paths.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Quit) {
			m.recordFinalHandle()
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// View satisfies tea.Model. Phase 1 prints a single-line placeholder
// that names the focused handle; later phases compose pane views.
func (m *Model) View() string {
	if m.quitting {
		return ""
	}
	title := m.theme.Title.Render("sq explore")
	body := m.theme.Item.Render(m.currentAddress())
	return title + "\n" + body + "\n"
}

// currentAddress returns "@src" or "@src.table" for the current focus.
func (m *Model) currentAddress() string {
	if m.focusedSrc == nil {
		return ""
	}
	if m.focusedTbl == "" {
		return m.focusedSrc.Handle
	}
	return m.focusedSrc.Handle + "." + m.focusedTbl
}

// recordFinalHandle stashes the address to emit on quit. It's a no-op
// when EmitHandle is false.
func (m *Model) recordFinalHandle() {
	if m.cfg.EmitHandle {
		m.finalHandle = m.currentAddress()
	}
}
