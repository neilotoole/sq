package driver

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver/dialect"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// ConfigureDB configures DB using o. It is no-op if o is nil.
func ConfigureDB(ctx context.Context, db *sql.DB, o options.Options) {
	o2 := options.Effective(o, OptConnMaxOpen, OptConnMaxIdle, OptConnMaxIdleTime, OptConnMaxLifetime)

	lg.FromContext(ctx).Debug("Setting config on DB conn", "config", o2)

	db.SetMaxOpenConns(OptConnMaxOpen.Get(o2))
	db.SetMaxIdleConns(OptConnMaxIdle.Get(o2))
	db.SetConnMaxIdleTime(OptConnMaxIdleTime.Get(o2))
	db.SetConnMaxLifetime(OptConnMaxLifetime.Get(o2))
}

var (
	// OptConnMaxOpen controls sql.DB.SetMaxOpenConn.
	OptConnMaxOpen = options.NewInt(
		"conn.max-open",
		"",
		0,
		0,
		"Max open connections to DB",
		`Maximum number of open connections to the database.
A value of zero indicates no limit.`,
		options.TagSource,
		options.TagSQL,
	)

	// OptConnMaxIdle controls sql.DB.SetMaxIdleConns.
	OptConnMaxIdle = options.NewInt(
		"conn.max-idle",
		"",
		0,
		2,
		"Max connections in idle connection pool",
		`Set the maximum number of connections in the idle connection pool.
If conn.max-open is greater than 0 but less than the new conn.max-idle,
then the new conn.max-idle will be reduced to match the conn.max-open limit.
If n <= 0, no idle connections are retained.`,
		options.TagSource,
		options.TagSQL,
	)

	// OptConnMaxIdleTime controls sql.DB.SetConnMaxIdleTime.
	OptConnMaxIdleTime = options.NewDuration(
		"conn.max-idle-time",
		"",
		0,
		time.Second*2,
		"Max connection idle time",
		`Sets the maximum amount of time a connection may be idle.
Expired connections may be closed lazily before reuse. If n <= 0,
connections are not closed due to a connection's idle time.`,
		options.TagSource,
		options.TagSQL,
	)

	// OptConnMaxLifetime controls sql.DB.SetConnMaxLifetime.
	OptConnMaxLifetime = options.NewDuration(
		"conn.max-lifetime",
		"",
		0,
		time.Minute*10,
		"Max connection lifetime",
		`Set the maximum amount of time a connection may be reused.
Expired connections may be closed lazily before reuse.
If n <= 0, connections are not closed due to a connection's age.`,
		options.TagSource,
		options.TagSQL,
	)

	// OptConnOpenTimeout controls connection open timeout.
	OptConnOpenTimeout = options.NewDuration(
		"conn.open-timeout",
		"",
		0,
		time.Second*5,
		"Connection open timeout",
		"Max time to wait before a connection open timeout occurs.",
		options.TagSource,
		options.TagSQL,
	)

	// OptMaxRetryInterval is the maximum interval to wait
	// between retries.
	OptMaxRetryInterval = options.NewDuration(
		"retry.max-interval",
		"",
		0,
		time.Second*3,
		"Max interval between retries",
		`The maximum interval to wait between retries.
If an operation is retryable (for example, if the DB has too many clients),
repeated retry operations back off, typically using a Fibonacci backoff.`,
		options.TagSource,
	)

	// OptTuningErrgroupLimit controls the maximum number of goroutines that can be spawned
	// by an errgroup.
	OptTuningErrgroupLimit = options.NewInt(
		"tuning.errgroup-limit",
		"",
		0,
		16,
		"Max goroutines in any one errgroup",
		`Controls the maximum number of goroutines that can be spawned
by an errgroup. Note that this is the limit for any one errgroup, but not a
ceiling on the total number of goroutines spawned, as some errgroups may
themselves start an errgroup.

This knob is primarily for internal use. Ultimately it should go away
in favor of dynamic errgroup limit setting based on availability
of additional DB conns, etc.`,
		options.TagTuning,
	)

	// OptTuningRecChanSize is the size of the buffer chan for record
	// insertion/writing.
	OptTuningRecChanSize = options.NewInt(
		"tuning.record-buffer",
		"",
		0,
		1024,
		"Size of record buffer",
		`Controls the size of the buffer channel for record insertion/writing.`,
		options.TagTuning,
	)
)

// Provider is a factory that returns Driver instances.
type Provider interface {
	// DriverFor returns a driver instance for the given type.
	DriverFor(typ drivertype.Type) (Driver, error)
}

// PoolOpener opens a Pool.
type PoolOpener interface {
	// Open returns a Pool instance for src.
	Open(ctx context.Context, src *source.Source) (Pool, error)
}

// JoinPoolOpener can open a join database.
type JoinPoolOpener interface {
	// OpenJoin opens an appropriate Pool for use as
	// a work DB for joining across sources.
	OpenJoin(ctx context.Context, srcs ...*source.Source) (Pool, error)
}

// ScratchPoolOpener opens a scratch database pool. A scratch database is
// typically a short-lived database used as a target for loading
// non-SQL data (such as CSV).
type ScratchPoolOpener interface {
	// OpenScratch returns a pool for scratch use.
	OpenScratch(ctx context.Context, name string) (Pool, error)
}

// Driver is the core interface that must be implemented for each type
// of data source.
type Driver interface {
	PoolOpener

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
	//
	// TODO: Maybe move Truncate to SQLDriver?
	Truncate(ctx context.Context, src *source.Source, tbl string, reset bool) (affected int64, err error)
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
	// may not honor every field of tblDef, e.g. an impl might not
	// build the foreign key constraints. At a minimum the implementation
	// must honor the table name and column names and kinds from tblDef.
	CreateTable(ctx context.Context, db sqlz.DB, tblDef *sqlmodel.TableDef) error

	// CreateSchema creates a new schema in db. Note that db's current
	// connection schema is not changed.
	CreateSchema(ctx context.Context, db sqlz.DB, schemaName string) error

	// DropSchema drops the named schema in db.
	DropSchema(ctx context.Context, db sqlz.DB, schemaName string) error

	// TableExists returns true if there's an existing table tbl in db.
	TableExists(ctx context.Context, db sqlz.DB, tbl string) (bool, error)

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

// Pool models a database handle representing a pool of underlying
// connections. It is conceptually equivalent to
// stdlib sql.DB, and in fact encapsulates a sql.DB instance. The
// realized sql.DB instance can be accessed via the DB method.
type Pool interface {
	// DB returns the sql.DB object for this Pool.
	// This operation can take a long time if opening the DB requires
	// an ingest of data.
	// For example, with file-based sources such as XLSX, invoking Open
	// will ultimately read and import all CSV rows from the file.
	// Thus, set a timeout on ctx as appropriate for the source.
	DB(ctx context.Context) (*sql.DB, error)

	// SQLDriver returns the underlying database driver. The type of the SQLDriver
	// may be different from the driver type reported by the Source.
	SQLDriver() SQLDriver

	// Source returns the data source for which this connection was opened.
	Source() *source.Source

	// SourceMetadata returns metadata about the data source.
	// If noSchema is true, schema details are not populated
	// on the returned metadata.Source.
	//
	// TODO: SourceMetadata doesn't really belong on driver.Pool? It
	// should be moved to driver.Driver?
	SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error)

	// TableMetadata returns metadata for the specified table in the data source.
	//
	// TODO: TableMetadata doesn't really belong on driver.Pool? It
	// should be moved to driver.Driver?
	TableMetadata(ctx context.Context, tblName string) (*metadata.Table, error)

	// Close is invoked to close and release any underlying resources.
	Close() error
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

var (
	_ PoolOpener     = (*Pools)(nil)
	_ JoinPoolOpener = (*Pools)(nil)
)

// Pools provides a mechanism for getting Pool instances.
// Note that at this time instances returned by Open are cached
// and then closed by Close. This may be a bad approach.
type Pools struct {
	log          *slog.Logger
	drvrs        Provider
	mu           sync.Mutex
	scratchSrcFn ScratchSrcFunc
	pools        map[string]Pool
	clnup        *cleanup.Cleanup
}

// NewPools returns a Pools instances.
func NewPools(log *slog.Logger, drvrs Provider, scratchSrcFn ScratchSrcFunc) *Pools {
	return &Pools{
		log:          log,
		drvrs:        drvrs,
		mu:           sync.Mutex{},
		scratchSrcFn: scratchSrcFn,
		pools:        map[string]Pool{},
		clnup:        cleanup.New(),
	}
}

// Open returns an opened Pool for src. The returned Pool
// may be cached and returned on future invocations for the
// same source (where each source fields is identical).
// Thus, the caller should typically not close
// the Pool: it will be closed via d.Close.
//
// NOTE: This entire logic re caching/not-closing is a bit sketchy,
// and needs to be revisited.
//
// Open implements PoolOpener.
func (d *Pools) Open(ctx context.Context, src *source.Source) (Pool, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)

	d.mu.Lock()
	defer d.mu.Unlock()

	key := src.Handle + "_" + hashSource(src)

	pool, ok := d.pools[key]
	if ok {
		return pool, nil
	}

	drvr, err := d.drvrs.DriverFor(src.Type)
	if err != nil {
		return nil, err
	}

	baseOptions := options.FromContext(ctx)
	o := options.Merge(baseOptions, src.Options)

	ctx = options.NewContext(ctx, o)
	pool, err = drvr.Open(ctx, src)
	if err != nil {
		return nil, err
	}

	d.clnup.AddC(pool)

	d.pools[key] = pool
	return pool, nil
}

// OpenScratch returns a scratch database instance. It is not
// necessary for the caller to close the returned Pool as
// its Close method will be invoked by d.Close.
//
// OpenScratch implements ScratchPoolOpener.
func (d *Pools) OpenScratch(ctx context.Context, name string) (Pool, error) {
	const msgCloseScratch = "close scratch db"

	scratchSrc, cleanFn, err := d.scratchSrcFn(ctx, name)
	if err != nil {
		// if err is non-nil, cleanup is guaranteed to be nil
		return nil, err
	}
	d.log.Debug("Opening scratch src", lga.Src, scratchSrc)

	drvr, err := d.drvrs.DriverFor(scratchSrc.Type)
	if err != nil {
		lg.WarnIfFuncError(d.log, msgCloseScratch, cleanFn)
		return nil, err
	}

	sqlDrvr, ok := drvr.(SQLDriver)
	if !ok {
		lg.WarnIfFuncError(d.log, msgCloseScratch, cleanFn)
		return nil, errz.Errorf("driver for scratch source %s is not a SQLDriver but is %T", scratchSrc.Handle, drvr)
	}

	var backingPool Pool
	backingPool, err = sqlDrvr.Open(ctx, scratchSrc)
	if err != nil {
		lg.WarnIfFuncError(d.log, msgCloseScratch, cleanFn)
		return nil, err
	}

	d.clnup.AddE(cleanFn)
	return backingPool, nil
}

// OpenJoin opens an appropriate database for use as
// a work DB for joining across sources.
//
// Note: There is much work to be done on this method. At this time, only
// two sources are supported. Ultimately OpenJoin should be able to
// inspect the join srcs and use heuristics to determine the best
// location for the join to occur (to minimize copying of data for
// the join etc.). Currently the implementation simply delegates
// to OpenScratch.
//
// OpenJoin implements JoinPoolOpener.
func (d *Pools) OpenJoin(ctx context.Context, srcs ...*source.Source) (Pool, error) {
	var names []string
	for _, src := range srcs {
		names = append(names, src.Handle[1:])
	}

	d.log.Debug("OpenJoin", "sources", strings.Join(names, ","))
	return d.OpenScratch(ctx, "joindb__"+strings.Join(names, "_"))
}

// Close closes d, invoking Close on any instances opened via d.Open.
func (d *Pools) Close() error {
	d.log.Debug("Closing databases(s)...", lga.Count, d.clnup.Len())
	return d.clnup.Run()
}

// OpeningPing is a standardized mechanism to ping db using
// driver.OptConnOpenTimeout. This should be invoked by each SQL
// driver impl in its Open method. If the ping fails, db is closed.
func OpeningPing(ctx context.Context, src *source.Source, db *sql.DB) error {
	o := options.FromContext(ctx)
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

// hashSource computes a hash for src. If src is nil, empty string is returned.
func hashSource(src *source.Source) string {
	if src == nil {
		return ""
	}

	h := sha256.New()
	h.Write([]byte(src.Handle))
	h.Write([]byte(src.Location))
	h.Write([]byte(src.Type))

	if len(src.Options) > 0 {
		keys := src.Options.Keys()
		for _, k := range keys {
			v := src.Options[k]
			h.Write([]byte(fmt.Sprintf("%s:%v", k, v)))
		}
	}

	b := h.Sum(nil)
	return string(b)
}
