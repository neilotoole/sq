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

	// Name is the base name of the source, e.g. the base filename
	// or DB name etc. For example, "sakila".
	Name string `json:"name"`

	// FQName is the full name of the data source, typically
	// including catalog/schema etc. For example, "sakila.public"
	FQName string `json:"name_fq"`

	// SourceType is the source driver type.
	SourceType Type `json:"driver"`

	// DBDriverType is the type of the underling DB driver.
	// This is the same value as SourceType for SQL database types.
	DBDriverType Type `json:"db_driver"`

	// DBProduct is the DB product string, such as "PostgreSQL 9.6.17 on x86_64-pc-linux-gnu".
	DBProduct string `json:"db_product"`

	// DBVersion is the DB version.
	DBVersion string `json:"db_version"`

	// DBVars are configuration name-value pairs from the DB.
	DBVars []DBVar `json:"db_variables,omitempty"`

	// Location is the source location such as a DB connection string,
	// a file path, or a URL.
	Location string `json:"location"`

	// User is the username, if applicable.
	User string `json:"user,omitempty"`

	// Size is the physical size of the source loc bytes, e.g. DB file size.
	Size int64 `json:"size"`

	// Tables is the metadata for each table loc the source.
	Tables []*TableMetadata `json:"tables"`
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
// REVISIT: maybe better named as SourceSetting or such?
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
