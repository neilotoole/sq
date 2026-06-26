// Package commonw contains miscellaneous common output writer functionality.
package commonw

import (
	"cmp"
	"reflect"
	"slices"
	"strings"

	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// VerboseOpt is a verbose realization of an options.Opt value.
// This is used primarily to print metadata about the opt.
type VerboseOpt struct { //nolint:govet // field alignment
	Key          string `json:"key"`
	Usage        string `json:"usage"`
	Type         string `json:"type"`
	IsSet        bool   `json:"is_set"`
	DefaultValue any    `json:"default_value"`
	Value        any    `json:"value"`
	// FIXME: Append Flag?
	Help string `json:"help"`
}

// NewVerboseOpt returns a VerboseOpt built from opt and o.
func NewVerboseOpt(opt options.Opt, o options.Options) VerboseOpt {
	v := VerboseOpt{
		Key:          opt.Key(),
		Usage:        opt.Usage(),
		DefaultValue: opt.GetAny(nil),
		IsSet:        o.IsSet(opt),
		Help:         opt.Help(),
		Value:        opt.GetAny(o),
		Type:         reflect.TypeOf(opt.GetAny(nil)).String(),
	}

	return v
}

// FKColumnSet returns the set of column names on tbl that participate in any
// outgoing foreign key.
func FKColumnSet(tbl *metadata.Table) map[string]bool {
	if tbl.FK == nil {
		return nil
	}
	set := make(map[string]bool)
	for _, fk := range tbl.FK.Outgoing {
		if fk == nil {
			continue
		}
		for _, c := range fk.Columns {
			set[c] = true
		}
	}
	return set
}

// IsView reports whether tbl is a view (rather than a table). Table.TableType
// is the driver-independent canonical type; both "view" and "materialized_view"
// are considered views.
func IsView(tbl *metadata.Table) bool {
	return tbl != nil &&
		(tbl.TableType == sqlz.TableTypeView || tbl.TableType == sqlz.TableTypeMaterializedView)
}

// HasViews reports whether tables contains any view.
func HasViews(tables []*metadata.Table) bool {
	return slices.ContainsFunc(tables, IsView)
}

// FKRow is a single foreign-key relationship flattened for tabular rendering,
// anchored on the owning ("this") table: Local is this table's side and Remote
// the other table's side, with Direction indicating which way the reference
// points. Local and Remote are plain (unwrapped) strings; callers apply any
// format-specific code-wrapping or escaping.
type FKRow struct {
	// Direction is "outgoing" (a foreign key declared on this table,
	// referencing another) or "incoming" (a foreign key on another table
	// that references this one).
	Direction string

	// Local is this table's column(s), bare — the owning table is implied by
	// the surrounding section, so it's "language_id" for a single column and
	// "(col1, col2)" for a composite key.
	Local string

	// Remote is the other table's side: the referenced
	// "[catalog.][schema.]table(col, ...)" for outgoing rows, or the
	// referencing "table(col, ...)" for incoming rows.
	Remote string

	// Constraint is the constraint name, or "" when the source doesn't
	// expose one (e.g. some SQLite tables).
	Constraint string

	// OnUpdate and OnDelete are the lower-cased referential actions (e.g.
	// "cascade"), or "" when not reported or for the SQL-default "NO ACTION".
	OnUpdate string
	OnDelete string
}

// FKRows flattens tbl's outgoing and incoming foreign keys into rows for
// tabular rendering, anchored on tbl. Outgoing rows (sorted) precede incoming
// rows (sorted). Returns nil when tbl has no foreign keys in either direction.
func FKRows(tbl *metadata.Table) []FKRow {
	if tbl == nil || tbl.FK == nil {
		return nil
	}

	outgoing := slices.Clone(tbl.FK.Outgoing)
	slices.SortFunc(outgoing, compareFK)
	incoming := slices.Clone(tbl.FK.Incoming)
	slices.SortFunc(incoming, compareFK)

	rows := make([]FKRow, 0, len(outgoing)+len(incoming))
	for _, fk := range outgoing {
		if fk == nil {
			continue
		}
		rows = append(rows, FKRow{
			Direction:  "outgoing",
			Local:      bareCols(fk.Columns),
			Remote:     qualify(refPrefix(fk), fk.RefColumns),
			Constraint: fk.Name,
			OnUpdate:   fkAction(fk.OnUpdate),
			OnDelete:   fkAction(fk.OnDelete),
		})
	}
	for _, fk := range incoming {
		if fk == nil {
			continue
		}
		rows = append(rows, FKRow{
			Direction:  "incoming",
			Local:      bareCols(fk.RefColumns),
			Remote:     qualify(fk.Table, fk.Columns),
			Constraint: fk.Name,
			OnUpdate:   fkAction(fk.OnUpdate),
			OnDelete:   fkAction(fk.OnDelete),
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return rows
}

// qualify renders the remote (other table) side of an FK relationship,
// qualified by a table prefix: "t.col" for a single column (dot notation),
// "t(col1, col2)" for a composite key (the paren form keeps the columns
// unambiguous).
func qualify(prefix string, cols []string) string {
	if len(cols) == 1 {
		return prefix + "." + cols[0]
	}
	return prefix + "(" + strings.Join(cols, ", ") + ")"
}

// bareCols renders the local (this table) side of an FK relationship as the
// column(s) only, without a table prefix — the owning table is already implied
// by the surrounding section. A single column is returned as-is; a composite
// key is parenthesized: "(col1, col2)".
func bareCols(cols []string) string {
	if len(cols) == 1 {
		return cols[0]
	}
	return "(" + strings.Join(cols, ", ") + ")"
}

// fkAction lower-cases a referential action, returning "" for the SQL
// default "NO ACTION" — which is the implied behavior and just noise in
// output — so the On update / On delete cells stay blank for it.
func fkAction(s string) string {
	s = strings.ToLower(s)
	if s == "no action" {
		return ""
	}
	return s
}

// refPrefix returns the referenced table name for fk, qualified with the
// referenced schema/catalog when the reference points outside this source.
// Same-source references stay unqualified because [metadata.LinkForeignKeys]
// clears RefCatalog / RefSchema when they match the owning source.
func refPrefix(fk *metadata.ForeignKey) string {
	target := fk.RefTable
	if fk.RefSchema != "" {
		target = fk.RefSchema + "." + target
	}
	if fk.RefCatalog != "" {
		target = fk.RefCatalog + "." + target
	}
	return target
}

func compareFK(a, b *metadata.ForeignKey) int {
	if c := cmp.Compare(a.Name, b.Name); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Table, b.Table); c != 0 {
		return c
	}
	if c := cmp.Compare(a.RefTable, b.RefTable); c != 0 {
		return c
	}
	return cmp.Compare(strings.Join(a.Columns, ","), strings.Join(b.Columns, ","))
}

// IndexRow is a single index flattened for tabular rendering. Name and
// Columns are plain (unwrapped) strings; callers apply any format-specific
// code-wrapping or escaping.
type IndexRow struct { //nolint:govet // field order mirrors the rendered columns
	// Name is the index name as reported by the source (may be
	// engine-generated, e.g. "sqlite_autoindex_film_actor_1").
	Name string

	// Columns are the indexed columns joined as "col, col".
	Columns string

	// Unique reports whether the index enforces uniqueness.
	Unique bool

	// Primary reports whether the index backs the primary key (a subset
	// of Unique).
	Primary bool

	// Type is the lower-cased access-method name (e.g. "btree", "gist"),
	// or "" when the source doesn't report it (e.g. SQLite).
	Type string
}

// IndexRows flattens tbl's indexes into rows for tabular rendering, sorted
// by index name. Returns nil when tbl has no indexes.
func IndexRows(tbl *metadata.Table) []IndexRow {
	if tbl == nil || len(tbl.Indexes) == 0 {
		return nil
	}

	idxs := slices.Clone(tbl.Indexes)
	slices.SortFunc(idxs, func(a, b *metadata.Index) int {
		return cmp.Compare(a.Name, b.Name)
	})

	rows := make([]IndexRow, 0, len(idxs))
	for _, idx := range idxs {
		if idx == nil {
			continue
		}
		// Expression/functional key positions are empty-string sentinels
		// in idx.Columns (see [metadata.Index.Columns]); omit them from the
		// joined display string. JSON/YAML output carries the full arity.
		cols := make([]string, 0, len(idx.Columns))
		for _, c := range idx.Columns {
			if c != "" {
				cols = append(cols, c)
			}
		}
		rows = append(rows, IndexRow{
			Name:    idx.Name,
			Columns: strings.Join(cols, ", "),
			Unique:  idx.Unique,
			Primary: idx.Primary,
			Type:    strings.ToLower(idx.Type),
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return rows
}

// UCRow is a single unique constraint flattened for tabular rendering. Name
// and Columns are plain (unwrapped) strings; callers apply any
// format-specific code-wrapping or escaping.
type UCRow struct {
	// Name is the constraint name, or "" when the source doesn't expose
	// one (some SQLite cases).
	Name string

	// Columns are the constrained columns joined as "col, col".
	Columns string
}

// UCRows flattens tbl's unique constraints into rows for tabular rendering,
// sorted by name then columns. Returns nil when tbl has none.
func UCRows(tbl *metadata.Table) []UCRow {
	if tbl == nil || len(tbl.UniqueConstraints) == 0 {
		return nil
	}

	ucs := slices.Clone(tbl.UniqueConstraints)
	slices.SortFunc(ucs, func(a, b *metadata.UniqueConstraint) int {
		if c := cmp.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return cmp.Compare(strings.Join(a.Columns, ","), strings.Join(b.Columns, ","))
	})

	rows := make([]UCRow, 0, len(ucs))
	for _, uc := range ucs {
		if uc == nil {
			continue
		}
		rows = append(rows, UCRow{Name: uc.Name, Columns: strings.Join(uc.Columns, ", ")})
	}
	if len(rows) == 0 {
		return nil
	}
	return rows
}
