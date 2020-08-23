// Package core.sqlmodel provides functionality for modeling SQL constructs.
package sqlmodel

import (
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// TableDef models a database table definition.
type TableDef struct {
	// Name is the table name.
	Name string `json:"name"`

	// PKColName is the name of the primary key column, or empty.
	//
	// Deprecated: this construct does not allow for composite PK.
	PKColName string `json:"primary_key,omitempty"`

	// AutoIncrement, if true, indicates that a PK column
	// should autoincrement.
	//
	// Deprecated: this construct does not allow for composite PK.
	AutoIncrement bool `json:"auto_increment"`

	// Cols is the table's column definitions.
	Cols []*ColDef `json:"cols"`
}

// NewTableDef is a convenience constructor for creating
// a simple table definition.
func NewTableDef(tblName string, colNames []string, colKinds []kind.Kind) *TableDef {
	tblDef := &TableDef{Name: tblName}
	cols := make([]*ColDef, len(colNames))

	for i := range colNames {
		cols[i] = &ColDef{Table: tblDef, Name: colNames[i], Kind: colKinds[i]}
	}

	tblDef.Cols = cols
	return tblDef
}

// ColNames returns a new slice containing the names
// of t's columns.
func (t *TableDef) ColNames() []string {
	names := make([]string, len(t.Cols))
	for i, col := range t.Cols {
		names[i] = col.Name
	}
	return names
}

// ColKinds returns a new slice containing the kinds
// of t's columns.
func (t *TableDef) ColKinds() []kind.Kind {
	kinds := make([]kind.Kind, len(t.Cols))
	for i, col := range t.Cols {
		kinds[i] = col.Kind
	}
	return kinds
}

func (t *TableDef) String() string {
	return stringz.SprintJSON(t)
}

// ColsByName returns the ColDefs for each named column, or an error if any column
// is not matched.
func (t *TableDef) ColsByName(cols []string) ([]*ColDef, error) {
	defs := make([]*ColDef, len(cols))

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
			return nil, errz.Errorf("could not find column definition %q in table %q", name, t.Name)
		}
	}
	return defs, nil
}

// FindCol returns the named ColDef or nil if not found.
func (t *TableDef) FindCol(name string) (*ColDef, error) {
	for _, col := range t.Cols {
		if col.Name == name {
			return col, nil
		}
	}
	return nil, errz.Errorf("could not find column definition %q in table %q", name, t.Name)
}

// ColDef models a table column definition.
type ColDef struct {
	Name  string    `json:"name"`
	Table *TableDef `json:"-"`
	Kind  kind.Kind `json:"kind"`

	NotNull    bool `json:"not_null"`
	HasDefault bool `json:"has_default"`

	// Size typically applies to text fields, e.g. VARCHAR(255).
	Size int `json:"size"`

	Unique     bool          `json:"unique"`
	ForeignKey *FKConstraint `json:"foreign_key,omitempty"`
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
