// Package erdmodel holds the renderer-neutral parts of sq's
// entity-relationship model shared by the diagram renderers (the Mermaid
// source generator in package mermaid and the Graphviz DOT generator in
// package erddot). In particular it owns the foreign-key cardinality
// inference, so every ERD renderer reports the same relationships for a
// given schema rather than each re-deriving (and potentially disagreeing
// on) cardinality.
package erdmodel

import (
	"slices"

	"github.com/neilotoole/sq/libsq/source/metadata"
)

// Cardinality describes a foreign-key relationship's multiplicity in a
// renderer-neutral way. Each renderer maps these booleans to its own
// notation (Mermaid's "||--o{" tokens, Graphviz crow's-foot arrowheads).
type Cardinality struct {
	// ParentOptional reports that the parent (referenced, PK) side is
	// zero-or-one rather than exactly one: some referencing FK column is
	// nullable.
	ParentOptional bool

	// ChildUnique reports that the child (referencing, FK) side is
	// one-to-one rather than zero-or-many: the FK columns are themselves
	// unique on the child (its primary key or a unique constraint).
	ChildUnique bool
}

// Edge is a resolved foreign-key relationship between two tables in the same
// source, renderer-neutral. Parent is the referenced (PK) side, Child the
// referencing (FK) side. Label is the constraint name (may be empty);
// renderers are responsible for any quoting or escaping.
type Edge struct {
	Parent string
	Child  string
	Label  string
	Card   Cardinality
}

// Resolve builds the Edge for a single foreign key. It returns false for a
// nil fk or a reference that points outside this source (non-empty
// RefCatalog/RefSchema after [metadata.LinkForeignKeys] normalization),
// which renderers skip.
//
// Cardinality is inferred from the child (referencing) table when it's
// available in byName: ParentOptional when any FK column is nullable;
// ChildUnique when the FK columns are themselves unique on the child. When
// the child table isn't available, both default to false (exactly-one to
// zero-or-many).
func Resolve(fk *metadata.ForeignKey, byName map[string]*metadata.Table) (Edge, bool) {
	if fk == nil || fk.RefCatalog != "" || fk.RefSchema != "" {
		return Edge{}, false
	}

	var card Cardinality
	if child := byName[fk.Table]; child != nil {
		card.ParentOptional = fkColumnsNullable(child, fk.Columns)
		card.ChildUnique = fkColumnsUnique(child, fk.Columns)
	}

	return Edge{
		Parent: fk.RefTable,
		Child:  fk.Table,
		Card:   card,
		Label:  fk.Name,
	}, true
}

// Index returns a name→table lookup, for use as a renderer's cardinality
// index (e.g. erddot.TableDiagram's cardIndex).
func Index(tables []*metadata.Table) map[string]*metadata.Table {
	byName := make(map[string]*metadata.Table, len(tables))
	for _, t := range tables {
		byName[t.Name] = t
	}
	return byName
}

// KeyMarker returns the "PK", "FK", or "PK,FK" key marker for a column, or
// "" when the column is neither a primary key nor part of an outgoing
// foreign key. fkCols is the set of column names participating in any
// outgoing FK (see [FKColumnSet]).
func KeyMarker(col *metadata.Column, fkCols map[string]bool) string {
	switch {
	case col.PrimaryKey && fkCols[col.Name]:
		return "PK,FK"
	case col.PrimaryKey:
		return "PK"
	case fkCols[col.Name]:
		return "FK"
	default:
		return ""
	}
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

// fkColumnsNullable reports whether any of the named columns on tbl is
// nullable.
func fkColumnsNullable(tbl *metadata.Table, cols []string) bool {
	for _, name := range cols {
		if c := tbl.Column(name); c != nil && c.Nullable {
			return true
		}
	}
	return false
}

// fkColumnsUnique reports whether cols form the primary key or a unique
// constraint on tbl (i.e. the referencing side is itself unique, making the
// relationship one-to-one).
func fkColumnsUnique(tbl *metadata.Table, cols []string) bool {
	var pk []string
	for _, c := range tbl.Columns {
		if c.PrimaryKey {
			pk = append(pk, c.Name)
		}
	}
	if sameColSet(pk, cols) {
		return true
	}
	for _, uc := range tbl.UniqueConstraints {
		if uc != nil && sameColSet(uc.Columns, cols) {
			return true
		}
	}
	return false
}

// sameColSet reports whether a and b contain the same column names, ignoring
// order. An empty set never matches.
func sameColSet(a, b []string) bool {
	if len(a) == 0 || len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	slices.Sort(aa)
	slices.Sort(bb)
	return slices.Equal(aa, bb)
}
