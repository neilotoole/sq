package explore

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/neilotoole/sq/libsq/source"
)

// sourcesPane is the leftmost pane: a vim-navigable list of sources.
type sourcesPane struct { //nolint:govet // field grouping favors readability over alignment.
	theme    theme
	active   *source.Source // optionally bolded (e.g. the active source).
	sources  []*source.Source
	filter   string
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

// setFilter narrows the displayed sources to handles containing f
// (case-insensitive). An empty f clears the filter.
func (p *sourcesPane) setFilter(f string) {
	p.filter = strings.ToLower(f)
	// Clamp selection into the visible range.
	if vs := p.visibleSources(); len(vs) > 0 {
		if p.selected >= len(vs) {
			p.selected = len(vs) - 1
		}
	} else {
		p.selected = 0
	}
}

// visibleSources returns the sources currently visible under the
// active filter (or every source if no filter is set).
func (p *sourcesPane) visibleSources() []*source.Source {
	if p.filter == "" {
		return p.sources
	}
	out := make([]*source.Source, 0, len(p.sources))
	for _, s := range p.sources {
		if strings.Contains(strings.ToLower(s.Handle), p.filter) {
			out = append(out, s)
		}
	}
	return out
}

// selectedSource returns the currently highlighted source, or nil if
// the list is empty.
func (p *sourcesPane) selectedSource() *source.Source {
	vs := p.visibleSources()
	if len(vs) == 0 {
		return nil
	}
	if p.selected >= len(vs) {
		p.selected = len(vs) - 1
	}
	return vs[p.selected]
}

// update handles a key message scoped to this pane.
func (p *sourcesPane) update(msg tea.KeyMsg, keys keyMap) {
	vs := p.visibleSources()
	switch {
	case key.Matches(msg, keys.Down):
		if p.selected < len(vs)-1 {
			p.selected++
		}
	case key.Matches(msg, keys.Up):
		if p.selected > 0 {
			p.selected--
		}
	case key.Matches(msg, keys.Top):
		p.selected = 0
	case key.Matches(msg, keys.Bottom):
		if len(vs) > 0 {
			p.selected = len(vs) - 1
		}
	}
}

// view renders the pane at the given width/height.
func (p *sourcesPane) view(focused bool, width, height int) string {
	var b strings.Builder
	title := p.theme.Title.Render("Sources")
	b.WriteString(title)
	if p.filter != "" {
		fmt.Fprintf(&b, " (/%s)", p.filter)
	}
	b.WriteString("\n")
	vs := p.visibleSources()
	for i, s := range vs {
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
