// Package metadata contains types that model source metadata.
package metadata

import (
	"encoding/json"
	"log/slog"
	"maps"
	"sort"

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
	// A nil value means the size is not reported by the driver, distinct
	// from a non-nil pointer to 0 (a genuinely zero-sized source).
	// Mirrors [Table.Size].
	Size *int64 `json:"size,omitempty" yaml:"size,omitempty"`

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

// Clone returns a deep copy of s. If s is nil, nil is returned.
//
// Clone re-runs [LinkForeignKeys] with a nil logger on the result so
// the cloned tables' FK.Incoming slices point at the cloned outgoing
// FK objects rather than at the originals. Passing nil suppresses
// warnings about unresolved FK targets — callers that construct a
// *Source programmatically (i.e. not via a driver's
// getSourceMetadata) and want to surface those should call
// [LinkForeignKeys] themselves with a real logger before cloning, or
// after Clone to revalidate.
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

		// Per-table Clone() copies FK.Outgoing as independent values;
		// re-derive FK.Incoming so the clone's incoming list shares
		// identity with the cloned outgoing FKs rather than pointing
		// at the originals. A nil logger is fine here — the original
		// source was already validated when it was first linked.
		LinkForeignKeys(nil, s2)
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

	// FK groups the foreign-key relationships for this table: outgoing
	// constraints declared on it, and incoming constraints declared on
	// other tables that reference it. Nil for sources that don't
	// support foreign keys or tables with no FK relationships in
	// either direction.
	FK *FKGroup `json:"fk,omitempty" yaml:"fk,omitempty"`

	// UniqueConstraints are the unique-constraint declarations on this
	// table (UNIQUE in CREATE TABLE, or ALTER TABLE ADD CONSTRAINT ...
	// UNIQUE). Primary keys are *not* repeated here — they are reported
	// via [Column.PrimaryKey] and also appear in [Table.Indexes] as
	// an entry with [Index.Primary]=true. Composite unique constraints
	// are returned with their columns in declaration order.
	UniqueConstraints []*UniqueConstraint `json:"unique_constraints,omitempty" yaml:"unique_constraints,omitempty"`

	// Indexes are the physical indexes that back this table. Use
	// [Index.Unique] / [Index.Primary] to distinguish kinds. Non-unique
	// secondary indexes (e.g. created via CREATE INDEX for performance)
	// always appear here. Driver coverage of implicit constraint-backing
	// indexes varies:
	//
	//   - postgres, mysql, sqlserver, sqlite3: the implicit unique index
	//     that backs each primary key and unique constraint is reported.
	//   - oracle: only user-named indexes are reported (auto-named
	//     SYS_C… indexes from inline PRIMARY KEY / UNIQUE declarations
	//     are filtered out). PK membership is still available via
	//     [Column.PrimaryKey].
	//   - duckdb: only indexes from explicit CREATE INDEX statements are
	//     reported. PK / UNIQUE-backing indexes are not surfaced by the
	//     duckdb_indexes() catalog; PK / unique info is still available
	//     via [Column.PrimaryKey] and [Table.UniqueConstraints].
	Indexes []*Index `json:"indexes,omitempty" yaml:"indexes,omitempty"`
}

// String returns a log/debug friendly representation.
func (t *Table) String() string {
	bytes, _ := json.Marshal(t)
	return string(bytes)
}

// Clone returns a deep copy of t. If t is nil, nil is returned.
//
// If t.FK.Incoming was previously populated by [LinkForeignKeys],
// the clone's FK.Incoming will be nil — see [FKGroup.Clone] for the
// rationale. Use [Source.Clone] to clone a table inside a source and
// have its incoming back-references re-derived automatically.
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

	if t.FK != nil {
		c.FK = t.FK.Clone()
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
//
// Column does not carry a foreign-key back-reference. The canonical
// outgoing-FK list lives on [Table.FK].Outgoing; consumers that want
// a per-column view can build one in two lines:
//
//	byCol := map[string][]*ForeignKey{}
//	if tbl.FK != nil {
//	    for _, fk := range tbl.FK.Outgoing {
//	        for _, c := range fk.Columns { byCol[c] = append(byCol[c], fk) }
//	    }
//	}
//
// Composite FKs stay grouped on the table where they belong (each
// member column lists the same composite ForeignKey pointer). A
// column that participates in multiple outgoing FK constraints —
// unusual but valid SQL — keeps every entry.
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

// FKGroup groups the per-table foreign-key relationships under a
// single parent so the JSON / YAML shape stays cohesive — both
// directions of the same conceptual cluster live together under
// `fk.outgoing` / `fk.incoming` rather than as flat siblings.
// Drivers should construct one via [NewFKGroup] (or assign nil) so
// the wrapper is omitted entirely when both slices are empty.
type FKGroup struct {
	// Outgoing are the foreign-key constraints declared on the owning
	// table (constraints whose referencing side is this table).
	Outgoing []*ForeignKey `json:"outgoing,omitempty" yaml:"outgoing,omitempty"`

	// Incoming are the foreign-key constraints declared on other
	// tables whose referenced side is the owning table. Derived
	// cross-table by [LinkForeignKeys] for source-level inspect, or
	// loaded directly by per-table inspect.
	Incoming []*ForeignKey `json:"incoming,omitempty" yaml:"incoming,omitempty"`
}

// NewFKGroup returns a *FKGroup carrying the given outgoing and
// incoming slices. If both are empty it returns nil so the caller
// can assign directly to [Table.FK] without producing an empty
// `fk: {}` object in JSON / YAML output.
func NewFKGroup(outgoing, incoming []*ForeignKey) *FKGroup {
	if len(outgoing) == 0 && len(incoming) == 0 {
		return nil
	}
	return &FKGroup{Outgoing: outgoing, Incoming: incoming}
}

// Clone returns a deep copy of g. If g is nil, nil is returned.
//
// Only Outgoing is deep-copied; Incoming is left nil because it is a
// derived back-reference whose pointer identity must match the
// Outgoing entries on the (cloned) source's other tables — a
// requirement [Source.Clone] satisfies by re-running [LinkForeignKeys]
// over the cloned tables.
//
// Cloning an *FKGroup (or a parent *Table via [Table.Clone]) outside
// of a [Source.Clone] does NOT reconstruct Incoming, and cannot —
// [LinkForeignKeys] needs the whole source to know which other tables
// hold the corresponding outgoing FKs. Standalone clones therefore
// have Incoming == nil; if you need the incoming view, clone the
// owning *Source and read FK.Incoming from the cloned table.
func (g *FKGroup) Clone() *FKGroup {
	if g == nil {
		return nil
	}
	c := &FKGroup{}
	if g.Outgoing != nil {
		c.Outgoing = make([]*ForeignKey, len(g.Outgoing))
		for i := range g.Outgoing {
			c.Outgoing[i] = g.Outgoing[i].Clone()
		}
	}
	return c
}

// ForeignKey models a single foreign-key constraint between two tables.
// Constraints are populated by the driver from the perspective of the
// referencing table (as [Table.FK].Outgoing); the corresponding
// incoming entries on the referenced table's [Table.FK].Incoming slice
// are derived by [LinkForeignKeys] once all tables have been loaded.
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

	// Columns holds one entry per index key position, in key order. A
	// plain-column key is its column name; a functional or expression
	// key (e.g. lower(b)) is represented by the empty string "". Thus
	// len(Columns) equals the index's true key count, and the position
	// of an expression key is preserved. An index whose key positions
	// are all expressions is omitted from reported metadata entirely.
	// INCLUDE / covering columns are not key positions and do not
	// appear here. See [AllExpressionKeys].
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

// AllExpressionKeys reports whether every entry in an index's key
// columns is the empty-string expression sentinel — that is, the index
// has no plain-column key positions and should be omitted from reported
// metadata. It returns false for an empty slice (a degenerate keyless
// index is left for the caller to handle). See [Index.Columns].
func AllExpressionKeys(cols []string) bool {
	if len(cols) == 0 {
		return false
	}
	for _, c := range cols {
		if c != "" {
			return false
		}
	}
	return true
}

// AssignForeignKeys groups fks by their referencing-table name (the
// [ForeignKey.Table] field) and assigns each group to the matching
// entry's [Table.FK].Outgoing slice, replacing any previously-assigned
// outgoing slice on that table. Tables with no matching FKs are left
// untouched (their existing [Table.FK] is not modified).
//
// This helper exists so that driver implementations that fetch all
// foreign keys in a single source-wide query (postgres, mysql,
// sqlserver, oracle, duckdb) don't each have to repeat the same
// grouping loop.
//
// Any FK whose [ForeignKey.Table] does not match a table in tables
// is dropped. When log is non-nil, the names of the dropped FKs are
// reported at warn level so a driver case-folding mismatch or a
// table that was filtered out after the bulk loader ran doesn't
// silently lose FK data.
func AssignForeignKeys(log *slog.Logger, tables []*Table, fks []*ForeignKey) {
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
			if tbl.FK == nil {
				tbl.FK = &FKGroup{}
			}
			tbl.FK.Outgoing = tblFKs
			delete(byTable, tbl.Name)
		}
	}

	warnOrphans(log, "foreign key", byTable)
}

// AssignUniqueConstraints groups ucs by their owning-table name and
// assigns each group to the matching entry in tables. The analog of
// [AssignForeignKeys] for unique-constraint slices, including the
// warn-on-orphan behavior.
func AssignUniqueConstraints(log *slog.Logger, tables []*Table, ucs []*UniqueConstraint) {
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
			delete(byTable, tbl.Name)
		}
	}

	warnOrphans(log, "unique constraint", byTable)
}

// AssignIndexes groups idxs by their owning-table name and assigns
// each group to the matching entry in tables. The analog of
// [AssignForeignKeys] for index slices, including the warn-on-orphan
// behavior.
func AssignIndexes(log *slog.Logger, tables []*Table, idxs []*Index) {
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
			delete(byTable, tbl.Name)
		}
	}

	warnOrphans(log, "index", byTable)
}

// warnOrphans emits one log.Warn per owning-table name in orphans;
// callers pass label ("foreign key", "unique constraint", "index")
// to identify the kind via a structured attribute. orphans is the
// residual map left after the bulk loader's rows have been
// distributed — a non-empty map means the driver returned rows
// referencing a table that isn't in the source.
//
// In production this can fire on benign races (a table dropped
// between enumeration and per-table load — see the warn-suppress
// paths in the postgres and oracle source-level inspect) as well as
// on real driver bugs (case-folding mismatches, typos in bulk SQL).
// Both are worth surfacing; the structured attributes let operators
// disambiguate.
//
// The "dropped" attribute counts the number of metadata objects
// (composite-FK / UC / Index records, where one record may span
// multiple raw SQL rows) — not raw SQL rows.
//
// Owning-table names are sorted before emission so the log entry
// order is deterministic across runs.
func warnOrphans[T any](log *slog.Logger, label string, orphans map[string][]T) {
	if log == nil || len(orphans) == 0 {
		return
	}
	names := make([]string, 0, len(orphans))
	for name := range orphans {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		log.Warn("metadata: dropped rows for unknown owning table",
			slog.String("kind", label),
			slog.String("table", name),
			slog.Int("dropped", len(orphans[name])),
		)
	}
}

// LinkForeignKeys derives [FKGroup.Incoming] and normalizes FK
// qualifiers after the per-table outgoing foreign keys
// ([FKGroup.Outgoing]) have been populated by the driver. Specifically:
//
//   - For every outgoing FK whose referenced table is in s, append the
//     same *[ForeignKey] pointer to that table's FK.Incoming slice.
//     This is the only step that genuinely requires a cross-table
//     view — drivers see one table at a time and can't know which
//     other tables point at it.
//   - Normalize [ForeignKey.RefCatalog] and [ForeignKey.RefSchema]:
//     when they equal s.Catalog / s.Schema the fields are cleared so
//     the JSON / YAML output omits them and the "same-schema
//     reference" invariant the resolver relies on holds (a non-empty
//     RefSchema after normalization marks the reference as pointing
//     outside this Source).
//
// Entries on FK.Incoming follow the iteration order of s.Tables, then
// the order of each table's FK.Outgoing slice. Drivers that load
// tables in name order (most do) therefore produce a deterministic
// shape; consumers that need a strict sort should sort the slice
// themselves.
//
// LinkForeignKeys is idempotent: any pre-existing values in
// FK.Incoming are cleared before re-deriving. It is safe to call
// against a nil or empty Source.
//
// As a final cleanup, any [Table.FK] whose Outgoing and Incoming
// slices are both empty is set back to nil so the wrapper object is
// omitted from JSON / YAML output entirely.
//
// When log is non-nil, an FK that appears to be in-source (empty
// RefCatalog / RefSchema after normalization) but whose RefTable
// isn't present in s is reported at warn level. This surfaces driver
// bugs (typos in bulk FK SQL, case-folding mismatches) and
// transiently dropped tables that would otherwise leave the
// inspect output silently inconsistent.
func LinkForeignKeys(log *slog.Logger, s *Source) {
	if s == nil || len(s.Tables) == 0 {
		return
	}

	byName := make(map[string]*Table, len(s.Tables))
	for _, tbl := range s.Tables {
		if tbl == nil {
			continue
		}
		byName[tbl.Name] = tbl
		if tbl.FK != nil {
			tbl.FK.Incoming = nil
		}
	}

	for _, tbl := range s.Tables {
		if tbl == nil || tbl.FK == nil {
			continue
		}
		for _, fk := range tbl.FK.Outgoing {
			if fk == nil {
				continue
			}

			// Normalize same-source qualifiers so they're omitted on
			// output and so the cross-schema check below is reliable.
			if fk.RefCatalog == s.Catalog {
				fk.RefCatalog = ""
			}
			if fk.RefSchema == s.Schema {
				fk.RefSchema = ""
			}

			// Only link to a local table when the reference is in
			// this Source's catalog+schema. A non-empty RefCatalog or
			// RefSchema after normalization means the target lives
			// elsewhere; treat it as external so we don't incorrectly
			// match a same-named table in the local schema.
			if fk.RefCatalog != "" || fk.RefSchema != "" {
				continue
			}
			if refTbl, ok := byName[fk.RefTable]; ok {
				if refTbl.FK == nil {
					refTbl.FK = &FKGroup{}
				}
				refTbl.FK.Incoming = append(refTbl.FK.Incoming, fk)
			} else if log != nil {
				// Benign in some production scenarios: a table dropped
				// between enumeration and the per-table errgroup load,
				// or (oracle) a parent table whose per-table fetch was
				// warn-suppressed. Also surfaces real driver bugs
				// (case-folding mismatches, typos in bulk FK SQL).
				log.Warn("metadata: outgoing FK references unknown table",
					slog.String("constraint", fk.Name),
					slog.String("table", fk.Table),
					slog.String("ref_table", fk.RefTable),
				)
			}
		}
	}

	// Drop any empty wrapper so JSON / YAML omit `fk: {}`.
	for _, tbl := range s.Tables {
		if tbl != nil && tbl.FK != nil && len(tbl.FK.Outgoing) == 0 && len(tbl.FK.Incoming) == 0 {
			tbl.FK = nil
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
