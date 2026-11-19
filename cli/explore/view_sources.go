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

// view renders the pane at the given width/height, scrolling the list
// so the selected row stays in view.
func (p *sourcesPane) view(focused bool, width, height int) string {
	var b strings.Builder
	titleStyle := p.theme.Title
	if focused {
		titleStyle = p.theme.TitleFocus
	}
	b.WriteString(titleStyle.Render("Sources"))
	if p.filter != "" {
		fmt.Fprintf(&b, " (/%s)", p.filter)
	}
	b.WriteString("\n")

	vs := p.visibleSources()
	// Available content rows = height - 1 (title row).
	avail := height - 1
	if avail < 1 {
		avail = 1
	}
	start, end := scrollWindow(p.selected, len(vs), avail)
	for i := start; i < end; i++ {
		s := vs[i]
		// Exactly one style renders each row: nesting Render calls embeds
		// ANSI codes inside the outer style's text, which lipgloss mangles
		// into literal "[1;32m" garbage (notably via ItemCursor's
		// per-rune underline handling). Precedence: selection highlight
		// beats the Handle/Active coloring.
		st := p.theme.Handle
		if s == p.active {
			st = p.theme.ItemActiv
		}
		if i == p.selected {
			if focused {
				st = p.theme.ItemSel
			} else {
				st = p.theme.ItemCursor
			}
		}
		b.WriteString(st.Render(s.Handle))
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return p.theme.Pane.Width(width).Height(height).MaxHeight(height).Render(b.String())
}

// scrollWindow computes [start, end) item indices to render given the
// selected cursor, total item count, and the number of rows we can
// show. The selected row is kept in the visible window; at boundaries
// the window butts against the start or end of the list.
func scrollWindow(selected, total, view int) (start, end int) {
	if view >= total {
		return 0, total
	}
	start = selected - view/2
	if start < 0 {
		start = 0
	}
	end = start + view
	if end > total {
		end = total
		start = end - view
		if start < 0 {
			start = 0
		}
	}
	return start, end
}
