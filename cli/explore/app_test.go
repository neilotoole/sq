package explore

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"

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
	m := newTestModel(t)
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 30))
	defer tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	require.Equal(t, paneSources, m.focused)

	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	require.Eventually(t, func() bool { return m.focused == paneSchema }, time.Second, 10*time.Millisecond)

	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	require.Eventually(t, func() bool { return m.focused == paneDetail }, time.Second, 10*time.Millisecond)

	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	require.Eventually(t, func() bool { return m.focused == paneSources }, time.Second, 10*time.Millisecond)
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
