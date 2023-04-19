package source

import (
	"encoding/json"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
)

// Metadata holds metadata for a source.
type Metadata struct {
	// Handle is the source handle.
	Handle string `json:"handle"`

	// Location is the source location such as a DB connection string,
	// a file path, or a URL.
	Location string `json:"location"`

	// Name is the base name of the source, e.g. the base filename
	// or DB name etc. For example, "sakila".
	Name string `json:"name"`

	// FQName is the full name of the data source, typically
	// including catalog/schema etc. For example, "sakila.public"
	FQName string `json:"name_fq"`

	// Schema is the schema name, for example "public".
	// This may be empty for some sources.
	Schema string `json:"schema,omitempty"`

	// SourceType is the source driver type.
	SourceType Type `json:"driver"`

	// DBDriverType is the type of the underling DB driver.
	// This is the same value as SourceType for SQL database types.
	DBDriverType Type `json:"db_driver"`

	// DBProduct is the DB product string, such as "PostgreSQL 9.6.17 on x86_64-pc-linux-gnu".
	DBProduct string `json:"db_product"`

	// DBVersion is the DB version.
	DBVersion string `json:"db_version"`

	// User is the username, if applicable.
	User string `json:"user,omitempty"`

	// Size is the physical size of the source loc bytes, e.g. DB file size.
	Size int64 `json:"size"`

	// Tables is the metadata for each table loc the source.
	Tables []*TableMetadata `json:"tables"`

	// DBVars are configuration name-value pairs from the DB.
	DBVars []DBVar `json:"db_variables,omitempty"`
}

// Table returns the named table, or nil.
func (md *Metadata) Table(tblName string) *TableMetadata {
	if md == nil {
		return nil
	}

	for _, tbl := range md.Tables {
		if tbl.Name == tblName {
			return tbl
		}
	}

	return nil
}

// Clone returns a deep copy of md. If md is nil, nil is returned.
func (md *Metadata) Clone() *Metadata {
	if md == nil {
		return md
	}

	c := &Metadata{
		Handle:       md.Handle,
		Location:     md.Location,
		Name:         md.Name,
		FQName:       md.FQName,
		SourceType:   md.SourceType,
		DBDriverType: md.DBDriverType,
		DBProduct:    md.DBProduct,
		DBVersion:    md.DBVersion,
		User:         md.User,
		Size:         md.Size,
		Tables:       nil,
		DBVars:       nil,
	}

	if md.DBVars != nil {
		copy(c.DBVars, md.DBVars)
	}

	if md.Tables != nil {
		c.Tables = make([]*TableMetadata, len(md.Tables))
		for i := range md.Tables {
			c.Tables[i] = md.Tables[i].Clone()
		}
	}

	return c
}

// TableNames is a convenience method that returns md's table names.
func (md *Metadata) TableNames() []string {
	names := make([]string, len(md.Tables))
	for i, tblDef := range md.Tables {
		names[i] = tblDef.Name
	}
	return names
}

func (md *Metadata) String() string {
	bytes, _ := json.Marshal(md)
	return string(bytes)
}

// DBVar models a key-value pair for driver config.
type DBVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// TableMetadata models table (or view) metadata.
type TableMetadata struct {
	// Name is the table name, such as "actor".
	Name string `json:"name"`

	// FQName is the fully-qualified name, such as "sakila.public.actor"
	FQName string `json:"name_fq,omitempty"`

	// TableType indicates if this is a "table" or "view". The value
	// is driver-independent. See DBTableType for the driver-dependent
	// value.
	TableType string `json:"table_type,omitempty"`

	// DBTableType indicates if this is a table or view, etc.
	// The value is driver-dependent, e.g. "BASE TABLE" or "VIEW" for postgres.
	DBTableType string `json:"table_type_db,omitempty"`

	// RowCount is the number of rows loc the table.
	RowCount int64 `json:"row_count"`

	// Size is the physical size of the table loc bytes. For a view, this
	// may be nil.
	Size *int64 `json:"size,omitempty"`

	// Comment is the comment for the table. Typically empty.
	Comment string `json:"comment,omitempty"`

	// Columns holds the metadata for the table's columns.
	Columns []*ColMetadata `json:"columns"`
}

func (t *TableMetadata) String() string {
	bytes, _ := json.Marshal(t)
	return string(bytes)
}

// Clone returns a deep copy of t. If t is nil, nil is returned.
func (t *TableMetadata) Clone() *TableMetadata {
	if t == nil {
		return nil
	}

	c := &TableMetadata{
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
		c.Columns = make([]*ColMetadata, len(t.Columns))
		for i := range t.Columns {
			c.Columns[i] = t.Columns[i].Clone()
		}
	}

	return c
}

// Column returns the named col or nil.
func (t *TableMetadata) Column(colName string) *ColMetadata {
	for _, col := range t.Columns {
		if col.Name == colName {
			return col
		}
	}

	return nil
}

// PKCols returns a possibly empty slice of cols that are part
// of the table primary key.
func (t *TableMetadata) PKCols() []*ColMetadata {
	var pkCols []*ColMetadata
	for _, col := range t.Columns {
		if col.PrimaryKey {
			pkCols = append(pkCols, col)
		}
	}

	return pkCols
}

// ColMetadata models metadata for a particular column of a data source.
type ColMetadata struct {
	Name         string    `json:"name"`
	Position     int64     `json:"position"`
	PrimaryKey   bool      `json:"primary_key"`
	BaseType     string    `json:"base_type"`
	ColumnType   string    `json:"column_type"`
	Kind         kind.Kind `json:"kind"`
	Nullable     bool      `json:"nullable"`
	DefaultValue string    `json:"default_value,omitempty"`
	Comment      string    `json:"comment,omitempty"`
	// TODO: Add foreign key field
}

// Clone returns a deep copy of c. If c is nil, nil is returned.
func (c *ColMetadata) Clone() *ColMetadata {
	if c == nil {
		return nil
	}

	return &ColMetadata{
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

func (c *ColMetadata) String() string {
	bytes, _ := json.Marshal(c)
	return string(bytes)
}

// TableFromSourceMetadata returns TableMetadata whose name matches
// tblName.
//
// Deprecated: Each driver should implement this correctly for a single table.
func TableFromSourceMetadata(srcMeta *Metadata, tblName string) (*TableMetadata, error) {
	for _, tblMeta := range srcMeta.Tables {
		if tblMeta.Name == tblName {
			return tblMeta, nil
		}
	}
	return nil, errz.Errorf("metadata for table %s not found", tblName)
}
