// Package metadata contains types that model source metadata.
package metadata

import (
	"encoding/json"
	"log/slog"
	"maps"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// Source holds metadata for a source.
type Source struct { //nolint:govet // field alignment
	// Handle is the source handle.
	Handle string `json:"handle" yaml:"handle"`

	// Location is the source location such as a DB connection string,
	// a file path, or a URL.
	Location string `json:"location" yaml:"location"`

	// Name is the base name of the source, e.g. the base filename
	// or DB name etc. For example, "sakila".
	Name string `json:"name" yaml:"name"`

	// FQName is the full name of the data source, typically
	// including catalog/schema etc. For example, "sakila.public"
	FQName string `json:"name_fq" yaml:"name_fq"`

	// Schema is the schema name, for example "public".
	// This may be empty for some sources.
	Schema string `json:"schema,omitempty" yaml:"schema,omitempty"`

	// Catalog is the catalog name, for example "sakila".
	Catalog string `json:"catalog,omitempty" yaml:"catalog,omitempty"`

	// Driver is the source driver type.
	Driver drivertype.Type `json:"driver" yaml:"driver"`

	// DBDriver is the type of the underling DB driver.
	// This is the same value as Driver for SQL database types.
	DBDriver drivertype.Type `json:"db_driver" yaml:"db_driver"`

	// DBProduct is the DB product string, such as "PostgreSQL 9.6.17 on x86_64-pc-linux-gnu".
	DBProduct string `json:"db_product" yaml:"db_product"`

	// DBVersion is the DB version.
	DBVersion string `json:"db_version" yaml:"db_version"`

	// User is the username, if applicable.
	User string `json:"user,omitempty" yaml:"user,omitempty"`

	// Size is the physical size of the source in bytes, e.g. DB file size.
	Size int64 `json:"size" yaml:"size"`

	// TableCount is the count of tables (excluding views).
	TableCount int64 `json:"table_count" yaml:"table_count"`

	// ViewCount is the count of views.
	ViewCount int64 `json:"view_count" yaml:"view_count"`

	// Tables is the metadata for each table/view in the source.
	Tables []*Table `json:"tables" yaml:"tables"`

	// DBProperties are name-value pairs from the DB.
	// Typically the value is a scalar such as integer or string, but
	// it can be a nested value such as map or array.
	DBProperties map[string]any `json:"db_properties,omitempty" yaml:"db_properties,omitempty"`
}

// Table returns the named table, or nil.
func (s *Source) Table(tblName string) *Table {
	if s == nil {
		return nil
	}

	for _, tbl := range s.Tables {
		if tbl.Name == tblName {
			return tbl
		}
	}

	return nil
}

// Clone returns a deep copy of md. If md is nil, nil is returned.
func (s *Source) Clone() *Source {
	if s == nil {
		return s
	}

	s2 := &Source{
		Handle:     s.Handle,
		Location:   s.Location,
		Name:       s.Name,
		FQName:     s.FQName,
		Schema:     s.Schema,
		Catalog:    s.Catalog,
		Driver:     s.Driver,
		DBDriver:   s.DBDriver,
		DBProduct:  s.DBProduct,
		DBVersion:  s.DBVersion,
		User:       s.User,
		Size:       s.Size,
		TableCount: s.TableCount,
		ViewCount:  s.ViewCount,
	}

	if s.DBProperties != nil {
		s2.DBProperties = make(map[string]any, len(s.DBProperties))
		maps.Copy(s2.DBProperties, s.DBProperties)
	}

	if s.Tables != nil {
		s2.Tables = make([]*Table, len(s.Tables))
		for i := range s.Tables {
			s2.Tables[i] = s.Tables[i].Clone()
		}
	}

	return s2
}

// TableNames is a convenience method that returns md's table names.
func (s *Source) TableNames() []string {
	names := make([]string, len(s.Tables))
	for i, tblDef := range s.Tables {
		names[i] = tblDef.Name
	}
	return names
}

// String returns a log/debug friendly representation.
func (s *Source) String() string {
	bytes, _ := json.Marshal(s)
	return string(bytes)
}

// Table models table (or view) metadata.
type Table struct { //nolint:govet // field alignment
	// Name is the table name, such as "actor".
	Name string `json:"name" yaml:"name"`

	// FQName is the fully-qualified name, such as "sakila.public.actor"
	FQName string `json:"name_fq,omitempty" yaml:"name_fq,omitempty"`

	// TableType indicates if this is a "table" or "view". The value
	// is driver-independent. See DBTableType for the driver-dependent
	// value.
	TableType string `json:"table_type,omitempty" yaml:"table_type,omitempty"`

	// DBTableType indicates if this is a table or view, etc.
	// The value is driver-dependent, e.g. "BASE TABLE" or "VIEW" for postgres.
	DBTableType string `json:"table_type_db,omitempty" yaml:"table_type_db,omitempty"`

	// RowCount is the number of rows in the table.
	RowCount int64 `json:"row_count" yaml:"row_count"`

	// Size is the physical size of the table in bytes. For a view, this
	// may be nil.
	Size *int64 `json:"size,omitempty" yaml:"size,omitempty"`

	// Comment is the comment for the table. Typically empty.
	Comment string `json:"comment,omitempty" yaml:"comment,omitempty"`

	// Columns holds the metadata for the table's columns.
	Columns []*Column `json:"columns" yaml:"columns"`
}

// String returns a log/debug friendly representation.
func (t *Table) String() string {
	bytes, _ := json.Marshal(t)
	return string(bytes)
}

// Clone returns a deep copy of t. If t is nil, nil is returned.
func (t *Table) Clone() *Table {
	if t == nil {
		return nil
	}

	c := &Table{
		Name:        t.Name,
		FQName:      t.FQName,
		TableType:   t.TableType,
		DBTableType: t.DBTableType,
		RowCount:    t.RowCount,
		Size:        t.Size,
		Comment:     t.Comment,
		Columns:     nil,
	}

	if t.Columns != nil {
		c.Columns = make([]*Column, len(t.Columns))
		for i := range t.Columns {
			c.Columns[i] = t.Columns[i].Clone()
		}
	}

	return c
}

// Column returns the named col or nil.
func (t *Table) Column(colName string) *Column {
	for _, col := range t.Columns {
		if col.Name == colName {
			return col
		}
	}

	return nil
}

// PKCols returns a possibly empty slice of cols that are part
// of the table primary key.
func (t *Table) PKCols() []*Column {
	var pkCols []*Column
	for _, col := range t.Columns {
		if col.PrimaryKey {
			pkCols = append(pkCols, col)
		}
	}

	return pkCols
}

// Column models metadata for a particular column of a data source.
type Column struct { //nolint:govet // field alignment
	Name         string    `json:"name" yaml:"name"`
	Position     int64     `json:"position" yaml:"position"`
	PrimaryKey   bool      `json:"primary_key" yaml:"primary_key"`
	BaseType     string    `json:"base_type" yaml:"base_type"`
	ColumnType   string    `json:"column_type" yaml:"column_type"`
	Kind         kind.Kind `json:"kind" yaml:"kind"`
	Nullable     bool      `json:"nullable" yaml:"nullable"`
	DefaultValue string    `json:"default_value,omitempty" yaml:"default_value,omitempty"`
	Comment      string    `json:"comment,omitempty" yaml:"comment,omitempty"`
	// TODO: Add foreign key field
}

// Clone returns a deep copy of c. If c is nil, nil is returned.
func (c *Column) Clone() *Column {
	if c == nil {
		return nil
	}

	return &Column{
		Name:         c.Name,
		Position:     c.Position,
		PrimaryKey:   c.PrimaryKey,
		BaseType:     c.BaseType,
		ColumnType:   c.ColumnType,
		Kind:         c.Kind,
		Nullable:     c.Nullable,
		DefaultValue: c.DefaultValue,
		Comment:      c.Comment,
	}
}

// String returns a log/debug friendly representation.
func (c *Column) String() string {
	bytes, _ := json.Marshal(c)
	return string(bytes)
}

// Schema models metadata for a schema.
type Schema struct {
	// Name is the schema name, such as "public".
	Name string `json:"schema" yaml:"schema"`

	// Catalog is the catalog name, such as "sakila".
	Catalog string `json:"catalog" yaml:"catalog"`

	// Owner is the schema owner, such as "alice".
	Owner string `json:"owner,omitempty" yaml:"owner,omitempty"`
}

// Clone returns a deep copy of s. If s is nil, nil is returned.
func (s *Schema) Clone() *Schema {
	if s == nil {
		return nil
	}

	s2 := *s
	return &s2
}

// LogValue implements slog.LogValuer.
func (s *Schema) LogValue() slog.Value {
	if s == nil {
		return slog.Value{}
	}

	attrs := make([]slog.Attr, 2, 3)
	attrs[0] = slog.String("name", s.Name)
	attrs[1] = slog.String("catalog", s.Catalog)
	if s.Owner != "" {
		attrs = append(attrs, slog.String("owner", s.Owner))
	}

	return slog.GroupValue(attrs...)
}
