package explore

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
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

func TestSourcesPane_Filter(t *testing.T) {
	srcs := mkSources("@alpha", "@beta", "@gamma", "@delta")
	p := newSourcesPane(srcs, srcs[0], newTheme(true))

	p.setFilter("a")
	visible := p.visibleSources()
	require.Len(t, visible, 4) // all 4 handles contain 'a'

	p.setFilter("be")
	visible = p.visibleSources()
	require.Len(t, visible, 1)
	require.Equal(t, "@beta", visible[0].Handle)

	p.setFilter("")
	visible = p.visibleSources()
	require.Len(t, visible, 4)
}

func TestSourcesPane_View_NoNestedStyleArtifacts(t *testing.T) {
	// Regression: the active source under the cursor in an unfocused
	// pane was styled twice (ItemActiv, then ItemCursor over the
	// already-escaped string); ItemCursor's underline styles per-rune,
	// which mangled the inner ANSI codes into literal "[1;32m" text.
	//
	// Tests run without a TTY, so termenv would pick the no-op Ascii
	// profile and hide the bug; force ANSI so styles emit codes.
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	t.Cleanup(func() { lipgloss.SetColorProfile(oldProfile) })

	srcs := mkSources("@a", "@duck")
	p := newSourcesPane(srcs, srcs[1], newTheme(false)) // color theme; @duck is active + selected.
	out := p.view(false /* unfocused: cursor row uses ItemCursor */, 40, 20)

	// Strip well-formed SGR escape sequences; no bracket-code residue
	// may remain.
	stripped := ansiRe.ReplaceAllString(out, "")
	require.NotContains(t, stripped, "[1;32m", "nested styling leaked literal ANSI codes: %q", out)
	require.NotContains(t, stripped, "[0m", "nested styling leaked literal ANSI codes: %q", out)
	require.Contains(t, stripped, "@duck")
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestSourcesPane_View_ListsHandles(t *testing.T) {
	srcs := mkSources("@one", "@two", "@three")
	p := newSourcesPane(srcs, srcs[0], newTheme(true))
	out := p.view(true /* focused */, 40, 20)
	for _, h := range []string{"@one", "@two", "@three"} {
		require.True(t, strings.Contains(out, h), "view should contain %q; got: %s", h, out)
	}
}
