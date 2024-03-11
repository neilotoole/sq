// Package schema provides functionality for modeling SQL constructs.
package schema

import (
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
)

// Table models a database table definition.
type Table struct { //nolint:govet // field alignment
	// Name is the table name.
	Name string `json:"name"`

	// PKColName is the name of the primary key column, or empty.
	//
	// REVISIT: this construct does not allow for composite PK.
	PKColName string `json:"primary_key,omitempty"`

	// AutoIncrement, if true, indicates that a PK column
	// should autoincrement.
	//
	// REVISIT: this construct does not allow for composite PK.
	AutoIncrement bool `json:"auto_increment"`

	// Cols is the table's column definitions.
	Cols []*Column `json:"cols"`
}

func (t *Table) Equal(b *Table) bool {
	if t == nil && b == nil {
		return true
	}
	if t == nil || b == nil {
		return false
	}
	if t == b {
		return true
	}

	if t.Name != b.Name {
		return false
	}

	if t.PKColName != b.PKColName {
		return false
	}

	if t.AutoIncrement != b.AutoIncrement {
		return false
	}

	if len(t.Cols) != len(b.Cols) {
		return false
	}

	for i, col := range t.Cols {
		if !col.Equal(b.Cols[i]) {
			return false
		}
	}

	return true
}

// NewTable is a convenience constructor for creating
// a simple table definition.
func NewTable(tblName string, colNames []string, colKinds []kind.Kind) *Table {
	tblDef := &Table{Name: tblName}
	cols := make([]*Column, len(colNames))

	for i := range colNames {
		cols[i] = &Column{Table: tblDef, Name: colNames[i], Kind: colKinds[i]}
	}

	tblDef.Cols = cols
	return tblDef
}

// ColNames returns a new slice containing the names
// of t's columns.
func (t *Table) ColNames() []string {
	names := make([]string, len(t.Cols))
	for i, col := range t.Cols {
		names[i] = col.Name
	}
	return names
}

// ColKinds returns a new slice containing the kinds
// of t's columns.
func (t *Table) ColKinds() []kind.Kind {
	kinds := make([]kind.Kind, len(t.Cols))
	for i, col := range t.Cols {
		kinds[i] = col.Kind
	}
	return kinds
}

func (t *Table) String() string {
	return t.Name
}

// ColsByName returns the ColDefs for each named column, or an error if any column
// is not matched.
func (t *Table) ColsByName(cols []string) ([]*Column, error) {
	defs := make([]*Column, len(cols))

	for i, name := range cols {
		found := false
		for _, def := range t.Cols {
			if def.Name == name {
				defs[i] = def
				found = true
				break
			}
		}

		if !found {
			return nil, errz.Errorf("could not find column definition {%s} in table {%s}", name, t.Name)
		}
	}
	return defs, nil
}

// FindCol returns the named Column or nil if not found.
func (t *Table) FindCol(name string) (*Column, error) {
	for _, col := range t.Cols {
		if col.Name == name {
			return col, nil
		}
	}
	return nil, errz.Errorf("could not find column definition {%s} in table {%s}", name, t.Name)
}

// Column models a table column definition.
type Column struct { //nolint:govet // field alignment
	Name  string    `json:"name"`
	Table *Table    `json:"-"`
	Kind  kind.Kind `json:"kind"`

	NotNull    bool `json:"not_null"`
	HasDefault bool `json:"has_default"`

	// Size typically applies to text fields, e.g. VARCHAR(255).
	Size int `json:"size"`

	Unique     bool          `json:"unique"`
	ForeignKey *FKConstraint `json:"foreign_key,omitempty"`
}

func (c *Column) Equal(b *Column) bool {
	if c == nil && b == nil {
		return true
	}
	if c == nil || b == nil {
		return false
	}
	if c == b {
		return true
	}

	if c.Name != b.Name {
		return false
	}

	if c.Kind != b.Kind {
		return false
	}

	if c.NotNull != b.NotNull {
		return false
	}

	if c.HasDefault != b.HasDefault {
		return false
	}

	if c.Size != b.Size {
		return false
	}

	if c.Unique != b.Unique {
		return false
	}

	if !c.ForeignKey.Equal(b.ForeignKey) {
		return false
	}

	return true

}

func (c *Column) String() string {
	return c.Name
}

// FKConstraint models a foreign key constraint.
type FKConstraint struct {
	// RefTable is the name of the referenced parent table.
	RefTable string `json:"ref_table"`
	// RefCol is the name of the referenced col in the parent table.
	RefCol string `json:"ref_col"`
	// OnDelete is one of CASCADE or SET_NULL, defaults to CASCADE.
	OnDelete string `json:"on_delete"`
	// OnUpdate is one of CASCADE or SET_NULL, defaults to CASCADE.
	OnUpdate string `json:"on_update"`
}

func (fk *FKConstraint) Equal(b *FKConstraint) bool {
	if fk == nil && b == nil {
		return true
	}
	if fk == nil || b == nil {
		return false
	}
	if fk == b {
		return true
	}

	if fk.RefTable != b.RefTable {
		return false
	}

	if fk.RefCol != b.RefCol {
		return false
	}

	if fk.OnDelete != b.OnDelete {
		return false
	}

	if fk.OnUpdate != b.OnUpdate {
		return false
	}

	return true
}
