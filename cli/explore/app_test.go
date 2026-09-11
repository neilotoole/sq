package explore

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// helper: build a *Model with a fixed source and no I/O.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	src := &source.Source{Handle: "@test"}
	cfg := Config{
		Sources:    []*source.Source{src},
		FocusedSrc: src,
		NoColor:    true,
	}
	m, err := NewModel(cfg)
	require.NoError(t, err)
	m.sendFn = func(msg tea.Msg) { m.Update(msg) }
	return m
}

func TestModel_QuitOnQ(t *testing.T) {
	m := newTestModel(t)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestModel_QuitOnCtrlC(t *testing.T) {
	m := newTestModel(t)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestModel_View_ContainsHandle(t *testing.T) {
	m := newTestModel(t)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("@test"))
	}, teatest.WithDuration(2*time.Second), teatest.WithCheckInterval(10*time.Millisecond))
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestModel_TabCyclesPanes(t *testing.T) {
	// Drive Update synchronously rather than via teatest: reading
	// m.focused while the teatest program mutates it on its own
	// goroutine is a data race (caught by `go test -race`).
	m := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	require.Equal(t, paneSources, m.focused)

	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.Equal(t, paneSchema, m.focused)

	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.Equal(t, paneDetail, m.focused)

	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.Equal(t, paneSources, m.focused)

	// shift+tab cycles backward.
	m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	require.Equal(t, paneDetail, m.focused)
}

func TestModel_View_ContainsAllSources(t *testing.T) {
	srcs := mkSources("@one", "@two", "@three")
	cfg := Config{
		Sources:    srcs,
		FocusedSrc: srcs[0],
		NoColor:    true,
	}
	m, err := NewModel(cfg)
	require.NoError(t, err)

	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	out := m.View()
	for _, h := range []string{"@one", "@two", "@three"} {
		require.True(t, strings.Contains(out, h), "view should list %q; got: %s", h, out)
	}
}

func TestModel_SourcesPane_DownMovesSelection(t *testing.T) {
	srcs := mkSources("@a", "@b")
	cfg := Config{Sources: srcs, FocusedSrc: srcs[0], NoColor: true}
	m, _ := NewModel(cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	require.Equal(t, srcs[0], m.sources.selectedSource())
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	require.Equal(t, srcs[1], m.sources.selectedSource())
}

func TestModel_Init_DispatchesTableNamesFetch(t *testing.T) {
	src := &source.Source{Handle: "@x"}
	cfg := Config{Sources: []*source.Source{src}, FocusedSrc: src, NoColor: true}
	m, _ := NewModel(cfg)
	m.fetcher = &fakeFetcher{tableNames: map[string][]string{"@x": {"a"}}}

	cmd := m.Init()
	require.NotNil(t, cmd, "Init should dispatch initial fetches")
	// Init returns tea.Batch, which produces a tea.BatchMsg containing
	// the sub-Cmds. Sanity-check that calling it doesn't panic and
	// produces a non-nil message.
	msg := cmd()
	require.NotNil(t, msg)
}

func TestModel_TableNamesLoaded_PopulatesSchemaPane(t *testing.T) {
	src := &source.Source{Handle: "@x"}
	cfg := Config{Sources: []*source.Source{src}, FocusedSrc: src, NoColor: true}
	m, _ := NewModel(cfg)

	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.Update(tableNamesLoadedMsg{handle: "@x", names: []string{"actor", "film"}})

	out := m.View()
	require.Contains(t, out, "tables (2)")
}

func TestModel_ExpandTable_DispatchesTableMetaFetch(t *testing.T) {
	src := &source.Source{Handle: "@x"}
	cfg := Config{Sources: []*source.Source{src}, FocusedSrc: src, NoColor: true}
	m, _ := NewModel(cfg)
	m.fetcher = &fakeFetcher{}

	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.Update(tableNamesLoadedMsg{handle: "@x", names: []string{"actor"}})
	// Focus the schema pane.
	m.focused = paneSchema
	// Expand "tables (1)".
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	require.Nil(t, cmd, "expanding the group itself doesn't fetch")

	// Move down to "actor".
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	require.NotNil(t, cmd, "expanding an unloaded table should dispatch a fetch")
	msg := cmd()
	loaded, ok := msg.(tableMetaLoadedMsg)
	require.True(t, ok, "expected tableMetaLoadedMsg, got %T", msg)
	require.Equal(t, "actor", loaded.tableName)
}

func TestModel_DetailFollowsSchemaFocus(t *testing.T) {
	src := &source.Source{Handle: "@x"}
	cfg := Config{Sources: []*source.Source{src}, FocusedSrc: src, NoColor: true}
	m, _ := NewModel(cfg)
	m.fetcher = &fakeFetcher{
		tableNames: map[string][]string{"@x": {"actor"}},
	}

	m.Update(tea.WindowSizeMsg{Width: 150, Height: 30})

	m.Update(tableNamesLoadedMsg{handle: "@x", names: []string{"actor"}})
	m.Update(tableMetaLoadedMsg{
		handle:    "@x",
		tableName: "actor",
		meta: &metadata.Table{
			Name:    "actor",
			Columns: []*metadata.Column{{Name: "actor_id", PrimaryKey: true}},
		},
	})

	m.focused = paneSchema
	// Expand "tables (1)"
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	// Move down to "actor"
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	// Enter to open in detail.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	out := m.View()
	require.Contains(t, out, "actor_id")
	require.Contains(t, out, "PK")
}

func TestModel_InspectorFollowsSchemaCursor(t *testing.T) {
	src := &source.Source{Handle: "@x"}
	cfg := Config{Sources: []*source.Source{src}, FocusedSrc: src, NoColor: true}
	m, _ := NewModel(cfg)
	m.Update(tea.WindowSizeMsg{Width: 150, Height: 30})
	m.Update(tableNamesLoadedMsg{handle: "@x", names: []string{"actor"}})
	m.Update(tableMetaLoadedMsg{
		handle: "@x", tableName: "actor",
		meta: &metadata.Table{
			Name:    "actor",
			Columns: []*metadata.Column{{Name: "actor_id", BaseType: "int", PrimaryKey: true}},
		},
	})

	m.focused = paneSchema
	// Expand "tables (1)", then move the cursor onto "actor". No Enter:
	// the inspector follows the cursor.
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	out := m.View()
	require.Contains(t, out, "actor_id")
	require.Contains(t, out, "PK")

	// Drill down to the column node: the inspector switches to column detail.
	m.Update(tea.KeyMsg{Type: tea.KeySpace})                     // expand "actor"
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // "columns (1)"
	m.Update(tea.KeyMsg{Type: tea.KeySpace})                     // expand "columns (1)"
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // "actor_id"
	out = m.View()
	require.Contains(t, out, "position:")
	require.Contains(t, out, "base type:")
}

func TestModel_SchemaCursor_UnloadedTable_FetchesAndFillsIn(t *testing.T) {
	src := &source.Source{Handle: "@x"}
	cfg := Config{Sources: []*source.Source{src}, FocusedSrc: src, NoColor: true}
	m, _ := NewModel(cfg)
	m.fetcher = &fakeFetcher{tableNames: map[string][]string{"@x": {"actor"}}}
	m.Update(tea.WindowSizeMsg{Width: 150, Height: 30})
	m.Update(tableNamesLoadedMsg{handle: "@x", names: []string{"actor"}})

	m.focused = paneSchema
	// Expand "tables (1)", then move the cursor onto "actor", whose
	// metadata isn't loaded: the move itself must dispatch a fetch.
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	require.NotNil(t, cmd, "moving onto an unloaded table should dispatch a meta fetch")
	require.Equal(t, "actor", m.focusedTbl)

	// When the metadata lands, the inspector fills in.
	m.Update(tableMetaLoadedMsg{
		handle: "@x", tableName: "actor",
		meta: &metadata.Table{
			Name:    "actor",
			Columns: []*metadata.Column{{Name: "actor_id"}},
		},
	})
	require.Contains(t, m.View(), "actor_id")
}

func TestModel_View_Wide_HasInspectorDivider(t *testing.T) {
	m := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	// The no-color theme renders no pane borders, so the only "│" is the
	// inspector divider.
	require.Contains(t, m.View(), "│")
}

func TestModel_View_Stacked_HasInspectorRule(t *testing.T) {
	m := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 30})
	require.Contains(t, m.View(), "─")
}

func TestModel_PressR_TriggersPreviewFetch(t *testing.T) {
	src := &source.Source{Handle: "@x"}
	cfg := Config{Sources: []*source.Source{src}, FocusedSrc: src, NoColor: true, PreviewRows: 50}
	m, _ := NewModel(cfg)

	var (
		mu             sync.Mutex
		previewQueries []string
	)
	m.fetcher = &fakeFetcher{
		tableNames: map[string][]string{"@x": {"actor"}},
	}
	m.previewFn = func(_ context.Context, send func(any), handle, table string, _ int) {
		mu.Lock()
		previewQueries = append(previewQueries, handle+"."+table)
		mu.Unlock()
		send(previewMetaLoadedMsg{handle: handle, tableName: table})
	}
	m.sendFn = func(msg tea.Msg) { m.Update(msg) }

	m.Update(tea.WindowSizeMsg{Width: 150, Height: 30})
	m.Update(tableNamesLoadedMsg{handle: "@x", names: []string{"actor"}})
	m.Update(tableMetaLoadedMsg{
		handle: "@x", tableName: "actor",
		meta: &metadata.Table{Name: "actor", Columns: []*metadata.Column{{Name: "id"}}},
	})

	m.focused = paneSchema
	// Expand "tables (1)"
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	// Move to "actor"
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	// Enter to open the table in detail.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Press 'r' to trigger preview.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	require.NotNil(t, cmd, "pressing r should produce a tea.Cmd")
	// Execute the Cmd to actually invoke previewFn.
	cmd()
	// Allow goroutine to run.
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(previewQueries) == 1
	}, time.Second, 10*time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, "@x.actor", previewQueries[0])
}

func TestModel_Filter_FiltersSourcesPane(t *testing.T) {
	srcs := mkSources("@alpha", "@beta", "@gamma")
	cfg := Config{Sources: srcs, FocusedSrc: srcs[0], NoColor: true}
	m, _ := NewModel(cfg)
	m.Update(tea.WindowSizeMsg{Width: 150, Height: 30})

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

	out := m.View()
	require.Contains(t, out, "@beta")
	require.NotContains(t, out, "@gamma")
}

func TestModel_View_NarrowTerminal_StacksPanes(t *testing.T) {
	m := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 30})
	out := m.View()
	// In stacked mode, the focused source handle still appears.
	require.Contains(t, out, "@test")
}

func TestModel_HelpLineAlwaysVisible(t *testing.T) {
	m := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 30})

	out := m.View()
	// The condensed help line shows several binding hints on every frame.
	for _, want := range []string{"j/k nav", "tab cycle", "/ filter", "q quit"} {
		require.Contains(t, out, want, "help line should always include %q", want)
	}
}

func TestModel_PressR_Capital_TriggersRefresh(t *testing.T) {
	src := &source.Source{Handle: "@x"}
	cfg := Config{Sources: []*source.Source{src}, FocusedSrc: src, NoColor: true}
	m, _ := NewModel(cfg)

	calls := 0
	f := &fakeFetcher{tableNames: map[string][]string{"@x": {"a"}}}
	m.fetcher = &refreshSpy{base: f, calls: &calls}

	m.Update(tea.WindowSizeMsg{Width: 150, Height: 30})
	m.focused = paneSources

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	require.NotNil(t, cmd)
	cmd()
	require.Equal(t, 1, calls)
}

type refreshSpy struct {
	base  metaFetcher
	calls *int
}

func (r *refreshSpy) FetchSourceOverview(ctx context.Context, h string) (*sourceOverview, error) {
	return r.base.FetchSourceOverview(ctx, h)
}

func (r *refreshSpy) FetchTableNames(ctx context.Context, h string) ([]string, error) {
	return r.base.FetchTableNames(ctx, h)
}

func (r *refreshSpy) FetchTableMeta(ctx context.Context, h, t string) (*metadata.Table, error) {
	return r.base.FetchTableMeta(ctx, h, t)
}

func (r *refreshSpy) RefreshSource(ctx context.Context, h string) ([]string, error) {
	*r.calls++
	return r.base.RefreshSource(ctx, h)
}

func TestModel_PressY_CopiesHandle(t *testing.T) {
	srcs := mkSources("@one", "@two")
	cfg := Config{Sources: srcs, FocusedSrc: srcs[1], NoColor: true}
	m, _ := NewModel(cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	var copied string
	m.copyFn = func(s string) error { copied = s; return nil }

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	require.Equal(t, "@two", copied)

	m.focusedTbl = "actor"
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	require.Equal(t, "@two.actor", copied)
}

func TestRun_QuitImmediately(t *testing.T) {
	src := &source.Source{Handle: "@test"}
	cfg := Config{
		Sources:    []*source.Source{src},
		FocusedSrc: src,
		NoColor:    true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := RunWithIO(ctx, nil, cfg, &bytes.Buffer{}, &bytes.Buffer{})
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("RunWithIO did not return after ctx cancel")
	}
}

func TestModel_LeftRightCyclePaneFocus(t *testing.T) {
	m := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	require.Equal(t, paneSources, m.focused)

	// 'l' / right moves focus to the next pane.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	require.Equal(t, paneSchema, m.focused)
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	require.Equal(t, paneDetail, m.focused)

	// 'h' / left moves focus to the previous pane.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	require.Equal(t, paneSchema, m.focused)
	m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	require.Equal(t, paneSources, m.focused)
}

func TestModel_PreviewMsgs_IgnoredForStaleHandle(t *testing.T) {
	m := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	// The active preview is for @test.actor.
	m.preview = &previewBuffer{handle: "@test", tableName: "actor"}

	// A late batch from a different source that happens to share the
	// table name must not be appended to the current preview.
	m.Update(previewRowsAppendedMsg{
		handle: "@other", tableName: "actor",
		rows: []record.Record{{nil}}, done: true,
	})
	require.Empty(t, m.preview.rows, "rows from a different handle must be ignored")
	require.False(t, m.preview.done, "done from a different handle must be ignored")

	// A late error from that stale source must not be applied either.
	m.Update(previewErrMsg{handle: "@other", tableName: "actor", err: errors.New("boom")})
	require.NoError(t, m.preview.err, "error from a different handle must be ignored")
}

func TestModel_EmptyFilteredSources_NoNavPanic(t *testing.T) {
	srcs := mkSources("@a", "@b")
	cfg := Config{Sources: srcs, FocusedSrc: srcs[0], NoColor: true}
	m, _ := NewModel(cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	// Filter the sources pane down to a non-matching string.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // leave filter mode
	require.Nil(t, m.sources.selectedSource(), "no source should match the filter")

	// Navigating and selecting with an empty list must not panic.
	require.NotPanics(t, func() {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	})
	require.Equal(t, srcs[0], m.focusedSrc, "focus should be unchanged when no source is selectable")
}

func TestModel_FetchError_SurfacedInView(t *testing.T) {
	src := &source.Source{Handle: "@x"}
	cfg := Config{Sources: []*source.Source{src}, FocusedSrc: src, NoColor: true}
	m, _ := NewModel(cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	m.Update(tableNamesLoadedMsg{handle: "@x", err: errors.New("connection refused")})
	require.Contains(t, m.View(), "connection refused",
		"a failed metadata fetch should surface its error in the view")

	// A subsequent successful fetch clears the error.
	m.Update(tableNamesLoadedMsg{handle: "@x", names: []string{"actor"}})
	require.NotContains(t, m.View(), "connection refused", "a successful fetch should clear the stale error")
}

func TestModel_Enter_FetchesUnloadedTable(t *testing.T) {
	src := &source.Source{Handle: "@x"}
	cfg := Config{Sources: []*source.Source{src}, FocusedSrc: src, NoColor: true}
	m, _ := NewModel(cfg)
	m.fetcher = &fakeFetcher{tableNames: map[string][]string{"@x": {"actor"}}}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.Update(tableNamesLoadedMsg{handle: "@x", names: []string{"actor"}})

	m.focused = paneSchema
	// Expand the "tables" group (space), then move to "actor".
	m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Enter opens the table; its metadata isn't loaded yet, so it must
	// dispatch a fetch and focus the table.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd, "Enter on an unloaded table should dispatch a meta fetch")
	require.Equal(t, "actor", m.focusedTbl)
}

func TestModel_FetchError_StaleHandleNotShown(t *testing.T) {
	src := &source.Source{Handle: "@x"}
	cfg := Config{Sources: []*source.Source{src}, FocusedSrc: src, NoColor: true}
	m, _ := NewModel(cfg)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	// An error for a source the user has navigated past must not surface.
	m.Update(tableNamesLoadedMsg{handle: "@other", err: errors.New("stale boom")})
	require.NoError(t, m.lastErr)
	require.NotContains(t, m.View(), "stale boom")
}

func TestModel_CopyHandle_SurfacesClipboardError(t *testing.T) {
	src := &source.Source{Handle: "@x"}
	cfg := Config{Sources: []*source.Source{src}, FocusedSrc: src, NoColor: true}
	m, _ := NewModel(cfg)
	m.copyFn = func(string) error { return errors.New("no clipboard utilities available") }
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	require.Error(t, m.lastErr)
	require.Contains(t, m.View(), "copy to clipboard failed")
}
