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

		// Per-table Clone() copies outgoing ForeignKeys as independent
		// values; re-derive the back-references so Column.ForeignKey and
		// Table.ReferencedBy on the clone share identity with the cloned
		// outgoing FKs rather than pointing at the originals.
		LinkForeignKeys(s2)
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

	// ForeignKeys are the outgoing foreign-key constraints declared on
	// this table (i.e. constraints whose referencing side is this table).
	// May be nil for sources that don't support foreign keys, or for
	// tables that declare none.
	ForeignKeys []*ForeignKey `json:"foreign_keys,omitempty" yaml:"foreign_keys,omitempty"`

	// ReferencedBy are the incoming foreign-key constraints that point
	// at this table (i.e. constraints declared on other tables whose
	// referenced side is this table). This is derived from the outgoing
	// foreign keys of every table in the source and is populated by
	// [LinkForeignKeys] after all tables have been loaded.
	ReferencedBy []*ForeignKey `json:"referenced_by,omitempty" yaml:"referenced_by,omitempty"`

	// UniqueConstraints are the unique-constraint declarations on this
	// table (UNIQUE in CREATE TABLE, or ALTER TABLE ADD CONSTRAINT ...
	// UNIQUE). Primary keys are *not* repeated here — they are reported
	// via [Column.PrimaryKey] and also appear in [Table.Indexes] as
	// an entry with [Index.Primary]=true. Composite unique constraints
	// are returned with their columns in declaration order.
	UniqueConstraints []*UniqueConstraint `json:"unique_constraints,omitempty" yaml:"unique_constraints,omitempty"`

	// Indexes are the physical indexes that back this table — including
	// the implicit unique index that implements each unique constraint
	// and primary key. Use [Index.Unique] / [Index.Primary] to
	// distinguish kinds. Non-unique secondary indexes (e.g. created via
	// CREATE INDEX for performance) also appear here.
	Indexes []*Index `json:"indexes,omitempty" yaml:"indexes,omitempty"`
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

	if t.ForeignKeys != nil {
		c.ForeignKeys = make([]*ForeignKey, len(t.ForeignKeys))
		for i := range t.ForeignKeys {
			c.ForeignKeys[i] = t.ForeignKeys[i].Clone()
		}
	}

	if t.ReferencedBy != nil {
		c.ReferencedBy = make([]*ForeignKey, len(t.ReferencedBy))
		for i := range t.ReferencedBy {
			c.ReferencedBy[i] = t.ReferencedBy[i].Clone()
		}
	}

	if t.UniqueConstraints != nil {
		c.UniqueConstraints = make([]*UniqueConstraint, len(t.UniqueConstraints))
		for i := range t.UniqueConstraints {
			c.UniqueConstraints[i] = t.UniqueConstraints[i].Clone()
		}
	}

	if t.Indexes != nil {
		c.Indexes = make([]*Index, len(t.Indexes))
		for i := range t.Indexes {
			c.Indexes[i] = t.Indexes[i].Clone()
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

	// ForeignKey is the outgoing foreign-key constraint that this column
	// participates in, or nil if none. For composite foreign keys, this
	// pointer references the same ForeignKey shared by every participating
	// column. The canonical list lives on Table.ForeignKeys; this field is
	// a convenience back-reference populated by [LinkForeignKeys].
	ForeignKey *ForeignKey `json:"foreign_key,omitempty" yaml:"foreign_key,omitempty"`
}

// Clone returns a deep copy of c. If c is nil, nil is returned.
//
// Note that Column.ForeignKey is not carried across a Column-level clone:
// it is a back-reference into the owning [Table]'s ForeignKeys slice and
// must be re-linked at the [Source] level via [LinkForeignKeys].
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

// ForeignKey models a single foreign-key constraint between two tables.
// Constraints are populated by the driver from the perspective of the
// referencing table (as Table.ForeignKeys); the corresponding incoming
// entries on the referenced table's [Table.ReferencedBy] slice are
// derived by [LinkForeignKeys] once all tables have been loaded.
//
// Composite foreign keys are represented by populating Columns and
// RefColumns with multiple entries in matching positional order.
type ForeignKey struct { //nolint:govet // field alignment
	// Name is the constraint name as reported by the source, such as
	// "fk_film_language". May be empty for sources that don't expose
	// a constraint name (e.g. some SQLite tables).
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Table is the name of the referencing table — the table on which
	// the constraint is declared. This is the bare table name (not
	// fully qualified) and matches the owning [Table.Name].
	Table string `json:"table" yaml:"table"`

	// Columns are the column names on the referencing table, in
	// positional order. For composite keys, the order must match
	// RefColumns.
	Columns []string `json:"columns" yaml:"columns"`

	// RefCatalog is the catalog of the referenced table, if known.
	// Empty when the reference is in the same catalog or the source
	// doesn't expose catalogs.
	RefCatalog string `json:"ref_catalog,omitempty" yaml:"ref_catalog,omitempty"`

	// RefSchema is the schema of the referenced table, if known.
	// Empty when the reference is in the same schema or the source
	// doesn't expose schemas.
	RefSchema string `json:"ref_schema,omitempty" yaml:"ref_schema,omitempty"`

	// RefTable is the name of the referenced table.
	RefTable string `json:"ref_table" yaml:"ref_table"`

	// RefColumns are the column names on the referenced table, in
	// positional order corresponding to Columns.
	RefColumns []string `json:"ref_columns" yaml:"ref_columns"`

	// OnDelete is the referential action on DELETE, such as
	// "NO ACTION", "CASCADE", "SET NULL", or "RESTRICT". Empty
	// when not reported by the source.
	OnDelete string `json:"on_delete,omitempty" yaml:"on_delete,omitempty"`

	// OnUpdate is the referential action on UPDATE. Empty when
	// not reported by the source.
	OnUpdate string `json:"on_update,omitempty" yaml:"on_update,omitempty"`
}

// Clone returns a deep copy of fk. If fk is nil, nil is returned.
func (fk *ForeignKey) Clone() *ForeignKey {
	if fk == nil {
		return nil
	}

	c := &ForeignKey{
		Name:       fk.Name,
		Table:      fk.Table,
		RefCatalog: fk.RefCatalog,
		RefSchema:  fk.RefSchema,
		RefTable:   fk.RefTable,
		OnDelete:   fk.OnDelete,
		OnUpdate:   fk.OnUpdate,
	}

	if fk.Columns != nil {
		c.Columns = append([]string(nil), fk.Columns...)
	}
	if fk.RefColumns != nil {
		c.RefColumns = append([]string(nil), fk.RefColumns...)
	}

	return c
}

// String returns a log/debug friendly representation.
func (fk *ForeignKey) String() string {
	bytes, _ := json.Marshal(fk)
	return string(bytes)
}

// UniqueConstraint models a single UNIQUE constraint declared on a
// table — either inline in CREATE TABLE or via ALTER TABLE ADD
// CONSTRAINT ... UNIQUE. Primary keys are reported separately via
// [Column.PrimaryKey] and are not represented here.
//
// Composite unique constraints are returned as a single
// UniqueConstraint with Columns ordered as declared.
type UniqueConstraint struct {
	// Name is the constraint name as reported by the source, such as
	// "address_postal_code_key". May be empty when the source doesn't
	// expose a user-visible constraint name (some SQLite cases).
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Table is the bare name of the owning table (matches the owning
	// [Table.Name]).
	Table string `json:"table" yaml:"table"`

	// Columns are the column names that participate in the constraint,
	// in declaration order.
	Columns []string `json:"columns" yaml:"columns"`
}

// Clone returns a deep copy of uc. If uc is nil, nil is returned.
func (uc *UniqueConstraint) Clone() *UniqueConstraint {
	if uc == nil {
		return nil
	}
	c := &UniqueConstraint{
		Name:  uc.Name,
		Table: uc.Table,
	}
	if uc.Columns != nil {
		c.Columns = append([]string(nil), uc.Columns...)
	}
	return c
}

// String returns a log/debug friendly representation.
func (uc *UniqueConstraint) String() string {
	bytes, _ := json.Marshal(uc)
	return string(bytes)
}

// Index models a physical index defined on a table. Indexes include the
// implicit ones that back primary keys and unique constraints — use the
// [Index.Primary] and [Index.Unique] flags to distinguish. Drivers that
// don't expose a meaningful access-method name leave [Index.Type]
// empty.
//
// Indexes are reported per table in [Table.Indexes].
type Index struct { //nolint:govet // field alignment
	// Name is the index name as reported by the source.
	Name string `json:"name" yaml:"name"`

	// Table is the bare name of the owning table.
	Table string `json:"table" yaml:"table"`

	// Columns are the indexed column names, in key order. Expressions
	// (e.g. functional indexes) are not represented here; only direct
	// column references appear.
	Columns []string `json:"columns" yaml:"columns"`

	// Unique reports whether the index enforces uniqueness on Columns.
	// True for primary-key and unique-constraint backing indexes as
	// well as for any user-declared UNIQUE INDEX.
	Unique bool `json:"unique" yaml:"unique"`

	// Primary reports whether the index backs this table's primary key.
	Primary bool `json:"primary,omitempty" yaml:"primary,omitempty"`

	// Type is the driver-specific access-method name, such as "BTREE",
	// "HASH", "GIN", or "NONCLUSTERED". Empty when the source doesn't
	// report it.
	Type string `json:"type,omitempty" yaml:"type,omitempty"`
}

// Clone returns a deep copy of i. If i is nil, nil is returned.
func (i *Index) Clone() *Index {
	if i == nil {
		return nil
	}
	c := &Index{
		Name:    i.Name,
		Table:   i.Table,
		Unique:  i.Unique,
		Primary: i.Primary,
		Type:    i.Type,
	}
	if i.Columns != nil {
		c.Columns = append([]string(nil), i.Columns...)
	}
	return c
}

// String returns a log/debug friendly representation.
func (i *Index) String() string {
	bytes, _ := json.Marshal(i)
	return string(bytes)
}

// AssignForeignKeys groups fks by their referencing-table name (the
// ForeignKey.Table field) and assigns each group to the matching entry
// in tables. Tables with no matching FKs retain their existing
// [Table.ForeignKeys] slice; callers that want to replace rather than
// merge should clear the slice first.
//
// This helper exists so that driver implementations that fetch all
// foreign keys in a single source-wide query (postgres, mysql,
// sqlserver, oracle) don't each have to repeat the same grouping loop.
func AssignForeignKeys(tables []*Table, fks []*ForeignKey) {
	if len(fks) == 0 {
		return
	}

	byTable := make(map[string][]*ForeignKey, len(fks))
	for _, fk := range fks {
		if fk == nil {
			continue
		}
		byTable[fk.Table] = append(byTable[fk.Table], fk)
	}

	for _, tbl := range tables {
		if tbl == nil {
			continue
		}
		if tblFKs, ok := byTable[tbl.Name]; ok {
			tbl.ForeignKeys = tblFKs
		}
	}
}

// AssignUniqueConstraints groups ucs by their owning-table name and
// assigns each group to the matching entry in tables. The analog of
// [AssignForeignKeys] for unique-constraint slices.
func AssignUniqueConstraints(tables []*Table, ucs []*UniqueConstraint) {
	if len(ucs) == 0 {
		return
	}

	byTable := make(map[string][]*UniqueConstraint, len(ucs))
	for _, uc := range ucs {
		if uc == nil {
			continue
		}
		byTable[uc.Table] = append(byTable[uc.Table], uc)
	}

	for _, tbl := range tables {
		if tbl == nil {
			continue
		}
		if tblUCs, ok := byTable[tbl.Name]; ok {
			tbl.UniqueConstraints = tblUCs
		}
	}
}

// AssignIndexes groups idxs by their owning-table name and assigns
// each group to the matching entry in tables. The analog of
// [AssignForeignKeys] for index slices.
func AssignIndexes(tables []*Table, idxs []*Index) {
	if len(idxs) == 0 {
		return
	}

	byTable := make(map[string][]*Index, len(idxs))
	for _, idx := range idxs {
		if idx == nil {
			continue
		}
		byTable[idx.Table] = append(byTable[idx.Table], idx)
	}

	for _, tbl := range tables {
		if tbl == nil {
			continue
		}
		if tblIdxs, ok := byTable[tbl.Name]; ok {
			tbl.Indexes = tblIdxs
		}
	}
}

// LinkForeignKeys derives back-references on s after the per-table
// outgoing foreign keys have been populated by the driver. Specifically,
// for every entry in each Table.ForeignKeys it:
//
//   - sets [Column.ForeignKey] on each participating local column
//   - appends the same *[ForeignKey] to the referenced table's
//     [Table.ReferencedBy] slice
//
// Entries on Table.ReferencedBy are ordered first by the referencing
// table's name and then by the original order they were declared on
// that table, giving a stable result independent of map iteration.
//
// LinkForeignKeys is idempotent: any pre-existing values in Column.ForeignKey
// and Table.ReferencedBy are cleared before re-deriving. It is safe to call
// against a nil or empty Source.
func LinkForeignKeys(s *Source) {
	if s == nil || len(s.Tables) == 0 {
		return
	}

	byName := make(map[string]*Table, len(s.Tables))
	for _, tbl := range s.Tables {
		if tbl == nil {
			continue
		}
		byName[tbl.Name] = tbl
		tbl.ReferencedBy = nil
		for _, col := range tbl.Columns {
			if col != nil {
				col.ForeignKey = nil
			}
		}
	}

	for _, tbl := range s.Tables {
		if tbl == nil {
			continue
		}
		for _, fk := range tbl.ForeignKeys {
			if fk == nil {
				continue
			}

			// Wire the column back-references on the referencing table.
			for _, colName := range fk.Columns {
				if col := tbl.Column(colName); col != nil {
					col.ForeignKey = fk
				}
			}

			// Append to the referenced table's incoming list, if we can
			// resolve it within this Source. Cross-schema or cross-catalog
			// references that don't resolve are silently skipped — the
			// outgoing edge is still on Table.ForeignKeys for consumers
			// that want to render external references.
			if refTbl, ok := byName[fk.RefTable]; ok {
				refTbl.ReferencedBy = append(refTbl.ReferencedBy, fk)
			}
		}
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
