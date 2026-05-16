package explore

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/neilotoole/sq/libsq/source"
)

// sourcesPane is the leftmost pane: a vim-navigable list of sources.
type sourcesPane struct {
	theme    theme
	active   *source.Source // optionally bolded (e.g. the active source).
	sources  []*source.Source
	selected int
}

func newSourcesPane(sources []*source.Source, focused *source.Source, th theme) *sourcesPane {
	p := &sourcesPane{sources: sources, theme: th, active: focused}
	for i, s := range sources {
		if s == focused {
			p.selected = i
			break
		}
	}
	return p
}

// selectedSource returns the currently highlighted source, or nil if
// the list is empty.
//
//nolint:unused // mounted in Phase 3.3.
func (p *sourcesPane) selectedSource() *source.Source {
	if len(p.sources) == 0 {
		return nil
	}
	return p.sources[p.selected]
}

// update handles a key message scoped to this pane.
//
//nolint:unparam // tea.Cmd return is required by the pane contract; future phases will emit cmds.
func (p *sourcesPane) update(msg tea.KeyMsg, keys keyMap) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Down):
		if p.selected < len(p.sources)-1 {
			p.selected++
		}
	case key.Matches(msg, keys.Up):
		if p.selected > 0 {
			p.selected--
		}
	case key.Matches(msg, keys.Top):
		p.selected = 0
	case key.Matches(msg, keys.Bottom):
		if len(p.sources) > 0 {
			p.selected = len(p.sources) - 1
		}
	}
	return nil
}

// view renders the pane at the given width/height.
func (p *sourcesPane) view(focused bool, width, height int) string {
	var b strings.Builder
	title := p.theme.Title.Render("Sources")
	b.WriteString(title)
	b.WriteString("\n")
	for i, s := range p.sources {
		line := s.Handle
		if s == p.active {
			line = p.theme.ItemActiv.Render(line)
		}
		if i == p.selected && focused {
			line = p.theme.ItemSel.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	style := p.theme.Pane
	if focused {
		style = p.theme.PaneFocus
	}
	return style.Width(width).Height(height).Render(b.String())
}
