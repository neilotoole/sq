package driver

import (
	"context"
	"database/sql"
	"strings"
	"sync"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/libsq/core/cleanup"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/core/errz"

	"github.com/neilotoole/sq/libsq/ast/sqlbuilder"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/source"
)

// Provider is a factory that returns Driver instances.
type Provider interface {
	// DriverFor returns a driver instance for the given type.
	DriverFor(typ source.Type) (Driver, error)
}

// DatabaseOpener opens a Database.
type DatabaseOpener interface {
	// Open returns a Database instance for src.
	Open(ctx context.Context, src *source.Source) (Database, error)
}

// JoinDatabaseOpener can open a join database.
type JoinDatabaseOpener interface {
	// OpenJoin opens an appropriate Database for use as
	// a work DB for joining across sources.
	OpenJoin(ctx context.Context, src1, src2 *source.Source, srcN ...*source.Source) (Database, error)
}

// ScratchDatabaseOpener opens a scratch database. A scratch database is
// typically a short-lived database used as a target for loading
// non-SQL data (such as CSV).
type ScratchDatabaseOpener interface {
	// OpenScratch returns a database for scratch use.
	OpenScratch(ctx context.Context, name string) (Database, error)
}

// Driver is the interface that must be implemented for a data
// source type.
type Driver interface {
	DatabaseOpener

	// DriverMetadata returns driver metadata.
	DriverMetadata() Metadata

	// ValidateSource verifies that the source is valid for this driver. It
	// may transform the source into a canonical form, which is returned in
	// the "src" return value (the original source is not changed). An error
	// is returned if the source is invalid.
	ValidateSource(source *source.Source) (src *source.Source, err error)

	// Ping verifies that the source is reachable, or returns an error if not.
	// The exact behavior of Ping() is driver-dependent.
	Ping(ctx context.Context, src *source.Source) error

	// Truncate truncates tbl in src. If arg reset is true, the
	// identity counter for tbl should be reset, if supported
	// by the driver. Some DB impls may reset the identity
	// counter regardless of the val of reset.
	Truncate(ctx context.Context, src *source.Source, tbl string, reset bool) (affected int64, err error)
}

// SQLDriver is implemented by Driver instances for SQL databases.
type SQLDriver interface {
	Driver

	// Dialect returns the SQL dialect.
	Dialect() Dialect

	// SQLBuilder returns the SQL builder for this driver.
	SQLBuilder() (sqlbuilder.FragmentBuilder, sqlbuilder.QueryBuilder)

	// CurrentSchema returns the current schema name.
	CurrentSchema(ctx context.Context, db sqlz.DB) (string, error)

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
	RecordMeta(colTypes []*sql.ColumnType) (sqlz.RecordMeta, NewRecordFunc, error)

	// PrepareInsertStmt prepares a statement for inserting
	// values to destColNames in destTbl. numRows specifies
	// how many rows of values are inserted by each execution of
	// the insert statement (1 row being the prototypical usage).
	// It is the caller's responsibility to close the execer.
	//
	// Note that db must guarantee a single connection: that is, db
	// must be a sql.Conn or sql.Tx.
	PrepareInsertStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string, numRows int) (*StmtExecer,
		error)

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
	// may not honor all of the fields of tblDef, e.g. an impl might not
	// build the foreign key constraints. At a minimum the implementation
	// must honor the table name and column names and kinds from tblDef.
	CreateTable(ctx context.Context, db sqlz.DB, tblDef *sqlmodel.TableDef) error

	// TableExists returns true if there's an existing table tbl in db.
	TableExists(ctx context.Context, db sqlz.DB, tbl string) (bool, error)

	// CopyTable copies fromTable into a new table toTable.
	// If copyData is true, fromTable's data is also copied.
	// Constraints (keys, defaults etc.) may not be copied. The
	// number of copied rows is returned in copied.
	CopyTable(ctx context.Context, db sqlz.DB, fromTable, toTable string, copyData bool) (copied int64, err error)

	// DropTable drops tbl from db. If ifExists is true, an "IF EXISTS"
	// or equivalent clause is added, if supported.
	DropTable(ctx context.Context, db sqlz.DB, tbl string, ifExists bool) error

	// AlterTableRename renames a table.
	AlterTableRename(ctx context.Context, db sqlz.DB, tbl, newName string) error

	// AlterTableAddColumn adds column col to tbl. The column is appended
	// to the list of columns (that is, the column position cannot be
	// specified).
	AlterTableAddColumn(ctx context.Context, db sqlz.DB, tbl, col string, knd kind.Kind) error

	// AlterTableRenameColumn renames a column.
	AlterTableRenameColumn(ctx context.Context, db sqlz.DB, tbl, col, newName string) error
}

// Database models a database handle. It is conceptually equivalent to
// stdlib sql.DB, and in fact encapsulates a sql.DB instance. The
// realized sql.DB instance can be accessed via the DB method.
type Database interface {
	// DB returns the sql.DB object for this Database.
	DB() *sql.DB

	// SQLDriver returns the underlying database driver. This
	// may be different from the type reported by the
	// Database source.
	SQLDriver() SQLDriver

	// Source returns the data source for which this connection was opened.
	Source() *source.Source

	// SourceMetadata returns metadata about the data source.
	SourceMetadata(ctx context.Context) (*source.Metadata, error)

	// TableMetadata returns metadata for the specified table in the data source.
	TableMetadata(ctx context.Context, tblName string) (*source.TableMetadata, error)

	// Close is invoked to close and release any underlying resources.
	Close() error
}

// Metadata holds driver metadata.
type Metadata struct {
	// Type is the driver source type, e.g. "mysql" or "csv", etc.
	Type source.Type `json:"type"`

	// Description is typically the long name of the driver, e.g.
	// "MySQL" or "Microsoft Excel XLSX".
	Description string `json:"description"`

	// Doc is optional documentation, typically a URL.
	Doc string `json:"doc,omitempty"`

	// UserDefined is true if this driver is the product of a
	// user driver definition, and false if built-in.
	UserDefined bool `json:"user_defined"`

	// IsSQL is true if this driver is a SQL driver.
	IsSQL bool `json:"is_sql"`

	// Monotable is true if this is a non-SQL document type that
	// effectively has a single table, such as CSV.
	Monotable bool `json:"monotable"`
}

// Dialect holds driver-specific SQL dialect values.
type Dialect struct {
	// Type is the dialect's driver source type.
	Type source.Type `json:"type"`

	// Placeholders returns a string a SQL placeholders string.
	// For example "(?, ?, ?)" or "($1, $2, $3), ($4, $5, $6)".
	Placeholders func(numCols, numRows int) string

	// Quote is the quote rune, typically the double quote rune.
	Quote rune `json:"quote"`

	// IntBool is true if BOOLEAN is handled as an INT by the DB driver.
	IntBool bool `json:"int_bool"`

	// MaxBatchValues is the maximum number of values in a batch insert.
	MaxBatchValues int
}

// Enquote returns s surrounded by d.Quote.
func (d Dialect) Enquote(s string) string {
	return stringz.Surround(s, string(d.Quote))
}

func (d Dialect) String() string {
	return d.Type.String()
}

// Databases provides a mechanism for getting Database instances.
// Note that at this time instances returned by Open are cached
// and then closed by Close. This may be a bad approach.
type Databases struct {
	log          lg.Log
	drvrs        Provider
	mu           sync.Mutex
	scratchSrcFn ScratchSrcFunc
	dbases       map[string]Database
	clnup        *cleanup.Cleanup
}

// NewDatabases returns a Databases instances.
func NewDatabases(log lg.Log, drvrs Provider, scratchSrcFn ScratchSrcFunc) *Databases {
	return &Databases{
		log:          log,
		drvrs:        drvrs,
		mu:           sync.Mutex{},
		scratchSrcFn: scratchSrcFn,
		dbases:       map[string]Database{},
		clnup:        cleanup.New(),
	}
}

// Open returns an opened Database for src. The returned Database
// may be cached and returned on future invocations for the
// same handle. Thus, the caller should typically not close
// the Database: it will be closed via d.Close.
//
// NOTE: This entire logic re caching/not-closing is a bit sketchy,
// and needs to be revisited.
//
// Open implements DatabaseOpener.
func (d *Databases) Open(ctx context.Context, src *source.Source) (Database, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbase, ok := d.dbases[src.Handle]
	if ok {
		return dbase, nil
	}

	drvr, err := d.drvrs.DriverFor(src.Type)
	if err != nil {
		return nil, err
	}

	dbase, err = drvr.Open(ctx, src)
	if err != nil {
		return nil, err
	}

	d.clnup.AddC(dbase)

	d.dbases[src.Handle] = dbase
	return dbase, nil
}

// OpenScratch returns a scratch database instance. It is not
// necessary for the caller to close the returned Database as
// its Close method will be invoked by d.Close.
//
// OpenScratch implements ScratchDatabaseOpener.
func (d *Databases) OpenScratch(ctx context.Context, name string) (Database, error) {
	scratchSrc, cleanFn, err := d.scratchSrcFn(d.log, name)
	if err != nil {
		// if err is non-nil, cleanup is guaranteed to be nil
		return nil, err
	}
	d.log.Debugf("Will open Scratch src %s: %s", scratchSrc.Handle, scratchSrc.RedactedLocation())

	drvr, err := d.drvrs.DriverFor(scratchSrc.Type)
	if err != nil {
		d.log.WarnIfFuncError(cleanFn)
		return nil, err
	}

	sqlDrvr, ok := drvr.(SQLDriver)
	if !ok {
		d.log.WarnIfFuncError(cleanFn)
		return nil, errz.Errorf("driver for scratch source %s is not a SQLDriver but is %T", scratchSrc.Handle, drvr)
	}

	var backingDB Database
	backingDB, err = sqlDrvr.Open(ctx, scratchSrc)
	if err != nil {
		d.log.WarnIfFuncError(cleanFn)
		return nil, err
	}

	d.clnup.AddE(cleanFn)
	return backingDB, nil
}

// OpenJoin opens an appropriate database for use as
// as a work DB for joining across sources.
//
// Note: There is much work to be done on this method. At this time, only
// two sources are supported. Ultimately OpenJoin should be able to
// inspect the join srcs and use heuristics to determine the best
// location for the join to occur (to minimize copying of data for
// the join etc.). Currently the implementation simply delegates
// to OpenScratch.
//
// OpenJoin implements JoinDatabaseOpener.
func (d *Databases) OpenJoin(ctx context.Context, src1, src2 *source.Source, srcN ...*source.Source) (Database, error) {
	if len(srcN) > 0 {
		return nil, errz.Errorf("Currently only two-source join is supported")
	}

	names := []string{src1.Handle, src2.Handle}
	for _, src := range srcN {
		names = append(names, src.Handle)
	}

	d.log.Debugf("OpenJoin: [%s]", strings.Join(names, ","))
	return d.OpenScratch(ctx, "joindb__"+strings.Join(names, "_"))
}

// Close closes d, invoking Close on any instances opened via d.Open.
func (d *Databases) Close() error {
	d.log.Debugf("Closing %d databases(s)", d.clnup.Len())
	return d.clnup.Run()
}

// Tuning holds tuning params. Ultimately these params
// could come from user config or be dynamically calculated/adjusted?
var Tuning = struct {
	// ErrgroupNumG is the numG value for errgroup.WithContextN.
	ErrgroupNumG int

	// ErrgroupQSize is the qSize value for errgroup.WithContextN.
	ErrgroupQSize int

	// RecordChSize is the size of the buffer chan for record
	// insertion/writing.
	RecordChSize int

	// SampleSize is the number of samples that a detector should
	// take to determine type.
	SampleSize int
}{
	ErrgroupNumG:  16,
	ErrgroupQSize: 16,
	RecordChSize:  1024,
	SampleSize:    1024,
}

// requireSingleConn returns nil if db is a type that guarantees a
// single database connection. That is, requireSingleConn returns an
// error if db does not have type *sql.Conn or *sql.Tx.
func requireSingleConn(db sqlz.DB) error {
	switch db.(type) {
	case *sql.Conn, *sql.Tx:
	default:
		return errz.Errorf("db must be guaranteed single-connection (sql.Conn or sql.Tx) but was %T", db)
	}

	return nil
}
