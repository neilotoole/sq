// Package testh (test helper) contains functionality
// for testing.
package testh

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/config/yamlstore"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/drivers/csv"
	"github.com/neilotoole/sq/drivers/json"
	"github.com/neilotoole/sq/drivers/mysql"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/drivers/sqlserver"
	"github.com/neilotoole/sq/drivers/userdriver"
	"github.com/neilotoole/sq/drivers/userdriver/xmlud"
	"github.com/neilotoole/sq/drivers/xlsx"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/lockfile"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/testsrc"
	"github.com/neilotoole/sq/testh/tu"
)

// Option is a functional option type used with New to
// configure the helper.
type Option func(h *Helper)

// OptCaching enables or disables ingest caching.
func OptCaching(enable bool) Option {
	return func(h *Helper) {
		o := options.FromContext(h.Context)
		if o == nil {
			o = options.Options{driver.OptIngestCache.Key(): enable}
			h.Context = options.NewContext(h.Context, o)
			return
		}

		o[driver.OptIngestCache.Key()] = enable
	}
}

// OptNoLog disables the test logger.
func OptNoLog() Option {
	return func(h *Helper) {
		if h.Context == nil {
			h.Context = context.Background()
		}
		h.Context = lg.NewContext(h.Context, lg.Discard())
	}
}

// Helper encapsulates a test helper session.
type Helper struct {
	T testing.TB

	Context context.Context

	registry *driver.Registry
	files    *files.Files
	grips    *driver.Grips
	run      *run.Run

	coll     *source.Collection
	srcCache map[string]*source.Source

	cancelFn context.CancelFunc

	Cleanup *cleanup.Cleanup

	initOnce sync.Once

	closeOnce sync.Once

	mu sync.Mutex
}

// New returns a new Helper. The helper's Close func will be
// automatically invoked via t.Cleanup.
func New(tb testing.TB, opts ...Option) *Helper { //nolint:thelper
	h := &Helper{
		T:       tb,
		Cleanup: cleanup.New(),
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	h.cancelFn = cancelFn

	h.Context = lg.NewContext(ctx, lgt.New(tb))
	tb.Cleanup(h.Close)

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// NewWith is a convenience wrapper for New, that also returns
// the source.Source for handle, the driver.SQLDriver, driver.Grip,
// and the *sql.DB.
func NewWith(tb testing.TB, handle string) (*Helper, *source.Source, //nolint:revive,thelper
	driver.SQLDriver, driver.Grip, *sql.DB,
) {
	th := New(tb)
	src := th.Source(handle)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(tb, err)

	return th, src, drvr, grip, db
}

func (h *Helper) Log() *slog.Logger {
	return lg.FromContext(h.Context)
}

func (h *Helper) init() {
	h.initOnce.Do(func() {
		optRegistry := &options.Registry{}
		cli.RegisterDefaultOpts(optRegistry)
		h.registry = driver.NewRegistry(h.Log())

		cfg := config.New()
		var err error

		h.files, err = files.New(
			h.Context,
			optRegistry,
			TempLockFunc(h.T),
			tu.TempDir(h.T, "temp"),
			tu.TempDir(h.T, "cache"),
		)
		require.NoError(h.T, err)

		h.Cleanup.Add(func() {
			lg.FromContext(h.Context).Debug("Executing outer Files cleanup")
			err := h.files.Close()
			assert.NoError(h.T, err)
		})

		h.grips = driver.NewGrips(h.registry, h.files, sqlite3.NewScratchSource)
		h.Cleanup.AddC(h.grips)

		h.registry.AddProvider(drivertype.SQLite, &sqlite3.Provider{Log: h.Log()})
		h.registry.AddProvider(drivertype.Pg, &postgres.Provider{Log: h.Log()})
		h.registry.AddProvider(drivertype.MSSQL, &sqlserver.Provider{Log: h.Log()})
		h.registry.AddProvider(drivertype.MySQL, &mysql.Provider{Log: h.Log()})

		csvp := &csv.Provider{Log: h.Log(), Ingester: h.grips, Files: h.files}
		h.registry.AddProvider(drivertype.CSV, csvp)
		h.registry.AddProvider(drivertype.TSV, csvp)
		h.files.AddDriverDetectors(csv.DetectCSV, csv.DetectTSV)

		jsonp := &json.Provider{Log: h.Log(), Ingester: h.grips, Files: h.files}
		h.registry.AddProvider(drivertype.JSON, jsonp)
		h.registry.AddProvider(drivertype.JSONA, jsonp)
		h.registry.AddProvider(drivertype.JSONL, jsonp)
		h.files.AddDriverDetectors(
			json.DetectJSON(driver.OptIngestSampleSize.Get(nil)),
			json.DetectJSONA(driver.OptIngestSampleSize.Get(nil)),
			json.DetectJSONL(driver.OptIngestSampleSize.Get(nil)),
		)

		h.registry.AddProvider(drivertype.XLSX, &xlsx.Provider{Log: h.Log(), Ingester: h.grips, Files: h.files})
		h.files.AddDriverDetectors(xlsx.DetectXLSX)

		h.addUserDrivers()

		h.run = &run.Run{
			Stdin:           os.Stdin,
			Out:             os.Stdout,
			ErrOut:          os.Stdin,
			Config:          cfg,
			ConfigStore:     config.DiscardStore{},
			OptionsRegistry: optRegistry,
			DriverRegistry:  h.registry,
		}

		h.Cleanup.AddC(h.run)
	})
}

// Close runs any cleanup tasks, failing h's testing.T if any cleanup
// error occurs. Close is automatically invoked via t.Cleanup, it does
// not need to be explicitly invoked unless desired.
func (h *Helper) Close() {
	h.closeOnce.Do(func() {
		err := h.Cleanup.Run()
		lg.WarnIfError(h.Log(), "helper cleanup", err)
		assert.NoError(h.T, err)
		h.cancelFn()
	})
}

// Add adds src to the helper's collection.
func (h *Helper) Add(src *source.Source) *source.Source {
	// This is a bit of a hack to ensure that internals are loaded: we
	// load a known source. The loading mechanism should be refactored
	// to not require this.
	_ = h.Source(sakila.SL3)

	h.mu.Lock()
	defer h.mu.Unlock()

	require.False(h.T, h.coll.IsExistingSource(src.Handle),
		"source {%s} already exists", src.Handle)

	require.NoError(h.T, h.coll.Add(src))

	h.srcCache[src.Handle] = src

	// envDiffDB is the name of the envar that controls whether the testing
	// diffdb mechanism is executed automatically by Helper.Source.
	const envDiffDB = "SQ_TEST_DIFFDB"

	if proj.BoolEnvar(envDiffDB) {
		h.DiffDB(src)
	}

	return src
}

// Source returns a test Source with the given handle. The standard test
// source collection is loaded from the sq config file at TestSourcesConfigPath,
// (but additional sources can be added via Helper.Add). Variables
// such as ${SQ_ROOT} in the config file are expanded. The same
// instance of *source.Source will be returned for multiple invocations
// of this method on the same Helper instance.
//
// For certain file-based driver types, the returned src's Location
// may point to a copy of the file. This helps avoid tests dirtying
// a version-controlled data file.
//
// Any external database source (that is, any SQL source other than SQLite3)
// will have its location determined from an envar. Given a source @sakila_pg12,
// its location is derived from an envar SQ_TEST_SRC__SAKILA_PG12. If that envar
// is not set, the test calling this method will be skipped.
//
// If envar SQ_TEST_DIFFDB is true, DiffDB is run on every SQL source
// returned by Source.
func (h *Helper) Source(handle string) *source.Source {
	h.mu.Lock()
	defer h.mu.Unlock()
	t := h.T

	// invoke h.init to ensure that its cleanup side effects
	// happen in the correct order (files get cleaned after
	// databases, etc.).
	h.init()

	// If the handle refers to an external database, we will skip
	// the test if the envar for the handle is not set.
	if stringz.InSlice(sakila.SQLAllExternal(), handle) {
		// Skip the test if the envar for the handle is not set
		handleEnvar := "SQ_TEST_SRC__" + strings.ToUpper(strings.TrimPrefix(handle, "@"))
		if envar, ok := os.LookupEnv(handleEnvar); !ok || strings.TrimSpace(envar) == "" {
			h.T.Skipf("Skip test %s because envar {%s} for {%s} is not set",
				h.T.Name(), handleEnvar, handle)
		}
	}

	if h.coll == nil {
		// It might be expected that we would simply use the
		// collection (h.coll) to return the source, but this
		// method also uses a cache. This is because this
		// method makes a copy of the data file of file-based sources
		// as mentioned in the method godoc.
		h.coll = mustLoadCollection(h.Context, t)
		h.srcCache = map[string]*source.Source{}
	}

	// If it's already in the cache, return it.
	src, ok := h.srcCache[handle]
	if ok {
		return src
	}

	src, err := h.coll.Get(handle)
	require.NoError(t, err,
		"source %s was not found in %s", handle, testsrc.PathSrcsConfig)

	if src.Type == drivertype.SQLite {
		// This could be easily generalized for CSV/XLSX etc.
		srcPath, err := sqlite3.PathFromLocation(src)
		require.NoError(t, err)

		dstPath := filepath.Join(tu.TempDir(t), filepath.Base(srcPath))
		require.NoError(t, ioz.CopyFile(dstPath, srcPath, true))
		src.Location = sqlite3.Prefix + dstPath
	}

	h.srcCache[handle] = src

	// envDiffDB is the name of the envar that controls whether the testing
	// diffdb mechanism is executed automatically by Helper.Source.
	const envDiffDB = "SQ_TEST_DIFFDB"

	if proj.BoolEnvar(envDiffDB) {
		h.DiffDB(src)
	}

	return src
}

// SourceConfigured returns true if the source is configured. Note
// that Helper.Source skips the test if the source is not configured: that
// is to say, if the source location requires population via an envar, and
// the envar is not set. For example, for the PostgreSQL source @sakila_pg12,
// the envar SQ_TEST_SRC__SAKILA_PG12 is required. SourceConfigured tests
// if that envar is set.
func (h *Helper) SourceConfigured(handle string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !stringz.InSlice(sakila.SQLAllExternal(), handle) {
		// Non-SQL and SQLite sources are always available.
		return true
	}

	handleEnvar := "SQ_TEST_SRC__" + strings.ToUpper(strings.TrimPrefix(handle, "@"))
	if envar, ok := os.LookupEnv(handleEnvar); !ok || strings.TrimSpace(envar) == "" {
		return false
	}

	return true
}

// NewCollection is a convenience function for building a
// new *source.Collection incorporating the supplied handles. See
// Helper.Source for more on the behavior.
func (h *Helper) NewCollection(handles ...string) *source.Collection {
	coll := &source.Collection{}
	for _, handle := range handles {
		require.NoError(h.T, coll.Add(h.Source(handle)))
	}
	return coll
}

// Open opens a driver.Grip for src via h's internal Grips
// instance: thus subsequent calls to Open may return the
// same driver.Grip instance. The opened driver.Grip will be closed
// during h.Close.
func (h *Helper) Open(src *source.Source) driver.Grip {
	ctx := h.Context
	grip, err := h.Grips().Open(ctx, src)
	require.NoError(h.T, err)

	db, err := grip.DB(ctx)
	require.NoError(h.T, err)

	require.NoError(h.T, db.PingContext(ctx))
	return grip
}

// OpenDB is a convenience method for getting the sql.DB for src.
// The returned sql.DB is closed during h.Close, via the closing
// of its parent driver.Grip.
func (h *Helper) OpenDB(src *source.Source) *sql.DB {
	grip := h.Open(src)
	db, err := grip.DB(h.Context)
	require.NoError(h.T, err)
	return db
}

// openNew opens a new driver.Grip. It is the caller's responsibility
// to close the returned Grip. Unlike method Open, this method
// will always invoke the driver's Open method.
//
// Some of Helper's methods (e.g. DropTable) need to use openNew rather
// than Open, as the Grip returned by Open can be closed by test code,
// potentially causing problems during Cleanup.
func (h *Helper) openNew(src *source.Source) driver.Grip {
	h.Log().Debug("openNew", lga.Src, src)
	reg := h.Registry()
	drvr, err := reg.DriverFor(src.Type)
	require.NoError(h.T, err)
	grip, err := drvr.Open(h.Context, src)
	require.NoError(h.T, err)
	return grip
}

// SQLDriverFor is a convenience method to get src's driver.SQLDriver.
func (h *Helper) SQLDriverFor(src *source.Source) driver.SQLDriver {
	reg := h.Registry()
	drvr, err := reg.DriverFor(src.Type)
	require.NoError(h.T, err)
	sqlDrvr, ok := drvr.(driver.SQLDriver)
	require.True(h.T, ok,
		"driver %T is not a driver.SQLDriver: ensure that the src passed to SQLDriverFor implements driver.SQLDriver",
		drvr)
	return sqlDrvr
}

// DriverFor is a convenience method to get src's driver.Driver.
func (h *Helper) DriverFor(src *source.Source) driver.Driver {
	reg := h.Registry()
	drvr, err := reg.DriverFor(src.Type)
	require.NoError(h.T, err)
	return drvr
}

// RowCount returns the result of "SELECT COUNT(*) FROM tbl",
// failing h's test on any error.
func (h *Helper) RowCount(src *source.Source, tbl string) int64 {
	grip := h.openNew(src)
	defer lg.WarnIfCloseError(h.Log(), lgm.CloseDB, grip)

	query := "SELECT COUNT(*) FROM " + grip.SQLDriver().Dialect().Enquote(tbl)
	var count int64
	db, err := grip.DB(h.Context)
	require.NoError(h.T, err)

	require.NoError(h.T, db.QueryRowContext(h.Context, query).Scan(&count))
	return count
}

// CreateTable creates a new table in src, and inserts data, returning
// the number of data rows inserted. If dropAfter is true, the created
// table is dropped when t.Cleanup is run.
func (h *Helper) CreateTable(dropAfter bool, src *source.Source, tblDef *schema.Table,
	data ...[]any,
) (affected int64) {
	grip := h.openNew(src)
	defer lg.WarnIfCloseError(h.Log(), lgm.CloseDB, grip)

	db, err := grip.DB(h.Context)
	require.NoError(h.T, err)

	require.NoError(h.T, grip.SQLDriver().CreateTable(h.Context, db, tblDef))
	h.T.Logf("Created table %s.%s", src.Handle, tblDef.Name)

	if dropAfter {
		h.Cleanup.Add(func() { h.DropTable(src, tablefq.From(tblDef.Name)) })
	}

	if len(data) == 0 {
		return 0
	}

	return h.Insert(src, tblDef.Name, tblDef.ColNames(), data...)
}

// Insert inserts records for cols into src.tbl, returning the number of
// records inserted. Note that the records arg may be mutated by src's
// driver InsertMungeFunc.
func (h *Helper) Insert(src *source.Source, tbl string, cols []string, records ...[]any) (affected int64) {
	if len(records) == 0 {
		return 0
	}

	grip := h.openNew(src)
	defer lg.WarnIfCloseError(h.Log(), lgm.CloseDB, grip)

	drvr := grip.SQLDriver()
	db, err := grip.DB(h.Context)
	require.NoError(h.T, err)

	conn, err := db.Conn(h.Context)
	require.NoError(h.T, err)
	defer lg.WarnIfCloseError(h.Log(), lgm.CloseDB, conn)

	batchSize := driver.MaxBatchRows(drvr, len(cols))
	bi, err := driver.NewBatchInsert(
		h.Context,
		libsq.MsgIngestRecords,
		drvr,
		conn,
		tbl,
		cols,
		batchSize,
	)
	require.NoError(h.T, err)

	for _, rec := range records {
		require.NoError(h.T, bi.Munge(rec))

		select {
		case <-h.Context.Done():
			// Should not happen
			close(bi.RecordCh)
			require.NoError(h.T, h.Context.Err())

		case err = <-bi.ErrCh:
			// Should not happen
			if err != nil {
				close(bi.RecordCh)
				require.NoError(h.T, err)
			} else {
				break
			}

		case bi.RecordCh <- rec:
		}
	}

	close(bi.RecordCh) // Indicate that we're finished writing records

	err = <-bi.ErrCh // Stop for bi to complete
	require.NoError(h.T, err)

	h.T.Logf("Inserted %d rows to %s.%s", bi.Written(), src.Handle, tbl)
	return bi.Written()
}

// CopyTable copies fromTable into a new table toTable. If
// toTable is empty, a unique table name is generated based on
// fromTable. The table name used is returned.
// If dropAfter is true, the table is dropped when t.Cleanup is run.
// If copyData is true, fromTable's data is also copied.
// Constraints (keys, defaults etc.) may not be copied.
//
// TODO: CopyTable should return tablefq.T instead of string.
func (h *Helper) CopyTable(
	dropAfter bool,
	src *source.Source,
	fromTable, toTable tablefq.T,
	copyData bool,
) string {
	if lo.IsEmpty(toTable) {
		toTable = fromTable
		toTable.Table = stringz.UniqTableName(fromTable.Table)
	}

	grip := h.openNew(src)
	defer lg.WarnIfCloseError(h.Log(), lgm.CloseDB, grip)

	db, err := grip.DB(h.Context)
	require.NoError(h.T, err)

	copied, err := grip.SQLDriver().CopyTable(
		h.Context,
		db,
		fromTable,
		toTable,
		copyData,
	)
	require.NoError(h.T, err)
	if dropAfter {
		h.Cleanup.Add(func() { h.DropTable(src, toTable) })
	}

	h.Log().Debug("Copied table",
		lga.From, fromTable,
		lga.To, toTable,
		"copy_data", copyData,
		lga.Count, copied,
		"drop_after", dropAfter,
	)
	return toTable.Table
}

// DropTable drops tbl from src.
func (h *Helper) DropTable(src *source.Source, tbl tablefq.T) {
	grip := h.openNew(src)
	defer lg.WarnIfCloseError(h.Log(), lgm.CloseDB, grip)

	db, err := grip.DB(h.Context)
	require.NoError(h.T, err)

	require.NoError(h.T, grip.SQLDriver().DropTable(h.Context, db, tbl, true))
	h.Log().Debug("Dropped table", lga.Target, source.Target(src, tbl.Table))
}

// QuerySQL uses libsq.QuerySQL to execute SQL query
// against src, returning a sink to which all records have
// been written. Typically the db arg is nil, and QuerySQL uses the
// same driver.Grip instance as returned by Helper.Open. If db
// is non-nil, it is passed to libsq.QuerySQL (e.g. the query needs to
// execute against a sql.Tx), and the caller is responsible for closing db.
func (h *Helper) QuerySQL(src *source.Source, db sqlz.DB, query string, args ...any) (*RecordSink, error) {
	grip := h.Open(src)

	sink := &RecordSink{}
	recw := output.NewRecordWriterAdapter(h.Context, sink)
	err := libsq.QuerySQL(h.Context, grip, db, recw, query, args...)
	if err != nil {
		return nil, err
	}

	_, err = recw.Wait()
	if err != nil {
		return nil, err
	}
	return sink, nil
}

// QuerySLQ executes the SLQ query. Args are predefined variables for
// substitution.
func (h *Helper) QuerySLQ(query string, args map[string]string) (*RecordSink, error) {
	// h.init()
	// We need to ensure that each of the handles in the query is loaded.
	a, err := ast.Parse(h.Log(), query)
	require.NoError(h.T, err)

	for _, handle := range ast.NewInspector(a).FindHandles() {
		// This triggers handle loading
		_ = h.Source(handle)
	}

	qc := &libsq.QueryContext{
		Collection: h.coll,
		Grips:      h.grips,
		Args:       args,
	}

	sink := &RecordSink{}
	recw := output.NewRecordWriterAdapter(h.Context, sink)

	err = libsq.ExecuteSLQ(h.Context, qc, query, recw)
	if err != nil {
		return nil, err
	}

	if _, err = recw.Wait(); err != nil {
		return nil, err
	}

	return sink, nil
}

// ExecSQL is a convenience wrapper for sql.DB.Exec that returns the
// rows affected, failing on any error. Note that ExecSQL uses the
// same Grip instance as returned by h.Open.
func (h *Helper) ExecSQL(src *source.Source, query string, args ...any) (affected int64) {
	db := h.OpenDB(src)

	res, err := db.ExecContext(h.Context, query, args...)

	require.NoError(h.T, err)

	affected, err = res.RowsAffected()
	require.NoError(h.T, err)

	return affected
}

// InsertDefaultRow executes the equivalent
// of INSERT INTO tbl DEFAULT VALUES.
// It fails if a row was not inserted.
//
// Note that for some driver types, the driver does not
// support DEFAULT values for some col types. For example
// this method may fail for a MySQL column "col_text TEXT NOT NULL",
// as TEXT and BLOB cannot have a DEFAULT value.
func (h *Helper) InsertDefaultRow(src *source.Source, tbl string) {
	drvr := h.SQLDriverFor(src)
	var query string

	if src.Type == drivertype.MySQL {
		// One driver had to be different...
		// We could push this mysql-specific logic down to the driver impl
		// but prob not worth the effort just for one driver.
		query = "INSERT INTO " + drvr.Dialect().Enquote(tbl) + " () VALUES ()"
	} else {
		query = "INSERT INTO " + drvr.Dialect().Enquote(tbl) + " DEFAULT VALUES"
	}

	affected := h.ExecSQL(src, query)
	require.Equal(h.T, int64(1), affected)
}

// TruncateTable truncates tbl in src.
func (h *Helper) TruncateTable(src *source.Source, tbl string) (affected int64) {
	grip := h.openNew(src)
	defer lg.WarnIfCloseError(h.Log(), lgm.CloseDB, grip)

	drvr := h.SQLDriverFor(src)
	require.NotNil(h.T, drvr, "not a SQL driver")

	affected, err := drvr.Truncate(h.Context, src, tbl, true)
	require.NoError(h.T, err)
	return affected
}

// Registry returns the helper's registry instance,
// configured with standard providers.
func (h *Helper) Registry() *driver.Registry {
	h.init()
	return h.registry
}

// Run returns the helper's run instance.
func (h *Helper) Run() *run.Run {
	h.init()
	return h.run
}

// addUserDrivers adds some user drivers to the registry.
func (h *Helper) addUserDrivers() {
	userDriverDefs := DriverDefsFrom(h.T, testsrc.PathDriverDefPpl, testsrc.PathDriverDefRSS)

	// One day we may have more supported user driver genres.
	userDriverImporters := map[string]userdriver.IngestFunc{
		xmlud.Genre: xmlud.Ingest,
	}

	for _, userDriverDef := range userDriverDefs {
		userDriverDef := userDriverDef

		errs := userdriver.ValidateDriverDef(userDriverDef)
		require.Empty(h.T, errs)

		importFn, ok := userDriverImporters[userDriverDef.Genre]
		require.True(h.T, ok, "unsupported genre {%s} for user driver {%s} specified via config",
			userDriverDef.Genre, userDriverDef.Name)

		// For each user driver definition, we register a
		// distinct userdriver.Provider instance.
		udp := &userdriver.Provider{
			Log:       h.Log(),
			DriverDef: userDriverDef,
			IngestFn:  importFn,
			Ingester:  h.grips,
			Files:     h.files,
		}

		h.registry.AddProvider(drivertype.Type(userDriverDef.Name), udp)
		h.files.AddDriverDetectors(udp.Detectors()...)
	}
}

// IsMonotable returns true if src's driver is monotable.
func (h *Helper) IsMonotable(src *source.Source) bool {
	return h.DriverFor(src).DriverMetadata().Monotable
}

// Grips returns the helper's driver.Grips instance.
func (h *Helper) Grips() *driver.Grips {
	h.init()
	return h.grips
}

// Files returns the helper's source.Files instance.
func (h *Helper) Files() *files.Files {
	h.init()
	return h.files
}

// SourceMetadata returns metadata for src.
func (h *Helper) SourceMetadata(src *source.Source) (*metadata.Source, error) {
	grip, err := h.Grips().Open(h.Context, src)
	if err != nil {
		return nil, err
	}

	return grip.SourceMetadata(h.Context, false)
}

// TableMetadata returns metadata for src's table.
func (h *Helper) TableMetadata(src *source.Source, tbl string) (*metadata.Table, error) {
	grip, err := h.Grips().Open(h.Context, src)
	if err != nil {
		return nil, err
	}

	return grip.TableMetadata(h.Context, tbl)
}

// DiffDB fails the test if src's metadata is substantially different
// when t.Cleanup runs vs when DiffDB is invoked. Effectively DiffDB
// takes before and after snapshots of src's metadata, and compares
// various elements such as the number of tables, table row counts, etc.
// DiffDB is useful for verifying that tests are leaving the database
// as they found it.
//
// Note that DiffDB adds considerable overhead to test runtime.
//
// If envar SQ_TEST_DIFFDB is true, DiffDB is run on every SQL source
// returned by Helper.Source.
func (h *Helper) DiffDB(src *source.Source) {
	if !h.DriverFor(src).DriverMetadata().IsSQL {
		// SkipDiffDB for non-SQL driver types
		return
	}

	h.T.Logf("Executing DiffDB for %s", src.Handle)

	beforeDB := h.openNew(src)
	defer lg.WarnIfCloseError(h.Log(), lgm.CloseDB, beforeDB)

	beforeMeta, err := beforeDB.SourceMetadata(h.Context, false)
	require.NoError(h.T, err)

	h.Cleanup.Add(func() {
		// Currently DiffDB just checks if the tables and each
		// table's row count match.

		afterDB := h.openNew(src)
		defer lg.WarnIfCloseError(h.Log(), lgm.CloseDB, afterDB)

		afterMeta, err := afterDB.SourceMetadata(h.Context, false)
		require.NoError(h.T, err)
		require.Equal(h.T, beforeMeta.TableNames(), afterMeta.TableNames(),
			"diffdb: should have the same set of tables before and after")

		for i, beforeTbl := range beforeMeta.Tables {
			assert.Equal(h.T, beforeTbl.RowCount, afterMeta.Tables[i].RowCount,
				"diffdb: %s: row count for {%s} is expected to be %d but got %d", src.Handle, beforeTbl.Name,
				beforeTbl.RowCount, afterMeta.Tables[i].RowCount)
		}
	})
}

func mustLoadCollection(ctx context.Context, tb testing.TB) *source.Collection { //nolint:thelper
	hookExpand := func(data []byte) ([]byte, error) {
		// expand vars such as "${SQ_ROOT}"
		return []byte(proj.Expand(string(data))), nil
	}

	store := &yamlstore.Store{
		Path:            proj.Abs(testsrc.PathSrcsConfig),
		OptionsRegistry: &options.Registry{},
		HookLoad:        hookExpand,
	}
	cli.RegisterDefaultOpts(store.OptionsRegistry)

	cfg, err := store.Load(ctx)
	require.NoError(tb, err)
	require.NotNil(tb, cfg)
	require.NotNil(tb, cfg.Collection)

	return cfg.Collection
}

// DriverDefsFrom builds DriverDef values from cfg files.
func DriverDefsFrom(tb testing.TB, cfgFiles ...string) []*userdriver.DriverDef {
	tb.Helper()
	var userDriverDefs []*userdriver.DriverDef
	for _, f := range cfgFiles {
		ext := &config.Ext{}
		require.NoError(tb, ioz.UnmarshallYAML(proj.ReadFile(f), ext))
		userDriverDefs = append(userDriverDefs, ext.UserDrivers...)
	}
	return userDriverDefs
}

// DriverDetectors returns the common set of TypeDetectorFuncs.
func DriverDetectors() []files.TypeDetectFunc {
	return []files.TypeDetectFunc{
		files.DetectMagicNumber,
		xlsx.DetectXLSX,
		csv.DetectCSV,
		csv.DetectTSV,
		json.DetectJSON(1000),
		json.DetectJSONA(1000),
		json.DetectJSONL(1000),
	}
}

// SetBuildVersion sets the build version for the lifecycle
// of test t.
func SetBuildVersion(tb testing.TB, vers string) {
	tb.Helper()
	prevVers := buildinfo.Version
	tb.Setenv(buildinfo.EnvOverrideVersion, vers)
	buildinfo.Version = vers
	tb.Cleanup(func() {
		buildinfo.Version = prevVers
	})
}

// TempLockfile returns a lockfile.Lockfile that uses a temp file.
func TempLockfile(tb testing.TB) lockfile.Lockfile {
	tb.Helper()
	return lockfile.Lockfile(tu.TempFile(tb, "pid.lock"))
}

// TempLockFunc returns a lockfile.LockFunc that uses a temp file.
func TempLockFunc(tb testing.TB) lockfile.LockFunc {
	tb.Helper()
	return func(ctx context.Context) (unlock func(), err error) {
		lock := TempLockfile(tb)
		timeout := config.OptConfigLockTimeout.Default()
		if err = lock.Lock(ctx, timeout); err != nil {
			return nil, err
		}

		return func() {
			if err := lock.Unlock(); err != nil {
				tb.Logf("failed to release temp lock: %v", err)
			}
		}, nil
	}
}

// ExtractHandlesFromQuery returns all handles mentioned in the query.
// If failOnErr is true, the test will fail on any parse error; otherwise,
// the test will log the error and return an empty slice.
func ExtractHandlesFromQuery(tb testing.TB, query string, failOnErr bool) []string {
	tb.Helper()
	a, err := ast.Parse(lg.Discard(), query)
	if err != nil {
		if failOnErr {
			require.NoError(tb, err)
			return nil
		}
		tb.Logf("Failed to parse query: >> %s << : %v", query, err)
		return []string{}
	}

	return ast.ExtractHandles(a)
}

// NewActorSource returns a new *source.Source for a copy of the Sakila
// actor.csv datafile, using the given handle. If clean is true, the copy
// is deleted by t.Cleanup.
func NewActorSource(tb testing.TB, handle string, clean bool) *source.Source {
	tb.Helper()

	require.NoError(tb, source.ValidHandle(handle))
	tmpDir := tu.TempDir(tb)
	loc := filepath.Join(tmpDir, "actor.csv")
	err := ioz.CopyFile(
		loc,
		proj.Abs("drivers/csv/testdata/sakila-csv/actor.csv"),
		true,
	)
	require.NoError(tb, err)

	if clean {
		tb.Cleanup(func() {
			assert.NoError(tb, os.RemoveAll(tmpDir))
		})
	}
	return &source.Source{
		Handle:   handle,
		Type:     drivertype.CSV,
		Location: loc,
	}
}
