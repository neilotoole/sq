package driver

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
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
	// OpenScratchFor returns a pool for scratch use.
	OpenScratchFor(ctx context.Context, src *source.Source) (Pool, error)

	// OpenCachedFor returns any already cached ingested pool for src.
	// If no such cache, or if it's expired, false is returned.
	OpenCachedFor(ctx context.Context, src *source.Source) (Pool, bool, error)

	// OpenIngest opens a pool for src by executing ingestFn. If allowCache
	// is false, ingest always occurs; if true, the cache is consulted first.
	OpenIngest(ctx context.Context, src *source.Source,
		ingestFn func(ctx context.Context, destPool Pool) error, allowCache bool) (Pool, error)
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
	_ PoolOpener        = (*Pools)(nil)
	_ JoinPoolOpener    = (*Pools)(nil)
	_ ScratchPoolOpener = (*Pools)(nil)
)

// ScratchSrcFunc is a function that returns a scratch source.
// The caller is responsible for invoking cleanFn.
type ScratchSrcFunc func(ctx context.Context, name string) (src *source.Source, cleanFn func() error, err error)

// Pools provides a mechanism for getting Pool instances.
// Note that at this time instances returned by Open are cached
// and then closed by Close. This may be a bad approach.
//
// FIXME: Why not rename driver.Pools to driver.Sources?
type Pools struct {
	log          *slog.Logger
	drvrs        Provider
	mu           sync.Mutex
	scratchSrcFn ScratchSrcFunc
	files        *source.Files
	pools        map[string]Pool
	clnup        *cleanup.Cleanup
}

// NewPools returns a Pools instances.
func NewPools(log *slog.Logger, drvrs Provider,
	files *source.Files, scratchSrcFn ScratchSrcFunc,
) *Pools {
	return &Pools{
		log:          log,
		drvrs:        drvrs,
		mu:           sync.Mutex{},
		scratchSrcFn: scratchSrcFn,
		files:        files,
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

// OpenScratchFor returns a scratch database instance. It is not
// necessary for the caller to close the returned Pool as
// its Close method will be invoked by d.Close.
//
// OpenScratchFor implements ScratchPoolOpener.
//
// REVISIT: do we really need to pass a source here? Just a string should do.
//
// FIXME: the problem is with passing src?
//
// FIXME: Add cacheAllowed bool?
func (d *Pools) OpenScratchFor(ctx context.Context, src *source.Source) (Pool, error) {
	const msgCloseScratch = "Close scratch db"

	_, _, srcCacheFilepath, err := d.getCachePaths(src)
	if err != nil {
		return nil, err
	}

	scratchSrc, cleanFn, err := d.scratchSrcFn(ctx, srcCacheFilepath)
	if err != nil {
		// if err is non-nil, cleanup is guaranteed to be nil
		return nil, err
	}
	d.log.Debug("Opening scratch src", lga.Src, scratchSrc)

	backingDrvr, err := d.drvrs.DriverFor(scratchSrc.Type)
	if err != nil {
		lg.WarnIfFuncError(d.log, msgCloseScratch, cleanFn)
		return nil, err
	}

	var backingPool Pool
	backingPool, err = backingDrvr.Open(ctx, scratchSrc)
	if err != nil {
		lg.WarnIfFuncError(d.log, msgCloseScratch, cleanFn)
		return nil, err
	}

	allowCache := OptIngestCache.Get(options.FromContext(ctx))
	if !allowCache {
		// If the ingest cache is disabled, we add the cleanup func
		// so the scratch DB is deleted when the session ends.
		d.clnup.AddE(cleanFn)
	}

	return backingPool, nil
}

// OpenIngest implements driver.ScratchPoolOpener.
func (d *Pools) OpenIngest(ctx context.Context, src *source.Source,
	ingestFn func(ctx context.Context, destPool Pool) error, allowCache bool,
) (Pool, error) {
	log := lg.FromContext(ctx)

	ingestFilePath, err := d.files.Filepath(src)
	if err != nil {
		return nil, err
	}

	var checksumsPath string
	var cacheDir string
	if cacheDir, _, checksumsPath, err = d.getCachePaths(src); err != nil {
		return nil, err
	}

	log.Debug("Using cache dir", lga.Path, cacheDir)

	if allowCache {
		var (
			impl        Pool
			foundCached bool
		)
		if impl, foundCached, err = d.OpenCachedFor(ctx, src); err != nil {
			return nil, err
		}
		if foundCached {
			log.Debug("Ingest cache HIT: found cached copy of source",
				lga.Src, src, "cached", impl.Source(),
			)
			return impl, nil
		}

		log.Debug("Ingest cache MISS: no cache for source", lga.Src, src)
	}

	impl, err := d.OpenScratchFor(ctx, src)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	err = ingestFn(ctx, impl)
	elapsed := time.Since(start)

	if err != nil {
		log.Error("Ingest failed",
			lga.Src, src, lga.Dest, impl.Source(),
			lga.Elapsed, elapsed, lga.Err, err,
		)
		return nil, err
	}

	log.Debug("Ingest completed", lga.Src, src, lga.Dest, impl.Source(), lga.Elapsed, elapsed)

	if allowCache {
		// Write the checksums file.
		var sum string
		if sum, err = ioz.FileChecksum(ingestFilePath); err != nil {
			log.Warn("Failed to compute checksum for source file; caching not in effect",
				lga.Src, src, lga.Dest, impl.Source(), lga.Path, ingestFilePath, lga.Err, err)
			return impl, nil
		}

		if err = ioz.WriteChecksumFile(checksumsPath, sum, ingestFilePath); err != nil {
			log.Warn("Failed to write checksum; file caching not in effect",
				lga.Src, src, lga.Dest, impl.Source(), lga.Path, ingestFilePath, lga.Err, err)
		}
	}

	return impl, nil
}

// getCachePaths returns the paths to the cache files for src.
// There is no guarantee that these files exist, or are accessible.
// It's just the paths.
func (d *Pools) getCachePaths(src *source.Source) (dir, cacheDB, checksums string, err error) { //nolint:unparam
	if dir, err = source.CacheDirFor(src); err != nil {
		return "", "", "", err
	}

	checksums = filepath.Join(dir, "checksums.txt")
	cacheDB = filepath.Join(dir, "cached.db")
	return dir, cacheDB, checksums, nil
}

// OpenCachedFor implements ScratchPoolOpener.
func (d *Pools) OpenCachedFor(ctx context.Context, src *source.Source) (Pool, bool, error) {
	_, cacheDBPath, checksumsPath, err := d.getCachePaths(src)
	if err != nil {
		return nil, false, err
	}

	if !ioz.FileAccessible(checksumsPath) {
		return nil, false, nil
	}

	mChecksums, err := ioz.ReadChecksumsFile(checksumsPath)
	if err != nil {
		return nil, false, err
	}

	drvr, err := d.drvrs.DriverFor(src.Type)
	if err != nil {
		return nil, false, err
	}

	if drvr.DriverMetadata().IsSQL {
		return nil, false, errz.Errorf("open file cache for source %s: driver {%s} is SQL, not document",
			src.Handle, src.Type)
	}

	srcFilepath, err := d.files.Filepath(src)
	if err != nil {
		return nil, false, err
	}

	cachedChecksum, ok := mChecksums[srcFilepath]
	if !ok {
		return nil, false, nil
	}

	srcChecksum, err := ioz.FileChecksum(srcFilepath)
	if err != nil {
		return nil, false, err
	}

	if srcChecksum != cachedChecksum {
		return nil, false, nil
	}

	// The checksums match, so we can use the cached DB,
	// if it exists.
	if !ioz.FileAccessible(cacheDBPath) {
		return nil, false, nil
	}

	backingType, err := d.files.DriverType(ctx, cacheDBPath)
	if err != nil {
		return nil, false, err
	}

	backingSrc := &source.Source{
		Handle:   src.Handle + "_cached",
		Location: cacheDBPath,
		Type:     backingType,
	}

	backingPool, err := d.Open(ctx, backingSrc)
	if err != nil {
		return nil, false, errz.Wrapf(err, "open cached DB for source %s", src.Handle)
	}

	return backingPool, true, nil
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
	return d.OpenScratchFor(ctx, srcs[0])
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
