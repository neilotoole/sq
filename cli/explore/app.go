package explore

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/record"
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
	previewFn   previewFunc
	preview     *previewBuffer
	sendFn      func(tea.Msg)
	copyFn      func(string) error
	focusedTbl  string
	finalHandle string
	filterBuf   string
	keys        keyMap
	theme       theme
	cfg         Config
	width       int
	height      int
	focused     paneID
	quitting    bool
	filtering   bool
}

// previewBuffer holds in-memory preview rows for the focused table.
// Fields are mutated only on the tea event loop goroutine (in Update).
type previewBuffer struct {
	err       error
	handle    string
	tableName string
	meta      record.Meta
	rows      []record.Record
	done      bool
}

// previewStartedMsg is the completion message for the Cmd that kicks
// off the preview goroutine. It carries no state — the actual data
// arrives via subsequent previewMetaLoadedMsg / previewRowsAppendedMsg
// dispatched by previewWriter through sendFn.
type previewStartedMsg struct {
	handle    string
	tableName string
	count     int
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
	m.copyFn = clipboard.WriteAll
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
		if m.acceptFetch(msg.handle, msg.err) {
			m.detail.setSource(msg.meta)
		}
		return m, nil
	case tableNamesLoadedMsg:
		if m.acceptFetch(msg.handle, msg.err) {
			m.schema.setTableNames(msg.names)
		}
		return m, nil
	case tableMetaLoadedMsg:
		if m.acceptFetch(msg.handle, msg.err) {
			m.schema.setTableMeta(msg.tableName, msg.meta)
			if m.focusedTbl == msg.tableName && m.detail.kind != detailColumn {
				m.detail.setTable(msg.meta)
			}
		}
		return m, nil
	case previewMetaLoadedMsg:
		if m.preview == nil || m.preview.tableName != msg.tableName || m.preview.handle != msg.handle {
			m.preview = &previewBuffer{handle: msg.handle, tableName: msg.tableName}
		}
		m.preview.meta = msg.recMeta
		m.detail.setPreview(m.preview)
		return m, nil
	case previewRowsAppendedMsg:
		if m.preview == nil || m.preview.tableName != msg.tableName {
			return m, nil
		}
		m.preview.rows = append(m.preview.rows, msg.rows...)
		m.preview.done = msg.done
		m.detail.setPreview(m.preview)
		return m, nil
	case previewErrMsg:
		if m.preview != nil && m.preview.tableName == msg.tableName {
			m.preview.err = msg.err
			m.detail.setPreview(m.preview)
		}
		return m, nil
	case previewStartedMsg:
		return m, nil
	case tea.KeyMsg:
		if m.filtering {
			switch msg.Type { //nolint:exhaustive // we only care about a handful of key types in filter mode.
			case tea.KeyEnter:
				m.filtering = false
				return m, nil
			case tea.KeyEsc:
				m.filtering = false
				m.filterBuf = ""
				m.applyFilter("")
				return m, nil
			case tea.KeyBackspace:
				if len(m.filterBuf) > 0 {
					m.filterBuf = m.filterBuf[:len(m.filterBuf)-1]
					m.applyFilter(m.filterBuf)
				}
				return m, nil
			case tea.KeyRunes:
				m.filterBuf += string(msg.Runes)
				m.applyFilter(m.filterBuf)
				return m, nil
			}
		}
		if key.Matches(msg, m.keys.Quit) {
			m.recordFinalHandle()
			m.quitting = true
			return m, tea.Quit
		}
		if key.Matches(msg, m.keys.Filter) {
			m.filtering = true
			m.filterBuf = ""
			return m, nil
		}
		if key.Matches(msg, m.keys.ClearFilt) {
			m.filterBuf = ""
			m.applyFilter("")
			return m, nil
		}
		if key.Matches(msg, m.keys.Tab) {
			m.focused = (m.focused + 1) % numPanes
			return m, nil
		}
		if key.Matches(msg, m.keys.ShiftTab) {
			m.focused = (m.focused + numPanes - 1) % numPanes
			return m, nil
		}
		if key.Matches(msg, m.keys.Refresh) {
			if m.fetcher != nil && m.focusedSrc != nil {
				m.schema = newSchemaTree(m.focusedSrc.Handle, m.theme)
				return m, refreshSourceCmd(context.Background(), m.fetcher, m.focusedSrc.Handle)
			}
			return m, nil
		}
		if key.Matches(msg, m.keys.Copy) {
			m.copyCurrentAddress()
			return m, nil
		}
		return m, m.routeKey(msg)
	}
	return m, nil
}

// Width thresholds for the three-mode View layout.
const (
	widthFullThreshold  = 100 // below this, drop sources pane to a header.
	widthStackThreshold = 70  // below this, stack vertically.
)

// View satisfies tea.Model. It composes the panes into a single frame,
// choosing a layout based on terminal width.
func (m *Model) View() string {
	if m.quitting || m.width == 0 || m.height == 0 {
		return ""
	}

	body := m.height - 2 // 1 for title, 1 for help footer.

	var row string
	switch {
	case m.width >= widthFullThreshold:
		row = m.viewWide(body)
	case m.width >= widthStackThreshold:
		row = m.viewCompact(body)
	default:
		row = m.viewStacked(body)
	}

	top := m.theme.Help.Render(m.helpLine())
	if m.filtering {
		top += "  " + m.theme.Title.Render("/"+m.filterBuf+"▏")
	}
	return top + "\n" + row
}

// viewWide is the three-pane side-by-side layout used when the terminal
// is at least widthFullThreshold columns wide.
func (m *Model) viewWide(body int) string {
	col := m.width / numPanes
	srcCol := m.sources.view(m.focused == paneSources, col, body)
	schCol := m.schema.view(m.focused == paneSchema, col, body)
	detCol := m.detail.view(m.focused == paneDetail, m.width-2*col, body)
	return lipgloss.JoinHorizontal(lipgloss.Top, srcCol, schCol, detCol)
}

// viewCompact drops the sources pane to a header line and shows the
// schema + detail panes side by side.
func (m *Model) viewCompact(body int) string {
	head := m.theme.Faint.Render("source: " + m.currentAddress())
	col := m.width / 2
	schCol := m.schema.view(m.focused == paneSchema, col, body-1)
	detCol := m.detail.view(m.focused == paneDetail, m.width-col, body-1)
	return head + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, schCol, detCol)
}

// viewStacked stacks the header, schema, and detail panes vertically
// for very narrow terminals.
func (m *Model) viewStacked(body int) string {
	schH := (body - 1) / 2
	detH := body - 1 - schH
	head := m.theme.Faint.Render("source: " + m.currentAddress())
	schView := m.schema.view(m.focused == paneSchema, m.width, schH)
	detView := m.detail.view(m.focused == paneDetail, m.width, detH)
	return head + "\n" + schView + "\n" + detView
}

// helpLine returns the always-visible top-line key summary. It is
// kept concise so it fits on a standard terminal width without wrap.
func (m *Model) helpLine() string {
	return strings.Join([]string{
		"j/k nav",
		"h/l pane",
		"tab cycle",
		"enter open",
		"space expand",
		"/ filter",
		"r preview",
		"R refresh",
		"y copy",
		"q quit",
	}, "  ")
}

// applyFilter pushes the filter string to whichever pane is focused.
func (m *Model) applyFilter(f string) {
	switch m.focused {
	case paneSources:
		m.sources.setFilter(f)
	case paneSchema:
		m.schema.setFilter(f)
	case paneDetail:
		// no-op: detail pane doesn't support filtering yet.
	}
}

// copyCurrentAddress writes the current address to the system clipboard
// via m.copyFn. It is best-effort: errors are swallowed so the TUI keeps
// running on platforms where no clipboard is available.
func (m *Model) copyCurrentAddress() {
	addr := m.currentAddress()
	if addr == "" || m.copyFn == nil {
		return
	}
	_ = m.copyFn(addr)
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

// acceptFetch decides whether a fetch-result message should be applied
// to model state. It records the error and returns false when the
// fetch failed, when no source is focused, or when the message is for
// a source the user has already moved past.
func (m *Model) acceptFetch(handle string, err error) bool {
	if err != nil {
		m.lastErr = err
		return false
	}
	return m.focusedSrc != nil && handle == m.focusedSrc.Handle
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
			// Reset the detail pane so the previous source's data
			// doesn't linger while the new fetch is in flight.
			m.detail = newDetailPane(m.theme)
			m.preview = nil
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
		if key.Matches(msg, m.keys.Preview) {
			return m.startPreview()
		}
		return nil
	case paneDetail:
		if key.Matches(msg, m.keys.Preview) {
			return m.startPreview()
		}
		return nil
	}
	return nil
}

// startPreview begins (or restarts) the preview for the currently
// focused table. The returned tea.Cmd launches the previewFn goroutine;
// that goroutine delivers previewMetaLoadedMsg / previewRowsAppendedMsg /
// previewErrMsg back into the tea runtime via m.sendFn (captured from
// tea.Program at Run time).
func (m *Model) startPreview() tea.Cmd {
	if m.previewFn == nil || m.focusedTbl == "" {
		return nil
	}
	handle, table := m.focusedSrc.Handle, m.focusedTbl
	n := m.cfg.PreviewRows
	m.preview = &previewBuffer{handle: handle, tableName: table}
	m.detail.setPreview(m.preview)
	send := m.sendFn
	if send == nil {
		send = func(_ tea.Msg) {}
	}
	previewFn := m.previewFn
	return func() tea.Msg {
		go previewFn(context.Background(), func(msg any) {
			send(msg.(tea.Msg))
		}, handle, table, n)
		return previewStartedMsg{handle: handle, tableName: table, count: n}
	}
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
		rf := newRunFetcher(ru)
		m.fetcher = rf
		m.previewFn = rf.runPreview
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
	// Capture p.Send so async dispatchers (e.g. previewWriter) can
	// inject messages into the runtime.
	m.sendFn = func(msg tea.Msg) { p.Send(msg) }
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
