package explore

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/neilotoole/sq/libsq/source/metadata"
)

// schemaNodeKind identifies a node's role in the tree. It governs
// rendering, expansion, and what "open" means in the detail pane.
type schemaNodeKind int

const (
	nodeLoading schemaNodeKind = iota
	nodeGroupTables
	nodeTable
	nodeGroupColumns
	nodeColumn
	nodeGroupIndexes
	nodeIndex
	nodeGroupFK
	nodeFK
	nodeGroupUniques
	nodeUnique
)

// schemaNode is one row in the rendered tree. depth governs indentation,
// expanded governs whether children are visible. children is computed
// lazily — populated when toggleExpand fires for a kind that needs to
// fetch (nodeTable) or built locally (nodeGroupColumns).
type schemaNode struct {
	// table metadata back-reference for nodeTable. Set by setTableMeta.
	tableMeta *metadata.Table

	// Identity for messages and detail-pane focus.
	tableName string // set on nodeTable and descendants.

	label string // rendered text, without depth indent.

	// Children, populated on expand. nil means "not yet expanded".
	children []*schemaNode

	kind  schemaNodeKind
	depth int

	expanded bool

	// loading is true between an expand-triggered fetch and its msg.
	loading bool
}

// schemaTree is the middle pane's model.
type schemaTree struct {
	// root is the synthetic top-level container; its children are the
	// visible top-level groups. We render starting at root.children.
	root *schemaNode

	handle string
	theme  theme

	// filter is the active case-insensitive substring filter; "" = none.
	filter string

	// selected is the visible-row index of the cursor.
	selected int

	// loaded is true once setTableNames has been called.
	loaded bool
}

// setFilter narrows visible nodes to those whose label (or any
// descendant label) contains f (case-insensitive). Empty f clears.
func (tr *schemaTree) setFilter(f string) {
	tr.filter = strings.ToLower(f)
}

func newSchemaTree(handle string, th theme) *schemaTree {
	loading := &schemaNode{kind: nodeLoading, label: "loading…"}
	return &schemaTree{
		handle: handle,
		theme:  th,
		root:   &schemaNode{children: []*schemaNode{loading}},
	}
}

// setTableNames replaces the tree contents with a single collapsed
// "tables (N)" group containing one stub per table. Individual
// table metadata is fetched on expand.
func (tr *schemaTree) setTableNames(names []string) {
	group := &schemaNode{
		kind:  nodeGroupTables,
		label: fmt.Sprintf("tables (%d)", len(names)),
		depth: 0,
	}
	for _, n := range names {
		group.children = append(group.children, &schemaNode{
			kind:      nodeTable,
			label:     n,
			depth:     1,
			tableName: n,
		})
	}
	tr.root = &schemaNode{children: []*schemaNode{group}}
	tr.selected = 0
	tr.loaded = true
}

// setTableMeta attaches the full table metadata to the matching
// nodeTable. Children for columns / indexes / FK / unique constraints
// are NOT auto-built until that node is expanded; setTableMeta only
// caches the meta so the next toggleExpand has data to build from.
func (tr *schemaTree) setTableMeta(tableName string, md *metadata.Table) {
	tr.walk(func(n *schemaNode) bool {
		if n.kind == nodeTable && n.tableName == tableName {
			n.tableMeta = md
			n.loading = false
			// If currently expanded, rebuild children with new meta.
			if n.expanded {
				n.children = buildTableChildren(md, n.depth+1)
			}
			return false
		}
		return true
	})
}

// buildTableChildren builds the per-table child nodes from metadata.
// Empty groups are omitted.
func buildTableChildren(md *metadata.Table, depth int) []*schemaNode {
	if md == nil {
		return nil
	}
	out := []*schemaNode{}
	if len(md.Columns) > 0 {
		g := &schemaNode{
			kind:      nodeGroupColumns,
			label:     fmt.Sprintf("columns (%d)", len(md.Columns)),
			depth:     depth,
			tableName: md.Name,
		}
		for _, c := range md.Columns {
			g.children = append(g.children, &schemaNode{
				kind:      nodeColumn,
				label:     c.Name,
				depth:     depth + 1,
				tableName: md.Name,
			})
		}
		out = append(out, g)
	}
	if len(md.Indexes) > 0 {
		g := &schemaNode{
			kind:      nodeGroupIndexes,
			label:     fmt.Sprintf("indexes (%d)", len(md.Indexes)),
			depth:     depth,
			tableName: md.Name,
		}
		for _, i := range md.Indexes {
			g.children = append(g.children, &schemaNode{
				kind:      nodeIndex,
				label:     i.Name,
				depth:     depth + 1,
				tableName: md.Name,
			})
		}
		out = append(out, g)
	}
	if md.FK != nil && (len(md.FK.Outgoing) > 0 || len(md.FK.Incoming) > 0) {
		label := fmt.Sprintf("fk (%d↗ %d↙)", len(md.FK.Outgoing), len(md.FK.Incoming))
		g := &schemaNode{kind: nodeGroupFK, label: label, depth: depth, tableName: md.Name}
		for _, fk := range md.FK.Outgoing {
			g.children = append(g.children, &schemaNode{
				kind: nodeFK, label: "↗ " + fk.RefTable, depth: depth + 1, tableName: md.Name,
			})
		}
		for _, fk := range md.FK.Incoming {
			g.children = append(g.children, &schemaNode{
				kind: nodeFK, label: "↙ " + fk.Table, depth: depth + 1, tableName: md.Name,
			})
		}
		out = append(out, g)
	}
	if len(md.UniqueConstraints) > 0 {
		g := &schemaNode{
			kind:      nodeGroupUniques,
			label:     fmt.Sprintf("unique (%d)", len(md.UniqueConstraints)),
			depth:     depth,
			tableName: md.Name,
		}
		for _, uc := range md.UniqueConstraints {
			g.children = append(g.children, &schemaNode{
				kind:      nodeUnique,
				label:     uc.Name,
				depth:     depth + 1,
				tableName: md.Name,
			})
		}
		out = append(out, g)
	}
	return out
}

// walk visits every node in tree pre-order. The visit fn returns false
// to stop traversal.
func (tr *schemaTree) walk(fn func(*schemaNode) bool) {
	if tr.root == nil {
		return
	}
	var rec func(n *schemaNode) bool
	rec = func(n *schemaNode) bool {
		if !fn(n) {
			return false
		}
		for _, c := range n.children {
			if !rec(c) {
				return false
			}
		}
		return true
	}
	for _, c := range tr.root.children {
		if !rec(c) {
			return
		}
	}
}

// visibleNodes flattens the tree to the rows the user can see, in
// rendering order. A child is included iff every ancestor (excluding
// the synthetic root) is expanded. When a filter is active, nodes
// (and their ancestors) that don't match are pruned.
func (tr *schemaTree) visibleNodes() []*schemaNode {
	var out []*schemaNode
	var rec func(n *schemaNode)
	rec = func(n *schemaNode) {
		if tr.filter != "" && !tr.nodeMatches(n) {
			return
		}
		out = append(out, n)
		if n.expanded {
			for _, c := range n.children {
				rec(c)
			}
		}
	}
	for _, c := range tr.root.children {
		rec(c)
	}
	return out
}

// nodeMatches returns true if n (or any descendant) matches the
// current filter. Group rows match if they have any matching child.
func (tr *schemaTree) nodeMatches(n *schemaNode) bool {
	if tr.filter == "" {
		return true
	}
	if strings.Contains(strings.ToLower(n.label), tr.filter) {
		return true
	}
	for _, c := range n.children {
		if tr.nodeMatches(c) {
			return true
		}
	}
	return false
}

func (tr *schemaTree) visibleCount() int { return len(tr.visibleNodes()) }

// selectedNode returns the visible node at tr.selected.
func (tr *schemaTree) selectedNode() *schemaNode {
	vs := tr.visibleNodes()
	if len(vs) == 0 {
		return nil
	}
	if tr.selected >= len(vs) {
		tr.selected = len(vs) - 1
	}
	return vs[tr.selected]
}

// selectedTableName returns the nearest enclosing table's name, or "".
func (tr *schemaTree) selectedTableName() string {
	n := tr.selectedNode()
	if n == nil {
		return ""
	}
	return n.tableName
}

// move adjusts the cursor by delta (positive = down). Clamps to range.
func (tr *schemaTree) move(delta int) {
	maxIdx := tr.visibleCount() - 1
	if maxIdx < 0 {
		maxIdx = 0
	}
	tr.selected += delta
	if tr.selected < 0 {
		tr.selected = 0
	}
	if tr.selected > maxIdx {
		tr.selected = maxIdx
	}
}

// toggleExpand flips the expanded flag on the node at the given visible
// index. For nodeTable, it returns true if a fetch should be dispatched
// (the table's children haven't been built yet because we don't have
// its metadata). The caller (schemaPane.update) uses this signal to
// emit a fetchTableMetaCmd.
func (tr *schemaTree) toggleExpand(idx int) (needsFetch bool, tableName string) {
	vs := tr.visibleNodes()
	if idx < 0 || idx >= len(vs) {
		return false, ""
	}
	n := vs[idx]
	n.expanded = !n.expanded
	if !n.expanded {
		return false, ""
	}
	// Just expanded. Decide if a fetch is needed.
	if n.kind == nodeTable && n.tableMeta == nil {
		n.loading = true
		// Build a single "loading…" child until meta arrives.
		n.children = []*schemaNode{{kind: nodeLoading, label: "loading…", depth: n.depth + 1}}
		return true, n.tableName
	}
	if n.kind == nodeTable && n.tableMeta != nil && len(n.children) == 0 {
		n.children = buildTableChildren(n.tableMeta, n.depth+1)
	}
	return false, ""
}

// view renders the tree at given width/height with the cursor styled
// if focused. Long lists scroll so the selected row stays in view.
func (tr *schemaTree) view(focused bool, width, height int) string {
	var b strings.Builder
	title := tr.theme.Title.Render("Schema (" + tr.handle + ")")
	b.WriteString(title)
	b.WriteString("\n")

	vs := tr.visibleNodes()
	avail := height - 3
	if avail < 1 {
		avail = 1
	}
	start, end := scrollWindow(tr.selected, len(vs), avail)
	for i := start; i < end; i++ {
		n := vs[i]
		line := strings.Repeat("  ", n.depth)
		switch {
		case n.kind == nodeLoading:
			line += "⏳ " + n.label
		case len(n.children) > 0 || n.kind == nodeTable:
			if n.expanded {
				line += "▾ " + n.label
			} else {
				line += "▸ " + n.label
			}
		default:
			line += "  " + n.label
		}
		if i == tr.selected && focused {
			line = tr.theme.ItemSel.Render(line)
		}
		b.WriteString(line)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	style := tr.theme.Pane
	if focused {
		style = tr.theme.PaneFocus
	}
	return style.Width(width).Height(height).MaxHeight(height).Render(b.String())
}

// selectedDetail returns the "open this in detail" target — either a
// *metadata.Table, *metadata.Column, or nil — for the current
// selection. Useful when the user presses Enter.
func (tr *schemaTree) selectedDetail() (tbl *metadata.Table, col *metadata.Column) {
	n := tr.selectedNode()
	if n == nil {
		return nil, nil
	}
	switch n.kind { //nolint:exhaustive // other kinds have no detail target.
	case nodeTable:
		return n.tableMeta, nil
	case nodeColumn:
		tableMeta := tr.findTableMeta(n.tableName)
		if tableMeta != nil {
			for _, c := range tableMeta.Columns {
				if c.Name == n.label {
					return nil, c
				}
			}
		}
	case nodeGroupColumns, nodeGroupIndexes, nodeGroupFK, nodeGroupUniques:
		return tr.findTableMeta(n.tableName), nil
	}
	return nil, nil
}

// findTableMeta returns the cached metadata for the named table, or nil.
func (tr *schemaTree) findTableMeta(tableName string) *metadata.Table {
	var out *metadata.Table
	tr.walk(func(n *schemaNode) bool {
		if n.kind == nodeTable && n.tableName == tableName {
			out = n.tableMeta
			return false
		}
		return true
	})
	return out
}

// update routes a key message scoped to the tree. Returns (needsFetch,
// tableName) when the action triggers a fetch (e.g. expanding an
// unloaded table); the caller dispatches the corresponding tea.Cmd.
func (tr *schemaTree) update(msg tea.KeyMsg, keys keyMap) (needsFetch bool, tableName string) {
	switch {
	case key.Matches(msg, keys.Down):
		tr.move(1)
	case key.Matches(msg, keys.Up):
		tr.move(-1)
	case key.Matches(msg, keys.Top):
		tr.selected = 0
	case key.Matches(msg, keys.Bottom):
		tr.selected = tr.visibleCount() - 1
		if tr.selected < 0 {
			tr.selected = 0
		}
	case key.Matches(msg, keys.Space), key.Matches(msg, keys.Enter), key.Matches(msg, keys.Right):
		needsFetch, tableName = tr.toggleExpand(tr.selected)
	}
	return needsFetch, tableName
}
