package explore

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
)

func mkSources(handles ...string) []*source.Source {
	out := make([]*source.Source, len(handles))
	for i, h := range handles {
		out[i] = &source.Source{Handle: h}
	}
	return out
}

func TestSourcesPane_InitialSelection(t *testing.T) {
	srcs := mkSources("@a", "@b", "@c")
	p := newSourcesPane(srcs, srcs[1], newTheme(true))
	require.Equal(t, 1, p.selected, "initial selection should be focused source index")
}

func TestSourcesPane_DownUp(t *testing.T) {
	srcs := mkSources("@a", "@b", "@c")
	p := newSourcesPane(srcs, srcs[0], newTheme(true))

	p.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, defaultKeys())
	require.Equal(t, 1, p.selected)

	p.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, defaultKeys())
	require.Equal(t, 2, p.selected)

	p.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, defaultKeys())
	require.Equal(t, 2, p.selected, "j past end clamps")

	p.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, defaultKeys())
	require.Equal(t, 1, p.selected)
}

func TestSourcesPane_View_ListsHandles(t *testing.T) {
	srcs := mkSources("@one", "@two", "@three")
	p := newSourcesPane(srcs, srcs[0], newTheme(true))
	out := p.view(true /* focused */, 40, 20)
	for _, h := range []string{"@one", "@two", "@three"} {
		require.True(t, strings.Contains(out, h), "view should contain %q; got: %s", h, out)
	}
}
