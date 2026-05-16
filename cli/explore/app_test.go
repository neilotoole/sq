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
