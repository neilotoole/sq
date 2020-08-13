// Package testh (test helper) contains functionality
// for testing.
package testh

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/neilotoole/lg"
	"github.com/neilotoole/lg/testlg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/drivers/csv"
	"github.com/neilotoole/sq/drivers/mysql"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/drivers/sqlserver"
	"github.com/neilotoole/sq/drivers/userdriver"
	"github.com/neilotoole/sq/drivers/userdriver/xmlud"
	"github.com/neilotoole/sq/drivers/xlsx"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/cleanup"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/sqlmodel"
	"github.com/neilotoole/sq/libsq/stringz"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/testsrc"
)

// Helper encapsulates a test helper session.
type Helper struct {
	mu       sync.Mutex
	T        testing.TB
	Log      lg.Log
	reg      *driver.Registry
	files    *source.Files
	dbases   *driver.Databases
	regOnce  sync.Once
	srcs     *source.Set
	srcCache map[string]*source.Source
	Context  context.Context
	cancelFn context.CancelFunc

	// Cleanup is used
	Cleanup *cleanup.Cleanup
}

// New returns a new Helper. The helper's Close func will be
// automatically invoked via t.Cleanup.
func New(t testing.TB) *Helper {
	h := &Helper{
		T:       t,
		Log:     testlg.New(t),
		Cleanup: cleanup.New(),
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	h.Context = ctx
	h.cancelFn = cancelFn

	t.Cleanup(h.Close)
	return h
}

// NewWith is a convenience wrapper for New that also returns
// a Source for handle, an open Database and the SQLDriver.
// The function will fail if handle is not the handle for a
// source whose driver implements driver.SQLDriver.
func NewWith(t testing.TB, handle string) (*Helper, *source.Source, driver.Database, driver.SQLDriver) {
	th := New(t)
	src := th.Source(handle)
	dbase := th.Open(src)
	drvr := th.SQLDriverFor(src)

	return th, src, dbase, drvr
}

// Close runs any cleanup tasks, failing h's testing.T if any cleanup
// error occurs. Close is automatically invoked via t.Cleanup, it does
// not need to be explicitly invoked unless desired.
func (h *Helper) Close() {
	err := h.Cleanup.Run()
	h.Log.WarnIfError(err)
	assert.NoError(h.T, err)
	h.cancelFn()
}

// Source returns a test Source with the given handle. The source
// is loaded from the sq config file at TestSourcesConfigPath. Variables
// such as ${SQ_ROOT} in the config file are expanded. The same
// instance of *source.Source will be returned for multiple invocations
// of this method on the same Helper instance.
//
// For certain file-based source types, the returned src's Location
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

	// invoke h.Registry to ensure that its cleanup side-effects
	// happen in the correct order (files get cleaned after
	// databases, etc.).
	_ = h.Registry() // FIXME: questionable if this is needed

	// If the handle refers to an external database, we will skip
	// the test if the envar for the handle is not set.
	if stringz.InSlice(sakila.SQLAllExternal(), handle) {
		// Skip the test if the envar for the handle is not set
		handleEnvar := "SQ_TEST_SRC__" + strings.ToUpper(strings.TrimPrefix(handle, "@"))
		if envar, ok := os.LookupEnv(handleEnvar); !ok || strings.TrimSpace(envar) == "" {
			h.T.Skipf("Skip test %s because envar %q for %q is not set",
				h.T.Name(), handleEnvar, handle)
		}
	}

	if h.srcs == nil {
		// It might be expected that we would simply use the
		// source set (h.srcs) to return the source, but this
		// method also uses a cache. This is because this
		// method makes a copy the data file of file-based sources
		// as mentioned in the method godoc.
		h.srcs = mustLoadSourceSet(t)
		h.srcCache = map[string]*source.Source{}
	}

	// If it's already in the cache, return it.
	src, ok := h.srcCache[handle]
	if ok {
		return src
	}

	src, err := h.srcs.Get(handle)
	require.NoError(t, err,
		"source %s was not found in %s", handle, testsrc.PathSrcsConfig)

	switch src.Type {
	case sqlite3.Type:
		// This could be easily generalized for CSV/XLSX etc.
		fpath, err := sqlite3.PathFromLocation(src)
		require.NoError(t, err)

		srcFile, err := os.Open(fpath)
		require.NoError(t, err)
		defer func() {
			assert.NoError(t, srcFile.Close())
		}()

		destFile, err := ioutil.TempFile("", "*_"+filepath.Base(src.Location))
		require.NoError(t, err)
		defer func() {
			assert.NoError(t, destFile.Close())
		}()

		destFileName := destFile.Name()

		h.Files().CleanupE(func() error {
			return errz.Err(os.Remove(destFileName))
		})

		_, err = io.Copy(destFile, srcFile)
		require.NoError(t, err)

		src.Location = sqlite3.Prefix + destFileName
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

// Open opens a Database for src via h's internal Databases
// instance: thus subsequent calls to Open may return the
// same Database instance. The opened Database will be closed
// during h.Close.
func (h *Helper) Open(src *source.Source) driver.Database {
	dbase, err := h.Databases().Open(h.Context, src)
	require.NoError(h.T, err)
	return dbase
}

// openNew opens a new Database. It is the caller's responsibility
// to close the returned Database. Unlike method Open, this method
// will always invoke the driver's Open method.
//
// Some of Helper's methods (e.g. DropTable) need to use openNew rather
// than Open, as the Database returned by Open can be closed by test code,
// potentially causing problems during Cleanup.
func (h *Helper) openNew(src *source.Source) driver.Database {
	reg := h.Registry()
	drvr, err := reg.DriverFor(src.Type)
	require.NoError(h.T, err)
	dbase, err := drvr.Open(h.Context, src)
	require.NoError(h.T, err)
	return dbase
}

// SQLDriverFor is a convenience method to get src's driver.SQLDriver.
func (h *Helper) SQLDriverFor(src *source.Source) driver.SQLDriver {
	reg := h.Registry()
	drvr, err := reg.DriverFor(src.Type)
	require.NoError(h.T, err)
	sqlDrvr, ok := drvr.(driver.SQLDriver)
	require.True(h.T, ok, "driver %T is not a driver.SQLDriver: ensure that the src passed to SQLDriverFor implements driver.SQLDriver", drvr)
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
	dbase := h.openNew(src)
	defer h.Log.WarnIfCloseError(dbase)

	query := "SELECT COUNT(*) FROM " + dbase.SQLDriver().Dialect().Enquote(tbl)
	var count int64
	require.NoError(h.T, dbase.DB().QueryRowContext(h.Context, query).Scan(&count))
	return count
}

// CreateTable creates a new table in src, and inserts data, returning
// the number of data rows inserted. If dropAfter is true, the created
// table is dropped when t.Cleanup is run.
func (h *Helper) CreateTable(dropAfter bool, src *source.Source, tblDef *sqlmodel.TableDef, data ...[]interface{}) (affected int64) {
	dbase := h.openNew(src)
	defer h.Log.WarnIfCloseError(dbase)

	require.NoError(h.T, dbase.SQLDriver().CreateTable(h.Context, dbase.DB(), tblDef))
	h.T.Logf("Created table %s.%s", src.Handle, tblDef.Name)

	if dropAfter {
		h.Cleanup.Add(func() { h.DropTable(src, tblDef.Name) })
	}

	if len(data) == 0 {
		return 0
	}

	return h.Insert(src, tblDef.Name, tblDef.ColNames(), data...)
}

// Insert inserts records for cols into src.tbl, returning the number of
// records inserted. Note that the records arg may be mutated by src's
// driver InsertMungeFunc.
func (h *Helper) Insert(src *source.Source, tbl string, cols []string, records ...[]interface{}) (affected int64) {
	if len(records) == 0 {
		return 0
	}

	dbase := h.openNew(src)
	defer h.Log.WarnIfCloseError(dbase)

	drvr := dbase.SQLDriver()

	conn, err := dbase.DB().Conn(h.Context)
	require.NoError(h.T, err)
	defer h.Log.WarnIfCloseError(conn)

	batchSize := driver.MaxBatchRows(drvr, len(cols))
	bi, err := driver.NewBatchInsert(h.Context, h.Log, drvr, conn, tbl, cols, batchSize)
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

	err = <-bi.ErrCh // Wait for bi to complete
	require.NoError(h.T, err)

	h.T.Logf("Inserted %d rows to %s.%s", bi.Written(), src.Handle, tbl)
	return bi.Written()
}

// CopyTable copies fromTable into a new table toTable. If
// toTable is empty, a table name is generated based on
// fromTable. The table name used is returned.
// If dropAfter is true, the table is dropped when t.Cleanup is run.
// If copyData is true, fromTable's data is also copied.
// Constraints (keys, defaults etc.) may not be copied.
func (h *Helper) CopyTable(dropAfter bool, src *source.Source, fromTable, toTable string, copyData bool) string {
	if toTable == "" {
		toTable = stringz.UniqTableName(fromTable)
	}

	dbase := h.openNew(src)
	defer h.Log.WarnIfCloseError(dbase)

	copied, err := dbase.SQLDriver().CopyTable(h.Context, dbase.DB(), fromTable, toTable, copyData)
	require.NoError(h.T, err)
	if dropAfter {
		h.Cleanup.Add(func() { h.DropTable(src, toTable) })
	}

	h.Log.Debugf("Copied table %s.%s --> %s.%s  (copy data=%v; drop after=%v; rows copied=%d)",
		src.Handle, fromTable, src.Handle, toTable, copyData, dropAfter, copied)
	return toTable
}

// DropTable drops tbl from src.
func (h *Helper) DropTable(src *source.Source, tbl string) {
	dbase := h.openNew(src)
	defer func() {
		h.Log.WarnIfError(errz.Err(dbase.Close()))
	}()

	require.NoError(h.T, dbase.SQLDriver().DropTable(h.Context, dbase.DB(), tbl, true))
	h.Log.Debugf("Dropped %s.%s", src.Handle, tbl)
}

// QuerySQL uses libsq.QuerySQL to execute SQL query
// against src, returning a sink to which all records have
// been written. Note that QuerySQL uses the
// same Database instance as returned by h.Open.
func (h *Helper) QuerySQL(src *source.Source, query string, args ...interface{}) (*RecordSink, error) {
	dbase := h.Open(src)

	sink := &RecordSink{}
	recw := output.NewRecordWriterAdapter(sink)
	err := libsq.QuerySQL(h.Context, h.Log, dbase, recw, query, args...)
	if err != nil {
		return nil, err
	}

	_, err = recw.Wait()
	if err != nil {
		return nil, err
	}
	return sink, nil
}

// ExecSQL is a convenience wrapper for sql.DB.Exec that returns the
// rows affected, failing on any error. Note that ExecSQL uses the
// same Database instance as returned by h.Open.
func (h *Helper) ExecSQL(src *source.Source, query string, args ...interface{}) (affected int64) {
	dbase := h.Open(src)

	res, err := dbase.DB().ExecContext(h.Context, query, args...)

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

	if src.Type == mysql.Type {
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
	dbase := h.openNew(src)
	defer h.Log.WarnIfCloseError(dbase)

	affected, err := h.DriverFor(src).Truncate(h.Context, src, tbl, true)
	require.NoError(h.T, err)
	return affected
}

// Registry returns the helper's registry instance,
// configured with standard providers. Invoking Registry has
// the important side-effect of initializing the helper's registry,
// files, and databases fields.
func (h *Helper) Registry() *driver.Registry {
	h.regOnce.Do(func() {
		log := h.Log
		h.reg = driver.NewRegistry(log)

		var err error
		h.files, err = source.NewFiles(log)
		require.NoError(h.T, err)
		h.Cleanup.AddC(h.files)
		h.files.AddTypeDetectors(source.DetectMagicNumber)

		h.dbases = driver.NewDatabases(log, h.reg, sqlite3.NewScratchSource)
		h.Cleanup.AddC(h.dbases)

		h.reg.AddProvider(sqlite3.Type, &sqlite3.Provider{Log: log})
		h.reg.AddProvider(postgres.Type, &postgres.Provider{Log: log})
		h.reg.AddProvider(sqlserver.Type, &sqlserver.Provider{Log: log})
		h.reg.AddProvider(mysql.Type, &mysql.Provider{Log: log})
		csvp := &csv.Provider{Log: log, Scratcher: h.dbases, Files: h.files}
		h.reg.AddProvider(csv.TypeCSV, csvp)
		h.reg.AddProvider(csv.TypeTSV, csvp)
		h.files.AddTypeDetectors(csv.DetectCSV, csv.DetectTSV)

		h.reg.AddProvider(xlsx.Type, &xlsx.Provider{Log: log, Scratcher: h.dbases, Files: h.files})
		h.files.AddTypeDetectors(xlsx.DetectXLSX)

		h.addUserDrivers()
	})

	return h.reg
}

// addUserDrivers adds some user drivers to the registry.
func (h *Helper) addUserDrivers() {
	userDriverDefs := DriverDefsFrom(h.T, testsrc.PathDriverDefPpl, testsrc.PathDriverDefRSS)

	// One day we may have more supported user driver genres.
	userDriverImporters := map[string]userdriver.ImportFunc{
		xmlud.Genre: xmlud.Import,
	}

	for _, userDriverDef := range userDriverDefs {
		userDriverDef := userDriverDef

		errs := userdriver.ValidateDriverDef(userDriverDef)
		require.Empty(h.T, errs)

		importFn, ok := userDriverImporters[userDriverDef.Genre]
		require.True(h.T, ok, "unsupported genre %q for user driver %q specified via config",
			userDriverDef.Genre, userDriverDef.Name)

		// For each user driver definition, we register a
		// distinct userdriver.Provider instance.
		udp := &userdriver.Provider{
			Log:       h.Log,
			DriverDef: userDriverDef,
			ImportFn:  importFn,
			Scratcher: h.dbases,
			Files:     h.files,
		}

		h.reg.AddProvider(source.Type(userDriverDef.Name), udp)
		h.files.AddTypeDetectors(udp.TypeDetectors()...)
	}
}

// IsMonotable returns true if src's driver is monotable.
func (h *Helper) IsMonotable(src *source.Source) bool {
	return h.DriverFor(src).DriverMetadata().Monotable
}

// Databases returns the helper's Databases instance.
func (h *Helper) Databases() *driver.Databases {
	_ = h.Registry() // h.dbases is initialized by h.Registry
	return h.dbases
}

// Files returns the helper's Files instance.
func (h *Helper) Files() *source.Files {
	_ = h.Registry() // h.files is initialized by h.Registry
	return h.files
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
		// SkipDiffDB for non-SQL source types
		return
	}

	h.T.Logf("Executing DiffDB for %s", src.Handle) // FIXME: zap this

	beforeDB := h.openNew(src)
	defer h.Log.WarnIfCloseError(beforeDB)

	beforeMeta, err := beforeDB.SourceMetadata(h.Context)
	require.NoError(h.T, err)

	h.Cleanup.Add(func() {
		// Currently DiffDB just checks if the tables and each
		// table's row count match.

		afterDB := h.openNew(src)
		defer h.Log.WarnIfCloseError(afterDB)

		afterMeta, err := afterDB.SourceMetadata(h.Context)
		require.NoError(h.T, err)
		require.Equal(h.T, beforeMeta.TableNames(), afterMeta.TableNames(),
			"diffdb: should have the same set of tables before and after")

		for i, beforeTbl := range beforeMeta.Tables {
			assert.Equal(h.T, beforeTbl.RowCount, afterMeta.Tables[i].RowCount,
				"diffdb: %s: row count for %q is expected to be %d but got %d", src.Handle, beforeTbl.Name, beforeTbl.RowCount, afterMeta.Tables[i].RowCount)
		}
	})
}

func mustLoadSourceSet(t testing.TB) *source.Set {
	hookExpand := func(data []byte) ([]byte, error) {
		// expand vars such as "${SQ_ROOT}"
		return []byte(proj.Expand(string(data))), nil
	}

	fs := &config.YAMLFileStore{Path: proj.Rel(testsrc.PathSrcsConfig), HookLoad: hookExpand}
	cfg, err := fs.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Sources)

	return cfg.Sources
}

// DriverDefsFrom builds DriverDef values from cfg files.
func DriverDefsFrom(t testing.TB, cfgFiles ...string) []*userdriver.DriverDef {
	var userDriverDefs []*userdriver.DriverDef
	for _, f := range cfgFiles {
		ext := &config.Ext{}
		require.NoError(t, yaml.Unmarshal(proj.ReadFile(f), ext))
		userDriverDefs = append(userDriverDefs, ext.UserDrivers...)
	}
	return userDriverDefs
}

// SkipShort invokes t.Skip if testing.Short and arg skip are both true.
func SkipShort(t *testing.T, skip bool) {
	if skip && testing.Short() {
		t.Skip("Skipping long-running test because -short is true.")
	}
}

// Val returns the fully dereferenced value of i. If i
// is nil, nil is returned. If i has type *(*string),
// Val(i) returns string.
// Useful for testing.
func Val(i interface{}) interface{} {
	if i == nil {
		return nil
	}

	v := reflect.ValueOf(i)
	for {
		if !v.IsValid() {
			return nil
		}

		switch v.Kind() {
		default:
			return v.Interface()
		case reflect.Ptr, reflect.Interface:
			if v.IsNil() {
				return nil
			}
			v = v.Elem()
			// Loop again
			continue
		}
	}
}

// TypeDetectors returns the common set of TypeDetectorFuncs.
func TypeDetectors() []source.TypeDetectorFunc {
	return []source.TypeDetectorFunc{source.DetectMagicNumber, xlsx.DetectXLSX, csv.DetectCSV, csv.DetectTSV}
}

// AssertCompareFunc matches several of the the testify/require funcs.
// It can be used to choose assertion comparision funcs in test cases.
type AssertCompareFunc func(require.TestingT, interface{}, interface{}, ...interface{})

// Verify that a sample of the require funcs match AssertCompareFunc.
var (
	_ AssertCompareFunc = require.Equal
	_ AssertCompareFunc = require.GreaterOrEqual
	_ AssertCompareFunc = require.Greater
)

// TName is a convenience function for generating names to
// pass to t.Run.
//
//  t.Run(testh.TName("mytest", 1), func(t *testing.T) {
//
// The most common usage is with test names that are file
// paths.
//
//   testh.TName("path/to/file") --> "path_to_file"
func TName(args ...interface{}) string {
	var strs []string
	var s string
	for _, a := range args {
		s = fmt.Sprintf("%v", a)
		s = strings.Replace(s, "/", "_", -1)
		strs = append(strs, s)
	}

	s = strings.Join(strs, "_")
	if s == "" {
		return "empty"
	}

	return s
}
