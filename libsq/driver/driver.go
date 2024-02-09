package driver

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver/dialect"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// Provider is a factory that returns Driver instances.
type Provider interface {
	// DriverFor returns a driver instance for the given type.
	DriverFor(typ drivertype.Type) (Driver, error)
}

// Driver is the core interface that must be implemented for each type
// of data source.
type Driver interface {
	// Open returns a Grip instance for src.
	Open(ctx context.Context, src *source.Source) (Grip, error)

	// Ping verifies that the source is reachable, or returns an error if not.
	// The exact behavior of Ping is driver-dependent. Even if Ping does not
	// return an error, the source may still be bad for other reasons.
	Ping(ctx context.Context, src *source.Source) error

	// DriverMetadata returns driver metadata.
	DriverMetadata() Metadata

	// ValidateSource verifies that the source is valid for this driver. It
	// may transform the source into a canonical form, which is returned in
	// the return value (the original source is not changed). An error
	// is returned if the source is invalid.
	ValidateSource(src *source.Source) (*source.Source, error)
}

// SQLDriver is implemented by Driver instances for SQL databases.
type SQLDriver interface {
	Driver

	// Dialect returns the SQL dialect.
	Dialect() dialect.Dialect

	// ConnParams returns the db parameters available for use in a connection
	// string. The key is the parameter name (e.g. "sslmode"), and the value
	// can be either the set of allowed values, sample values, or nil.
	// These values are used for shell completion and the like. The returned
	// map does not have to be exhaustive, and can be nil.
	ConnParams() map[string][]string

	// ErrWrapFunc returns a func that wraps the driver's errors.
	ErrWrapFunc() func(error) error

	// Renderer returns the SQL renderer for this driver.
	Renderer() *render.Renderer

	// CurrentSchema returns the current schema name.
	CurrentSchema(ctx context.Context, db sqlz.DB) (string, error)

	// ListSchemas lists the names of the schemas on db.
	ListSchemas(ctx context.Context, db sqlz.DB) ([]string, error)

	// ListSchemaMetadata returns the metadata for the schemas on db.
	ListSchemaMetadata(ctx context.Context, db sqlz.DB) ([]*metadata.Schema, error)

	// CurrentCatalog returns the current catalog name. An error is
	// returned if the driver doesn't support catalogs.
	CurrentCatalog(ctx context.Context, db sqlz.DB) (string, error)

	// ListCatalogs lists the available catalog names on db. The first
	// returned element is the current catalog, and the remaining
	// catalogs are sorted alphabetically. An error is returned
	// if the driver doesn't support catalogs.
	ListCatalogs(ctx context.Context, db sqlz.DB) ([]string, error)

	// TableColumnTypes returns the column type info from
	// the SQL driver. If len(colNames) is 0, info is returned
	// for all columns in the table.
	TableColumnTypes(ctx context.Context, db sqlz.DB, tblName string, colNames []string) ([]*sql.ColumnType, error)

	// RecordMeta returns the result metadata (the metadata for
	// each col) from colTypes. RecordMeta is preferred over
	// sql.Rows.ColumnTypes because of the inconsistent behavior
	// of various SQL driver implementations wrt reporting
	// "nullable" information and other quirks. The returned
	// metadata may differ from the original metadata returned
	// by rows.ColumnTypes.
	//
	// The caller should typically should invoke rows.Next before
	// this method is invoked, as some implementations do not return
	// complete column type info until after the first call to rows.Next.
	//
	// RecordMeta also returns a NewRecordFunc which can be
	// applied to the scan row from sql.Rows.
	RecordMeta(ctx context.Context, colTypes []*sql.ColumnType) (record.Meta, NewRecordFunc, error)

	// PrepareInsertStmt prepares a statement for inserting
	// values to destColNames in destTbl. numRows specifies
	// how many rows of values are inserted by each execution of
	// the insert statement (1 row being the prototypical usage).
	// It is the caller's responsibility to close the execer.
	//
	// Note that db must guarantee a single connection: that is, db
	// must be a sql.Conn or sql.Tx.
	PrepareInsertStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string,
		numRows int) (*StmtExecer, error)

	// PrepareUpdateStmt prepares a statement for updating destColNames in
	// destTbl, using the supplied where clause (which may be empty).
	// The where arg should use question mark "?" as the placeholder: it will
	// be translated to the appropriate driver-specific placeholder. For example,
	// the where arg could be:
	//
	//   "actor_id = ? AND first_name = ?".
	//
	// Use the returned StmtExecer per its documentation. It is the caller's
	// responsibility to close the execer.
	//
	// Note that db must guarantee a single connection: that is, db
	// must be a sql.Conn or sql.Tx.
	PrepareUpdateStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string,
		where string) (*StmtExecer, error)

	// CreateTable creates the table defined by tblDef. Some implementations
	// may not honor every field of tblDef, e.g. an impl might not
	// build the foreign key constraints. At a minimum the implementation
	// must honor the table name and column names and kinds from tblDef.
	CreateTable(ctx context.Context, db sqlz.DB, tblDef *schema.Table) error

	// CreateSchema creates a new schema in db. Note that db's current
	// connection schema is not changed.
	CreateSchema(ctx context.Context, db sqlz.DB, schemaName string) error

	// DropSchema drops the named schema in db.
	DropSchema(ctx context.Context, db sqlz.DB, schemaName string) error

	// CatalogExists returns true if db can reference the named catalog. If
	// catalog is empty string, false is returned.
	CatalogExists(ctx context.Context, db sqlz.DB, catalog string) (bool, error)

	// SchemaExists returns true if db can reference the named schema. If
	// schma is empty string, false is returned.
	SchemaExists(ctx context.Context, db sqlz.DB, schma string) (bool, error)

	// Truncate truncates tbl in src. If arg reset is true, the
	// identity counter for tbl should be reset, if supported
	// by the driver. Some DB impls may reset the identity
	// counter regardless of the val of reset.
	Truncate(ctx context.Context, src *source.Source, tbl string, reset bool) (affected int64, err error)

	// TableExists returns true if there's an existing table tbl in db.
	TableExists(ctx context.Context, db sqlz.DB, tbl string) (bool, error)

	// ListTableNames lists the tables of schma in db. The "tables" and "views"
	// args filter TABLE and VIEW types, respectively. If both are false, an empty
	// slice is returned. If schma is empty, the current schema is used.
	ListTableNames(ctx context.Context, db sqlz.DB, schma string, tables, views bool) ([]string, error)

	// CopyTable copies fromTable into a new table toTable.
	// If copyData is true, fromTable's data is also copied.
	// Constraints (keys, defaults etc.) may not be copied. The
	// number of copied rows is returned in copied.
	CopyTable(ctx context.Context, db sqlz.DB, fromTable, toTable tablefq.T, copyData bool) (copied int64, err error)

	// DropTable drops tbl from db. If ifExists is true, an "IF EXISTS"
	// or equivalent clause is added, if supported.
	DropTable(ctx context.Context, db sqlz.DB, tbl tablefq.T, ifExists bool) error

	// AlterTableRename renames a table.
	AlterTableRename(ctx context.Context, db sqlz.DB, tbl, newName string) error

	// AlterTableAddColumn adds column col to tbl. The column is appended
	// to the list of columns (that is, the column position cannot be
	// specified).
	AlterTableAddColumn(ctx context.Context, db sqlz.DB, tbl, col string, knd kind.Kind) error

	// AlterTableRenameColumn renames a column.
	AlterTableRenameColumn(ctx context.Context, db sqlz.DB, tbl, col, newName string) error

	// DBProperties returns a map of key-value database properties. The value
	// is often a scalar such as an int, string, or bool, but can be a nested
	// map or array.
	DBProperties(ctx context.Context, db sqlz.DB) (map[string]any, error)
}

// Metadata holds driver metadata.
//
// TODO: Can driver.Metadata and dialect.Dialect be merged?
type Metadata struct {
	// Type is the driver type, e.g. "mysql" or "csv", etc.
	Type drivertype.Type `json:"type" yaml:"type"`

	// Description is typically the long name of the driver, e.g.
	// "MySQL" or "Microsoft Excel XLSX".
	Description string `json:"description" yaml:"description"`

	// Doc is optional documentation, typically a URL.
	Doc string `json:"doc,omitempty" yaml:"doc,omitempty"`

	// UserDefined is true if this driver is the product of a
	// user driver definition, and false if built-in.
	UserDefined bool `json:"user_defined" yaml:"user_defined"`

	// IsSQL is true if this driver is a SQL driver.
	IsSQL bool `json:"is_sql" yaml:"is_sql"`

	// Monotable is true if this is a non-SQL document type that
	// effectively has a single table, such as CSV.
	Monotable bool `json:"monotable" yaml:"monotable"`

	// DefaultPort is the default port that a driver connects on. A
	// value <= 0 indicates not applicable.
	DefaultPort int `json:"default_port" yaml:"default_port"`
}

// OpeningPing is a standardized mechanism to ping db using
// driver.OptConnOpenTimeout. This should be invoked by each SQL
// driver impl in its Open method. If the ping fails, db is closed.
// In practice, this function probably isn't needed. Maybe ditch it.
func OpeningPing(ctx context.Context, src *source.Source, db *sql.DB) error {
	bar := progress.FromContext(ctx).NewWaiter("Ping "+src.Handle, true)
	defer bar.Stop()

	o := options.Merge(options.FromContext(ctx), src.Options)
	timeout := OptConnOpenTimeout.Get(o)
	ctx, cancelFn := context.WithTimeout(ctx, timeout)
	defer cancelFn()

	if err := db.PingContext(ctx); err != nil {
		err = errz.Wrapf(err, "open ping %s", src.Handle)
		log := lg.FromContext(ctx)
		log.Error("Failed opening ping", lga.Src, src, lga.Err, err)
		lg.WarnIfCloseError(log, lgm.CloseDB, db)
		return err
	}

	return nil
}

// EmptyDataError indicates that there's no data, e.g. an empty document.
// This is subtly different to NotExistError, which would indicate that
// the document doesn't exist.
type EmptyDataError string

// Error satisfies the stdlib error interface.
func (e EmptyDataError) Error() string { return string(e) }

// NewEmptyDataError returns a EmptyDataError.
func NewEmptyDataError(format string, args ...any) error {
	return errz.Err(EmptyDataError(fmt.Sprintf(format, args...)), errz.Skip(1))
}

// NotExistError indicates that a DB object, such
// as a table, does not exist.
type NotExistError struct {
	error
}

// Unwrap satisfies the stdlib errors.Unwrap function.
func (e *NotExistError) Unwrap() error { return e.error }

// NewNotExistError returns a NotExistError, or nil.
func NewNotExistError(err error) error {
	if err == nil {
		return nil
	}
	return errz.Err(&NotExistError{error: err})
}
