// Package commonw contains miscellaneous common output writer functionality.
package commonw

import (
	"cmp"
	"reflect"
	"slices"
	"strings"

	"github.com/neilotoole/sq/libsq/core/options"
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

// ColumnKey returns the combined "PK,FK,UK" marker for a column, or "" when
// the column participates in no key.
func ColumnKey(col *metadata.Column, fkCols, ucCols map[string]bool) string {
	var parts []string
	if col.PrimaryKey {
		parts = append(parts, "PK")
	}
	if fkCols[col.Name] {
		parts = append(parts, "FK")
	}
	if ucCols[col.Name] {
		parts = append(parts, "UK")
	}
	return strings.Join(parts, ",")
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

// UCColumnSet returns the set of column names on tbl that participate in any
// unique constraint.
func UCColumnSet(tbl *metadata.Table) map[string]bool {
	if len(tbl.UniqueConstraints) == 0 {
		return nil
	}
	set := make(map[string]bool)
	for _, uc := range tbl.UniqueConstraints {
		if uc == nil {
			continue
		}
		for _, c := range uc.Columns {
			set[c] = true
		}
	}
	return set
}

// FKRow is a single foreign-key relationship flattened for tabular
// rendering, in either direction relative to the owning table. The fields
// are plain (unwrapped) strings; callers apply any format-specific
// code-wrapping or escaping. From and To always read referencing → referenced;
// Direction identifies which side is the owning table.
type FKRow struct {
	// Direction is "outgoing" (a constraint declared on the owning table)
	// or "incoming" (a constraint on another table that references the
	// owning table).
	Direction string

	// From is the referencing side, formatted "table(col, ...)".
	From string

	// To is the referenced side, formatted "[catalog.][schema.]table(col, ...)".
	To string

	// Constraint is the constraint name, or "" when the source doesn't
	// expose one (e.g. some SQLite tables).
	Constraint string

	// OnUpdate and OnDelete are the lower-cased referential actions (e.g.
	// "cascade", "no action"), or "" when not reported by the source.
	OnUpdate string
	OnDelete string
}

// FKRows flattens tbl's outgoing and incoming foreign keys into rows for
// tabular rendering. Outgoing rows (sorted) precede incoming rows (sorted).
// Returns nil when tbl has no foreign keys in either direction.
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
		if fk != nil {
			rows = append(rows, newFKRow("outgoing", fk))
		}
	}
	for _, fk := range incoming {
		if fk != nil {
			rows = append(rows, newFKRow("incoming", fk))
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return rows
}

func newFKRow(direction string, fk *metadata.ForeignKey) FKRow {
	return FKRow{
		Direction:  direction,
		From:       fk.Table + "(" + strings.Join(fk.Columns, ", ") + ")",
		To:         fkRef(fk),
		Constraint: fk.Name,
		OnUpdate:   strings.ToLower(fk.OnUpdate),
		OnDelete:   strings.ToLower(fk.OnDelete),
	}
}

// fkRef returns "[catalog.][schema.]ref_table(ref_col, ...)" for fk,
// qualified with the referenced schema/catalog when the reference points
// outside this source. Same-source references stay unqualified because
// [metadata.LinkForeignKeys] clears RefCatalog / RefSchema when they match
// the owning source.
func fkRef(fk *metadata.ForeignKey) string {
	target := fk.RefTable
	if fk.RefSchema != "" {
		target = fk.RefSchema + "." + target
	}
	if fk.RefCatalog != "" {
		target = fk.RefCatalog + "." + target
	}
	return target + "(" + strings.Join(fk.RefColumns, ", ") + ")"
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
