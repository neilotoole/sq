package explore

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/neilotoole/sq/cli/run"
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

// paneID enumerates the focusable panes. They cycle left-to-right.
type paneID int

const (
	paneSources paneID = iota
	paneSchema
	paneDetail

	numPanes = 3
)

// Model is the root tea.Model. It composes the three panes and routes
// keystrokes to the focused one. Panes are added in later phases; v1
// keeps fields here so subsequent tasks can extend without re-defining
// the struct.
type Model struct {
	focusedSrc *source.Source
	lastErr    error
	// ru is set by RunWithIO and consumed by Phase 4+ for metadata fetches.
	ru          *run.Run
	sources     *sourcesPane
	schema      *schemaTree
	detail      *detailPane
	fetcher     metaFetcher
	focusedTbl  string
	finalHandle string
	keys        keyMap
	theme       theme
	cfg         Config
	width       int
	height      int
	focused     paneID
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
	m := &Model{
		cfg:        cfg,
		keys:       defaultKeys(),
		theme:      newTheme(cfg.NoColor),
		focusedSrc: cfg.FocusedSrc,
		focusedTbl: cfg.FocusedTable,
	}
	m.sources = newSourcesPane(cfg.Sources, cfg.FocusedSrc, m.theme)
	m.schema = newSchemaTree(cfg.FocusedSrc.Handle, m.theme)
	m.detail = newDetailPane(m.theme)
	return m, nil
}

// FinalHandle returns the address that was focused at quit time,
// formatted as "@src" or "@src.table". Empty until Update returns
// tea.Quit. Callers consult this after tea.Program.Run.
func (m *Model) FinalHandle() string { return m.finalHandle }

// Init satisfies tea.Model. It dispatches the initial source-overview
// and table-names fetches for the focused source.
func (m *Model) Init() tea.Cmd {
	if m.fetcher == nil {
		return nil
	}
	return tea.Batch(
		fetchSourceOverviewCmd(context.Background(), m.fetcher, m.focusedSrc.Handle),
		fetchTableNamesCmd(context.Background(), m.fetcher, m.focusedSrc.Handle),
	)
}

// Update routes messages. The v1 model handles only the quit and
// window-size paths.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case sourceOverviewLoadedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			return m, nil
		}
		m.detail.setSource(msg.meta)
		return m, nil
	case tableNamesLoadedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			return m, nil
		}
		m.schema.setTableNames(msg.names)
		return m, nil
	case tableMetaLoadedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			return m, nil
		}
		m.schema.setTableMeta(msg.tableName, msg.meta)
		if m.focusedTbl == msg.tableName && m.detail.kind != detailColumn {
			m.detail.setTable(msg.meta)
		}
		return m, nil
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Quit) {
			m.recordFinalHandle()
			m.quitting = true
			return m, tea.Quit
		}
		if key.Matches(msg, m.keys.Tab) {
			m.focused = (m.focused + 1) % numPanes
			return m, nil
		}
		if key.Matches(msg, m.keys.ShiftTab) {
			m.focused = (m.focused + numPanes - 1) % numPanes
			return m, nil
		}
		return m, m.routeKey(msg)
	}
	return m, nil
}

// View satisfies tea.Model. It composes the three panes into a single
// frame.
func (m *Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 || m.height == 0 {
		// First frame before a WindowSizeMsg arrives.
		return ""
	}

	col := m.width / numPanes
	body := m.height - 2 // 1 for title, 1 for help footer.

	srcCol := m.sources.view(m.focused == paneSources, col, body)
	schCol := m.schema.view(m.focused == paneSchema, col, body)
	detCol := m.detail.view(m.focused == paneDetail, m.width-2*col, body)

	row := lipgloss.JoinHorizontal(lipgloss.Top, srcCol, schCol, detCol)
	return m.theme.Title.Render("sq explore") + "\n" + row + "\n"
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

// routeKey dispatches a non-global key message to the focused pane.
// Later phases extend the switch with the paneDetail case.
func (m *Model) routeKey(msg tea.KeyMsg) tea.Cmd {
	switch m.focused {
	case paneSources:
		m.sources.update(msg, m.keys)
		newSrc := m.sources.selectedSource()
		if newSrc != m.focusedSrc {
			m.focusedSrc = newSrc
			m.focusedTbl = ""
			m.schema = newSchemaTree(newSrc.Handle, m.theme)
			if m.fetcher != nil {
				return tea.Batch(
					fetchSourceOverviewCmd(context.Background(), m.fetcher, newSrc.Handle),
					fetchTableNamesCmd(context.Background(), m.fetcher, newSrc.Handle),
				)
			}
		}
		return nil
	case paneSchema:
		needsFetch, tblName := m.schema.update(msg, m.keys)
		if needsFetch && m.fetcher != nil {
			return fetchTableMetaCmd(context.Background(), m.fetcher, m.focusedSrc.Handle, tblName)
		}
		if key.Matches(msg, m.keys.Enter) {
			tbl, col := m.schema.selectedDetail()
			switch {
			case col != nil:
				m.detail.setColumn(col)
			case tbl != nil:
				m.detail.setTable(tbl)
				m.focusedTbl = tbl.Name
			}
		} else if t := m.schema.selectedTableName(); t != "" {
			m.focusedTbl = t
		}
		return nil
	case paneDetail:
		// Detail pane key handling lands in Phase 5.
		return nil
	}
	return nil
}

// Run starts the explore TUI and blocks until the user quits or the
// context is cancelled. It returns the last-focused handle (empty
// unless Config.EmitHandle was true).
func Run(ctx context.Context, ru *run.Run, cfg Config) (finalHandle string, err error) {
	return RunWithIO(ctx, ru, cfg, os.Stdout, os.Stderr)
}

// RunWithIO is Run with explicit I/O so tests can substitute writers.
// ru may be nil in tests that exercise only the model.
func RunWithIO(ctx context.Context, ru *run.Run, cfg Config, out, errOut io.Writer) (string, error) {
	_ = errOut // reserved for future error/log routing.
	m, err := NewModel(cfg)
	if err != nil {
		return "", err
	}
	m.ru = ru
	if ru != nil {
		m.fetcher = newRunFetcher(ru)
	}

	opts := []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithOutput(out),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	}
	// When stdin isn't a TTY (e.g. under `go test`), disable input
	// rather than letting bubbletea fall through to opening /dev/tty,
	// which fails in non-interactive environments.
	if !isTerminal(os.Stdin) {
		opts = append(opts, tea.WithInput(nil))
	}

	p := tea.NewProgram(m, opts...)
	finalModel, err := p.Run()
	if err != nil && !errors.Is(err, context.Canceled) &&
		!errors.Is(err, tea.ErrProgramKilled) {
		return "", err
	}
	fm, ok := finalModel.(*Model)
	if !ok {
		return "", errors.New("explore: final model has unexpected type")
	}
	return fm.FinalHandle(), nil
}

// isTerminal reports whether f is a terminal. It exists so RunWithIO
// can degrade gracefully under `go test`, where stdin is a pipe or
// /dev/null.
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
