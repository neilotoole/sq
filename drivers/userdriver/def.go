package userdriver

import (
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// DriverDef is a user-defined driver definition.
type DriverDef struct {
	// Name is short name of the driver type, e.g. "rss".
	Name string `yaml:"driver" json:"driver"`

	// Genre is the generic document type, e.g. XML or JSON etc.
	Genre string `yaml:"genre" json:"genre"`

	// Title is the full name of the driver
	// type, e.g. "RSS (Really Simple Syndication)".
	Title string `yaml:"title" json:"title"`

	// Doc typically has a link to documentation for the driver..
	Doc string `yaml:"doc,omitempty" json:"doc,omitempty"`

	// Selector is the root doc element, e.g. "/rss"
	Selector string `yaml:"selector" json:"selector"`

	// Tables is the set of tables that define the type.
	Tables []*TableMapping `yaml:"tables" json:"tables"`
}

// TableBySelector returns the TableMapping that matches sel, or nil.
func (d *DriverDef) TableBySelector(sel string) *TableMapping {
	for _, t := range d.Tables {
		if t.Selector == sel {
			return t
		}
	}
	return nil
}

func (d *DriverDef) String() string {
	return stringz.SprintJSON(d)
}

// TableMapping describes how document data is mapped to a table.
type TableMapping struct {
	// Name is the table name.
	Name string `yaml:"table" json:"table"`

	// Selector specifies how the table data is selected.
	Selector string `yaml:"selector" json:"selector"`

	// Cols is the set of columns in the table.
	Cols []*ColMapping `yaml:"cols" json:"cols"`

	// PrimaryKey is a slice of the column names that constitute
	// the primary key. Typically this is one column, but can be
	// more than one for composite primary keys.
	PrimaryKey []string `yaml:"primary_key" json:"primary_key"`

	// Comment is an optional table comment.
	Comment string `yaml:"comment,omitempty" json:"comment,omitempty"`
}

func (t *TableMapping) String() string {
	return stringz.SprintJSON(t)
}

// ColBySelector returns the ColMapping associated with the element, or nil if no such col.
func (t *TableMapping) ColBySelector(sel string) *ColMapping {
	if sel == "" {
		return nil
	}

	for _, col := range t.Cols {
		if sel == t.absColSelector(col) {
			return col
		}
	}

	for _, col := range t.Cols {
		if sel == t.Selector+"/"+col.Name {
			return col
		}
	}
	return nil
}

// absColSelector returns an absolute (fully-qualified) selector for the provided ColMapping. For
// example, if ColMapping.Selector is "./item", the return value might be "/rss/channel/item".
func (t *TableMapping) absColSelector(col *ColMapping) string {
	if col.Selector == "" {
		return ""
	}

	if col.Selector[0] == '/' {
		return col.Selector
	}

	if strings.HasPrefix(col.Selector, "./") {
		return t.Selector + col.Selector[1:]
	}

	return t.Selector + "/" + col.Selector
}

// PKCols returns the cols that constitute this table's primary key,
// or an error if none defined. If error is non-nil, the returned
// slice will contain at least one ColMapping.
func (t *TableMapping) PKCols() ([]*ColMapping, error) {
	var cols []*ColMapping

	for i := range t.Cols {
		for j := range t.PrimaryKey {
			if t.Cols[i].Name == t.PrimaryKey[j] {
				cols = append(cols, t.Cols[i])
				break
			}
		}
	}

	if len(cols) == 0 {
		return nil, errz.Errorf("no primary key column(s) defined for table {%s}", t.Name)
	}

	return cols, nil
}

// SequenceCols returns the cols whose selector value is "../sequence()".
// In effect, this method returns the columns for whom a sequence value
// should be set during a db insert, similar to a db auto-increment column.
func (t *TableMapping) SequenceCols() []*ColMapping {
	var cols []*ColMapping

	for i := range t.Cols {
		if t.Cols[i].Selector == "../sequence()" {
			cols = append(cols, t.Cols[i])
		}
	}

	return cols
}

// RequiredCols returns the cols that are required. This includes columns
// with explicit ColMapping.Required field as well as other columns such
// as those part of the primary key or sequence cols.
func (t *TableMapping) RequiredCols() []*ColMapping {
	var cols []*ColMapping

	pkCols, _ := t.PKCols()
	seqCols := t.SequenceCols()

	for _, col := range t.Cols {
		col := col

		switch {
		case col.Required, colIndex(pkCols, col) >= 0, colIndex(seqCols, col) >= 0:
			cols = append(cols, col)
		default:
		}
	}

	return cols
}

// ColMapping models a database table column.
type ColMapping struct {
	// Name is the column name.
	Name string `yaml:"col" json:"col"`

	// Selector is an optional selector for the col value, e.g. "./guid/@isPermaLink" for an attribute of an XML element.
	Selector string `yaml:"selector,omitempty" json:"selector,omitempty"`

	// Kind is the data kind, e.g. "int", "text.
	Kind kind.Kind `yaml:"kind" json:"kind"`

	// Format is an optional type format for text values, e.g. "RFC3339" for a string.
	Format string `yaml:"format,omitempty" json:"format,omitempty"`

	// Charset is an optional charset for text values, e.g. "utf-8".
	Charset string `yaml:"charset,omitempty" json:"charset,omitempty"`

	// Foreign indicates that this column is a foreign key into a parent tbl.
	Foreign string `yaml:"foreign,omitempty" json:"foreign,omitempty"`

	// Unique is true if the column value is unique.
	Unique bool `yaml:"unique,omitempty" json:"unique,omitempty"`

	// Required is true if the column is required.
	Required bool `yaml:"required" json:"required"`

	// Comment is an optional column comment.
	Comment string `yaml:"comment,omitempty" json:"comment,omitempty"`
}

func (c *ColMapping) String() string {
	return stringz.SprintJSON(c)
}

// ValidateDriverDef checks that def is valid, returning one or
// more errors if not.
func ValidateDriverDef(def *DriverDef) []error {
	drvrName, errs := validateDefRoot(def)
	if len(errs) > 0 {
		return errs
	}

	for i, tbl := range def.Tables {
		tblName := fmt.Sprintf("%s.table[%d]", drvrName, i)
		if tbl.Name == "" {
			errs = append(errs, errz.Errorf("%s name is empty", tblName))
		} else {
			tblName = fmt.Sprintf("%s.table[%s]", drvrName, tbl.Name)
		}

		if tbl.Selector == "" {
			errs = append(errs, errz.Errorf("%s selector is empty", tblName))
		}
		if len(tbl.Cols) == 0 {
			errs = append(errs, errz.Errorf("%s cols is empty", tblName))
			continue
		}

		if len(tbl.PrimaryKey) == 0 {
			errs = append(errs, errz.Errorf("%s primary key must list at least one column", tblName))
		} else {
			for j, pkColName := range tbl.PrimaryKey {
				if pkColName == "" {
					errs = append(errs, errz.Errorf("%s primary key %d has empty name", tblName, j))
					continue
				}

				// verify that the pk col exists in the cols
				var foundIt bool
				for k := range tbl.Cols {
					if pkColName == tbl.Cols[k].Name {
						foundIt = true
						break
					}
				}
				if !foundIt {
					errs = append(errs,
						errz.Errorf("{%s} specified primary key {%s} not found in cols", tblName, pkColName))
				}
			}
		}

		for j, col := range tbl.Cols {
			colName := fmt.Sprintf("%s.col[%d]", tblName, j)
			if col.Name == "" {
				errs = append(errs, errz.Errorf("{%s} name is empty", colName))
			} else {
				colName = fmt.Sprintf("%s.col[%s]", tblName, col.Name)
			}

			// These kinds are nonsensical
			switch col.Kind { //nolint:exhaustive
			default:
			case kind.Unknown, kind.Null:
				errs = append(errs, errz.Errorf("{%s}.kind {%s} is invalid", colName, col.Kind))
			}
		}
	}

	return errs
}

func validateDefRoot(def *DriverDef) (drvrName string, errs []error) {
	if def == nil {
		// shouldn't happen
		errs = append(errs, errz.New("def is nil"))
		return "", errs
	}

	if def.Name == "" {
		errs = append(errs, errz.New("driver name is empty"))
		return "", errs
	}

	drvrName = fmt.Sprintf("driver[%s]", def.Name)
	if def.Genre == "" {
		errs = append(errs, errz.Errorf("%s.genre is empty", drvrName))
	}
	if def.Selector == "" {
		errs = append(errs, errz.Errorf("%s.selector is empty", drvrName))
	}
	if def.Title == "" {
		errs = append(errs, errz.Errorf("%s.title is empty", drvrName))
	}
	if len(def.Tables) == 0 {
		errs = append(errs, errz.Errorf("%s.tables is empty", drvrName))
	}

	return drvrName, errs
}

// ToTableDef builds a TableDef from the TableMapping.
func ToTableDef(tblMapping *TableMapping) (*sqlmodel.TableDef, error) {
	tblDef := &sqlmodel.TableDef{Name: tblMapping.Name}
	colDefs := make([]*sqlmodel.ColDef, len(tblMapping.Cols))

	pkCols, err := tblMapping.PKCols()
	if err != nil {
		return nil, err
	}

	tblDef.PKColName = pkCols[0].Name

	for i, colMapping := range tblMapping.Cols {
		colDef := &sqlmodel.ColDef{Table: tblDef, Name: colMapping.Name, Kind: colMapping.Kind}
		colDefs[i] = colDef
	}

	tblDef.Cols = colDefs
	return tblDef, nil
}

// NamesFromCols is a convenience function that returns a slice
// containing the name of each column.
func NamesFromCols(cols []*ColMapping) []string {
	if cols == nil {
		return nil
	}

	names := make([]string, len(cols))
	for i := range cols {
		names[i] = cols[i].Name
	}

	return names
}

// colIndex returns the index of needle in haystack, or -1.
func colIndex(haystack []*ColMapping, needle *ColMapping) int {
	for i := range haystack {
		if haystack[i] == needle {
			return i
		}
	}

	return -1
}

// Detector defines a document type detector.
type Detector struct {
	// Type is the detector type, e.g. "suffix", "header", "scheme", etc.
	Type string `yaml:"type" json:"type"`
	// Key is the expected match for the detector's key field. E.g. "Content-Type". May be empty.
	Key string `yaml:"key,omitempty" json:"key,omitempty"`
	// Value is the expected match for the detector's value field. E.g. "application/rss+xml"
	Value string `yaml:"value" json:"value"`
	// Example is an example value that would match the detector, e.g. "Content-Type: application/rss+xml"
	Example string `yaml:"example,omitempty" json:"example,omitempty"`
}

func (d *Detector) String() string {
	return stringz.SprintJSON(d)
}
