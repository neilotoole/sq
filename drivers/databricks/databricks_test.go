// Package databricks_test contains external tests for the Databricks driver.
//
// Most tests run against thriftfake, an in-process fake of a Databricks SQL
// warehouse. That exercises the real databricks-sql-go client, the DSN
// handling, type mapping and record scanning, but not the semantics of SQL:
// the fake returns canned results for the statements it expects. Tests that
// need a real workspace are skipped unless the envar for the @databricks_test
// source is set. See [TestLive].
package databricks_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/databricks"
	"github.com/neilotoole/sq/drivers/databricks/internal/thriftfake"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/tu"
)

const (
	// sparkVersion is the version() the fake reports.
	sparkVersion = "3.5.0 a1b2c3d4e5f6"

	// fakePAT is the access token the fake requires.
	fakePAT = "dapi-fake-token-0123456789"
)

// route maps statements to a result. The first matching route is used.
type route struct {
	match func(stmt string) bool
	res   func(req thriftfake.Request) thriftfake.Result
}

// stmtContains returns a matcher for statements containing sub.
func stmtContains(sub string) func(string) bool {
	return func(stmt string) bool { return strings.Contains(stmt, sub) }
}

// result returns a route function that always returns res.
func result(res thriftfake.Result) func(thriftfake.Request) thriftfake.Result {
	return func(thriftfake.Request) thriftfake.Result { return res }
}

// strCol returns a single-column, single-row string result.
func strCol(name, val string) thriftfake.Result {
	return thriftfake.Result{
		Columns: []thriftfake.Column{{Name: name, Type: thriftfake.TypeString}},
		Rows:    [][]any{{val}},
	}
}

// namesResult returns a single-column string result with one row per name.
func namesResult(col string, names ...string) thriftfake.Result {
	rows := make([][]any, len(names))
	for i, name := range names {
		rows[i] = []any{name}
	}
	return thriftfake.Result{
		Columns: []thriftfake.Column{{Name: col, Type: thriftfake.TypeString}},
		Rows:    rows,
	}
}

// newFake starts a fake warehouse that serves routes, plus the statements that
// every connection issues: SELECT version(), and the ping's "select 1".
// Anything else is a failed statement, which makes an unexpected query visible.
func newFake(t *testing.T, routes ...route) *thriftfake.Server {
	t.Helper()

	routes = append(routes,
		route{
			match: func(stmt string) bool { return stmt == "SELECT version()" },
			res:   result(strCol("version()", sparkVersion)),
		},
		route{
			match: func(stmt string) bool { return strings.EqualFold(stmt, "select 1") },
			res: result(thriftfake.Result{
				Columns: []thriftfake.Column{{Name: "1", Type: thriftfake.TypeInt}},
				Rows:    [][]any{{int32(1)}},
			}),
		},
	)

	return thriftfake.New(t, func(req thriftfake.Request) thriftfake.Result {
		for _, r := range routes {
			if r.match(req.Statement) {
				return r.res(req)
			}
		}
		t.Logf("fake warehouse: unexpected statement: %s", req.Statement)
		return thriftfake.Result{
			ErrMessage:  "[UNEXPECTED_STATEMENT] the fake has no route for: " + req.Statement,
			ErrSQLState: "XX000",
		}
	}, thriftfake.WithToken(fakePAT))
}

// fakeLocation returns the sq location for srv. The query string may be empty.
func fakeLocation(srv *thriftfake.Server, query string) string {
	loc := fmt.Sprintf("databricks://token:%s@%s/sql/1.0/warehouses/fake", fakePAT, srv.HostPort())
	// Telemetry is the driver's own concern, and there's nothing to
	// receive it here.
	loc += "?enableTelemetry=false"
	if query != "" {
		loc += "&" + query
	}
	return loc
}

// fakeSource returns a source for srv, with handle @dbx_fake.
func fakeSource(srv *thriftfake.Server) *source.Source {
	return &source.Source{
		Handle:   "@dbx_fake",
		Type:     databricks.Type,
		Location: fakeLocation(srv, ""),
	}
}

// openFake opens a grip for a source on srv.
func openFake(t *testing.T, srv *thriftfake.Server) (*testh.Helper, driver.Grip, driver.SQLDriver, *sql.DB) {
	t.Helper()

	th := testh.New(t)
	grip := th.Open(fakeSource(srv))
	db, err := grip.DB(th.Context)
	require.NoError(t, err)
	return th, grip, grip.SQLDriver(), db
}

func newDriver(t *testing.T) driver.SQLDriver {
	t.Helper()

	p := &databricks.Provider{}
	drvr, err := p.DriverFor(databricks.Type)
	require.NoError(t, err)
	sqlDrvr, ok := drvr.(driver.SQLDriver)
	require.True(t, ok)
	return sqlDrvr
}

func TestProvider_DriverFor(t *testing.T) {
	p := &databricks.Provider{}

	drvr, err := p.DriverFor(databricks.Type)
	require.NoError(t, err)
	require.NotNil(t, drvr)

	drvr, err = p.DriverFor("postgres")
	require.Error(t, err)
	require.Nil(t, drvr)
}

func TestDriverMetadata(t *testing.T) {
	drvr := newDriver(t)

	md := drvr.DriverMetadata()
	require.Equal(t, databricks.Type, md.Type)
	require.Equal(t, "databricks", md.Type.String())
	require.True(t, md.IsSQL)
	require.False(t, md.IsEmbeddedSQL)
	require.False(t, md.Monotable)
	require.Equal(t, 443, md.DefaultPort)

	require.NoError(t, drvr.LocationShape().Validate())
	require.NotEmpty(t, drvr.ConnParams())
}

func TestDialect(t *testing.T) {
	dlct := newDriver(t).Dialect()

	require.Equal(t, databricks.Type, dlct.Type)
	require.True(t, dlct.Catalog, "Databricks has catalog.schema.table")
	require.True(t, dlct.IsRowsAffectedUnsupported)
	require.False(t, dlct.IntBool)
	require.False(t, dlct.SingleWriter)
	require.Positive(t, dlct.MaxBatchValues)
	require.NotEmpty(t, dlct.Joins)
	require.NotNil(t, dlct.ExecModeFor)

	require.Equal(t, "`actor`", dlct.Enquote("actor"))
	require.Equal(t, "`we``ird`", dlct.Enquote("we`ird"))
	require.Equal(t, "`with space`", dlct.Enquote("with space"))

	require.Equal(t, `'O\'Brien'`, dlct.QuoteLiteral("O'Brien"))
	require.Equal(t, "(?, ?), (?, ?)", dlct.Placeholders(2, 2))

	mode, err := dlct.ExecModeFor("SELECT 1")
	require.NoError(t, err)
	require.Equal(t, "query", string(mode))
	mode, err = dlct.ExecModeFor("SHOW TABLES")
	require.NoError(t, err)
	require.Equal(t, "query", string(mode))
	mode, err = dlct.ExecModeFor("CREATE TABLE t (a INT)")
	require.NoError(t, err)
	require.Equal(t, "exec", string(mode))
}

func TestValidateSource(t *testing.T) {
	drvr := newDriver(t)
	good := "databricks://token:" + testPAT + "@" + testHost + "/sql/1.0/warehouses/abc123"

	src := &source.Source{Handle: "@dbx", Type: databricks.Type, Location: good}
	got, err := drvr.ValidateSource(src)
	require.NoError(t, err)
	require.Equal(t, good, got.Location, "the location must be left as typed")

	// A placeholder can't be parsed until it's resolved.
	src = &source.Source{
		Handle:   "@dbx",
		Type:     databricks.Type,
		Location: "databricks://token:${env:DBX_PAT}@" + testHost + "/sql/1.0/warehouses/abc123",
	}
	_, err = drvr.ValidateSource(src)
	require.NoError(t, err)

	src = &source.Source{Handle: "@dbx", Type: "postgres", Location: good}
	_, err = drvr.ValidateSource(src)
	require.Error(t, err)

	src = &source.Source{Handle: "@dbx", Type: databricks.Type, Location: "databricks://token:" + testPAT + "@" + testHost}
	_, err = drvr.ValidateSource(src)
	require.Error(t, err)
	require.ErrorContains(t, err, "no HTTP path")
	require.NotContains(t, err.Error(), testPAT)
}

func TestPing(t *testing.T) {
	srv := newFake(t)
	drvr := newDriver(t)
	ctx := context.Background()

	err := drvr.Ping(ctx, fakeSource(srv), driver.ModeReadOnly)
	require.NoError(t, err)
	require.Equal(t, []string{"select 1"}, srv.Statements())
}

func TestPing_BadToken(t *testing.T) {
	srv := newFake(t)
	drvr := newDriver(t)

	const badPAT = "dapi-wrong-token-9876543210"
	src := &source.Source{
		Handle:   "@dbx_fake",
		Type:     databricks.Type,
		Location: strings.Replace(fakeLocation(srv, ""), fakePAT, badPAT, 1),
	}

	err := drvr.Ping(context.Background(), src, driver.ModeReadOnly)
	require.Error(t, err)
	require.NotContains(t, err.Error(), badPAT, "error must not leak the token")
	require.Empty(t, srv.Statements(), "the fake must have rejected the request")
}

// TestOpen_CatalogSchema tests that the catalog and schema, from the location
// or from the source (as set by --src.schema), are the session's initial
// namespace.
func TestOpen_CatalogSchema(t *testing.T) {
	srv := newFake(t)
	drvr := newDriver(t)
	ctx := context.Background()

	// From the location.
	src := &source.Source{
		Handle:   "@dbx_fake",
		Type:     databricks.Type,
		Location: fakeLocation(srv, "catalog=main&schema=sakila"),
	}
	require.NoError(t, drvr.Ping(ctx, src, driver.ModeReadOnly))
	require.Equal(t, thriftfake.Namespace{Catalog: "main", Schema: "sakila"}, srv.Sessions()[0])

	// The source's catalog and schema win over those of the location.
	src.Catalog = "samples"
	src.Schema = "nyctaxi"
	require.NoError(t, drvr.Ping(ctx, src, driver.ModeReadOnly))
	require.Equal(t, thriftfake.Namespace{Catalog: "samples", Schema: "nyctaxi"}, srv.Sessions()[1])

	// With neither, the workspace defaults apply.
	src = fakeSource(srv)
	require.NoError(t, drvr.Ping(ctx, src, driver.ModeReadOnly))
	require.Equal(t, thriftfake.Namespace{}, srv.Sessions()[2])
}

func TestOpen_DBSemver(t *testing.T) {
	srv := newFake(t)
	th, grip, drvr, db := openFake(t, srv)

	semver, err := grip.DBSemver(th.Context)
	require.NoError(t, err)
	require.Equal(t, "v3.5.0", semver)
	require.Equal(t, 1, countStmts(srv, "SELECT version()"))

	semver, err = drvr.DBSemver(th.Context, db)
	require.NoError(t, err)
	require.Equal(t, "v3.5.0", semver)

	// The grip memoizes the version, which the opening ping fetched. Only
	// the direct driver call went to the warehouse again.
	require.Equal(t, 2, countStmts(srv, "SELECT version()"))
}

// countStmts returns the number of statements that srv has received that
// are equal to stmt.
func countStmts(srv *thriftfake.Server, stmt string) int {
	var n int
	for _, got := range srv.Statements() {
		if got == stmt {
			n++
		}
	}
	return n
}

// allTypesResult returns a result with a column of each Databricks type that
// databricks-sql-go can report, and two rows: one of values, and one of NULLs.
func allTypesResult() thriftfake.Result {
	cols := []thriftfake.Column{
		{Name: "c_boolean", Type: thriftfake.TypeBoolean},
		{Name: "c_tinyint", Type: thriftfake.TypeTinyInt},
		{Name: "c_smallint", Type: thriftfake.TypeSmallInt},
		{Name: "c_int", Type: thriftfake.TypeInt},
		{Name: "c_bigint", Type: thriftfake.TypeBigInt},
		{Name: "c_float", Type: thriftfake.TypeFloat},
		{Name: "c_double", Type: thriftfake.TypeDouble},
		{Name: "c_string", Type: thriftfake.TypeString},
		{Name: "c_varchar", Type: thriftfake.TypeVarchar},
		{Name: "c_char", Type: thriftfake.TypeChar},
		{Name: "c_decimal", Type: thriftfake.TypeDecimal},
		{Name: "c_binary", Type: thriftfake.TypeBinary},
		{Name: "c_date", Type: thriftfake.TypeDate},
		{Name: "c_timestamp", Type: thriftfake.TypeTimestamp},
		{Name: "c_array", Type: thriftfake.TypeArray},
		{Name: "c_map", Type: thriftfake.TypeMap},
		{Name: "c_struct", Type: thriftfake.TypeStruct},
		{Name: "c_interval", Type: thriftfake.TypeIntervalDayTime},
		{Name: "c_null", Type: thriftfake.TypeNull},
	}

	vals := []any{
		true, int8(-8), int16(300), int32(70000), int64(1) << 40, 1.5, 2.25,
		"héllo", "vc", "ch", "12345.6700",
		[]byte{1, 2, 3},
		"2024-02-29", "2024-02-29 13:14:15.123456",
		"[1,2,3]", `{"a":1}`, `{"x":1,"y":"z"}`, "1 02:03:04.000000000", nil,
	}
	nulls := make([]any, len(cols))

	return thriftfake.Result{Columns: cols, Rows: [][]any{vals, nulls}}
}

func TestQuery_AllTypes(t *testing.T) {
	srv := newFake(t, route{
		match: stmtContains("FROM all_types"),
		res:   result(allTypesResult()),
	})
	th, _, drvr, db := openFake(t, srv)
	ctx := th.Context

	rows, err := db.QueryContext(ctx, "SELECT * FROM all_types")
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

	colTypes, err := rows.ColumnTypes()
	require.NoError(t, err)
	recMeta, newRecFn, err := drvr.RecordMeta(ctx, colTypes, nil)
	require.NoError(t, err)

	wantKinds := []kind.Kind{
		kind.Bool, kind.Int, kind.Int, kind.Int, kind.Int, kind.Float, kind.Float,
		kind.Text, kind.Text, kind.Text, kind.Decimal, kind.Bytes,
		kind.Date, kind.Datetime,
		kind.Text, kind.Text, kind.Text, kind.Text, kind.Null,
	}
	require.Equal(t, wantKinds, recMeta.Kinds())

	var recs []record.Record
	for rows.Next() {
		scanRow := recMeta.NewScanRow()
		require.NoError(t, rows.Scan(scanRow...))
		rec, err := newRecFn(scanRow)
		require.NoError(t, err)
		_, err = record.Valid(rec)
		require.NoError(t, err, "record must hold only sq-valid types")
		recs = append(recs, rec)
	}
	require.NoError(t, rows.Err())
	require.Len(t, recs, 2)

	wantDec, err := decimal.NewFromString("12345.67")
	require.NoError(t, err)

	got := recs[0]
	require.Equal(t, true, got[0])
	require.Equal(t, int64(-8), got[1])
	require.Equal(t, int64(300), got[2])
	require.Equal(t, int64(70000), got[3])
	require.Equal(t, int64(1)<<40, got[4])
	require.Equal(t, 1.5, got[5])
	require.Equal(t, 2.25, got[6])
	require.Equal(t, "héllo", got[7])
	require.Equal(t, "vc", got[8])
	require.Equal(t, "ch", got[9])
	gotDec, ok := got[10].(decimal.Decimal)
	require.True(t, ok, "got %T", got[10])
	require.True(t, wantDec.Equal(gotDec), "want %s, got %s", wantDec, gotDec)
	require.Equal(t, []byte{1, 2, 3}, got[11])
	require.Equal(t, time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC), got[12])
	require.Equal(t, time.Date(2024, 2, 29, 13, 14, 15, 123456000, time.UTC), got[13])
	require.Equal(t, "[1,2,3]", got[14])
	require.Equal(t, `{"a":1}`, got[15])
	require.Equal(t, `{"x":1,"y":"z"}`, got[16])
	require.Equal(t, "1 02:03:04.000000000", got[17])
	require.Nil(t, got[18])

	for i, v := range recs[1] {
		require.Nil(t, v, "column %d (%s) of the NULL row", i, recMeta[i].Name())
	}
}

func TestQuery_EmptyResult(t *testing.T) {
	srv := newFake(t, route{
		match: stmtContains("FROM empty"),
		res: result(thriftfake.Result{
			Columns: []thriftfake.Column{
				{Name: "a", Type: thriftfake.TypeInt},
				{Name: "b", Type: thriftfake.TypeString},
			},
		}),
	})
	th, _, drvr, db := openFake(t, srv)

	rows, err := db.QueryContext(th.Context, "SELECT * FROM empty")
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

	colTypes, err := rows.ColumnTypes()
	require.NoError(t, err)
	recMeta, _, err := drvr.RecordMeta(th.Context, colTypes, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, recMeta.Names())
	require.Equal(t, []kind.Kind{kind.Int, kind.Text}, recMeta.Kinds())

	require.False(t, rows.Next())
	require.NoError(t, rows.Err())
}

func TestQuery_Error(t *testing.T) {
	srv := newFake(t, route{
		match: stmtContains("FROM nope"),
		res: result(thriftfake.Result{
			ErrMessage:  "[TABLE_OR_VIEW_NOT_FOUND] The table or view `nope` cannot be found.",
			ErrSQLState: "42P01",
		}),
	})
	th, _, drvr, db := openFake(t, srv)

	_, err := db.QueryContext(th.Context, "SELECT * FROM nope")
	require.Error(t, err)
	err = drvr.ErrWrapFunc()(err)
	require.Error(t, err)

	var notExistErr *driver.NotExistError
	require.True(t, errors.As(err, &notExistErr), "got %T: %v", err, err)
	require.ErrorContains(t, err, "TABLE_OR_VIEW_NOT_FOUND")

	// Another error is not a NotExistError.
	srv = newFake(t, route{
		match: stmtContains("FORM"),
		res: result(thriftfake.Result{
			ErrMessage:  "[PARSE_SYNTAX_ERROR] Syntax error at or near 'FORM'.",
			ErrSQLState: "42601",
		}),
	})
	th, _, drvr, db = openFake(t, srv)
	_, err = db.QueryContext(th.Context, "SELECT * FORM x")
	require.Error(t, err)
	err = drvr.ErrWrapFunc()(err)
	require.False(t, errors.As(err, &notExistErr), "got %T: %v", err, err)
	require.ErrorContains(t, err, "PARSE_SYNTAX_ERROR")
}

func TestTableColumnTypes(t *testing.T) {
	srv := newFake(t, route{
		match: stmtContains("LIMIT 0"),
		res: result(thriftfake.Result{
			Columns: []thriftfake.Column{
				{Name: "actor_id", Type: thriftfake.TypeBigInt},
				{Name: "first_name", Type: thriftfake.TypeString},
			},
		}),
	})
	th, _, drvr, db := openFake(t, srv)

	colTypes, err := drvr.TableColumnTypes(th.Context, db, "actor", nil)
	require.NoError(t, err)
	require.Len(t, colTypes, 2)
	require.Equal(t, "actor_id", colTypes[0].Name())
	require.Equal(t, "BIGINT", colTypes[0].DatabaseTypeName())

	_, err = drvr.TableColumnTypes(th.Context, db, "actor", []string{"first_name", "we`ird"})
	require.NoError(t, err)

	stmts := srv.Statements()
	require.Contains(t, stmts, "SELECT * FROM `actor` LIMIT 0")
	require.Contains(t, stmts, "SELECT `first_name`, `we``ird` FROM `actor` LIMIT 0")
}

// currentNS is the route for the current catalog and schema.
func currentNS(catalog, schema string) route {
	return route{
		match: stmtContains("SELECT current_catalog(), current_schema()"),
		res: result(thriftfake.Result{
			Columns: []thriftfake.Column{
				{Name: "current_catalog()", Type: thriftfake.TypeString},
				{Name: "current_schema()", Type: thriftfake.TypeString},
			},
			Rows: [][]any{{catalog, schema}},
		}),
	}
}

func TestCatalogsAndSchemas(t *testing.T) {
	srv := newFake(t,
		route{match: stmtContains("SELECT current_catalog()"), res: result(strCol("current_catalog()", "main"))},
		route{match: stmtContains("SELECT current_schema()"), res: result(strCol("current_schema()", "sakila"))},
		route{
			match: stmtContains("SHOW CATALOGS"),
			res:   result(namesResult("catalog", "samples", "main", "hive_metastore", "system")),
		},
		route{
			match: stmtContains("SHOW SCHEMAS"),
			res:   result(namesResult("databaseName", "sakila", "default", "information_schema")),
		},
	)
	th, _, drvr, db := openFake(t, srv)
	ctx := th.Context

	catalog, err := drvr.CurrentCatalog(ctx, db)
	require.NoError(t, err)
	require.Equal(t, "main", catalog)

	schema, err := drvr.CurrentSchema(ctx, db)
	require.NoError(t, err)
	require.Equal(t, "sakila", schema)

	// The current catalog is first, and the rest are sorted.
	catalogs, err := drvr.ListCatalogs(ctx, db)
	require.NoError(t, err)
	require.Equal(t, []string{"main", "hive_metastore", "samples", "system"}, catalogs)

	schemas, err := drvr.ListSchemas(ctx, db)
	require.NoError(t, err)
	require.Equal(t, []string{"default", "information_schema", "sakila"}, schemas)

	schemaMetas, err := drvr.ListSchemaMetadata(ctx, db)
	require.NoError(t, err)
	require.Len(t, schemaMetas, 3)
	require.Equal(t, "default", schemaMetas[0].Name)
	require.Equal(t, "main", schemaMetas[0].Catalog)

	// Databricks identifiers are case-insensitive.
	ok, err := drvr.CatalogExists(ctx, db, "MAIN")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = drvr.CatalogExists(ctx, db, "nope")
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = drvr.CatalogExists(ctx, db, "")
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = drvr.SchemaExists(ctx, db, "Sakila")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = drvr.SchemaExists(ctx, db, "nope")
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = drvr.SchemaExists(ctx, db, "")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestListTableNames(t *testing.T) {
	srv := newFake(t,
		currentNS("main", "sakila"),
		route{
			match: stmtContains("`main`.information_schema.tables"),
			res: func(req thriftfake.Request) thriftfake.Result {
				// Which names come back depends on the table_type filter,
				// just as it would on a real warehouse.
				switch {
				case strings.Contains(req.Statement, "NOT IN"):
					return namesResult("table_name", "actor", "film")
				case strings.Contains(req.Statement, "table_type IN"):
					return namesResult("table_name", "v_actor")
				default:
					return namesResult("table_name", "actor", "film", "v_actor")
				}
			},
		},
	)
	th, _, drvr, db := openFake(t, srv)
	ctx := th.Context

	names, err := drvr.ListTableNames(ctx, db, "", true, false)
	require.NoError(t, err)
	require.Equal(t, []string{"actor", "film"}, names)

	names, err = drvr.ListTableNames(ctx, db, "", false, true)
	require.NoError(t, err)
	require.Equal(t, []string{"v_actor"}, names)

	names, err = drvr.ListTableNames(ctx, db, "other", true, true)
	require.NoError(t, err)
	require.Equal(t, []string{"actor", "film", "v_actor"}, names)

	names, err = drvr.ListTableNames(ctx, db, "", false, false)
	require.NoError(t, err)
	require.Empty(t, names)

	// The schema is bound as a parameter, not interpolated: the current
	// schema when none is given, else the schema that was passed.
	var schemaParams []string
	for _, req := range srv.Requests() {
		if strings.Contains(req.Statement, "information_schema.tables") {
			require.Len(t, req.Params, 1)
			schemaParams = append(schemaParams, req.Params[0])
		}
	}
	require.Equal(t, []string{"sakila", "sakila", "other"}, schemaParams)

	ok, err := drvr.TableExists(ctx, db, "ACTOR")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = drvr.TableExists(ctx, db, "nope")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestListTableNames_HiveMetastore(t *testing.T) {
	srv := newFake(t, currentNS("hive_metastore", "default"))
	th, _, drvr, db := openFake(t, srv)

	_, err := drvr.ListTableNames(th.Context, db, "", true, true)
	require.Error(t, err)
	require.ErrorContains(t, err, "hive_metastore")
	require.ErrorContains(t, err, "information_schema")
}

// metadataRoutes returns the routes for source metadata of main.sakila.
func metadataRoutes() []route {
	str := func(name string) thriftfake.Column { return thriftfake.Column{Name: name, Type: thriftfake.TypeString} }

	return []route{
		{
			match: stmtContains("current_user()"),
			res: result(thriftfake.Result{
				Columns: []thriftfake.Column{
					str("version()"), str("current_catalog()"), str("current_schema()"), str("current_user()"),
				},
				Rows: [][]any{{sparkVersion, "main", "sakila", "alice@example.com"}},
			}),
		},
		currentNS("main", "sakila"),
		{
			match: stmtContains("`main`.information_schema.tables"),
			res: result(thriftfake.Result{
				Columns: []thriftfake.Column{str("table_name"), str("table_type"), str("comment")},
				Rows: [][]any{
					{"actor", "MANAGED", "People who act"},
					{"film", "EXTERNAL", nil},
					{"mv_actor", "MATERIALIZED_VIEW", nil},
					{"v_actor", "VIEW", nil},
				},
			}),
		},
		{
			match: stmtContains("`main`.information_schema.columns"),
			res: result(thriftfake.Result{
				Columns: []thriftfake.Column{
					str("table_name"), str("column_name"),
					{Name: "ordinal_position", Type: thriftfake.TypeInt},
					str("is_nullable"), str("data_type"), str("full_data_type"), str("column_default"), str("comment"),
				},
				Rows: [][]any{
					{"actor", "actor_id", int32(0), "NO", "LONG", "bigint", nil, "The PK"},
					{"actor", "first_name", int32(1), "YES", "STRING", "string", nil, nil},
					{"actor", "last_update", int32(2), "YES", "TIMESTAMP", "timestamp", "current_timestamp()", nil},
					{"film", "film_id", int32(0), "NO", "LONG", "bigint", nil, nil},
					{"film", "rental_rate", int32(1), "YES", "DECIMAL", "decimal(4,2)", nil, nil},
					{"film", "tags", int32(2), "YES", "ARRAY", "array<string>", nil, nil},
					{"mv_actor", "actor_id", int32(0), "YES", "LONG", "bigint", nil, nil},
					{"v_actor", "actor_id", int32(0), "YES", "LONG", "bigint", nil, nil},
					{"v_actor", "first_name", int32(1), "YES", "STRING", "string", nil, nil},
				},
			}),
		},
		{
			match: stmtContains("table_constraints"),
			res: result(thriftfake.Result{
				Columns: []thriftfake.Column{str("table_name"), str("column_name")},
				Rows:    [][]any{{"actor", "actor_id"}, {"film", "film_id"}},
			}),
		},
		{
			match: stmtContains("`main`.information_schema.views"),
			res: result(thriftfake.Result{
				Columns: []thriftfake.Column{str("table_name"), str("view_definition")},
				Rows: [][]any{
					{"v_actor", "SELECT actor_id, first_name FROM actor"},
					{"mv_actor", "SELECT actor_id FROM actor"},
				},
			}),
		},
		{
			match: stmtContains("SELECT COUNT(*) FROM"),
			res: func(req thriftfake.Request) thriftfake.Result {
				counts := map[string]int64{
					"`main`.`sakila`.`actor`": 200,
					"`main`.`sakila`.`film`":  1000,
				}
				count := int64(-1)
				for tbl, n := range counts {
					if strings.HasSuffix(req.Statement, tbl) {
						count = n
					}
				}
				if count < 0 {
					// Like a table that the user can't SELECT from.
					return thriftfake.Result{
						ErrMessage:  "[INSUFFICIENT_PERMISSIONS] User does not have SELECT on table.",
						ErrSQLState: "42501",
					}
				}
				return thriftfake.Result{
					Columns: []thriftfake.Column{{Name: "count(1)", Type: thriftfake.TypeBigInt}},
					Rows:    [][]any{{count}},
				}
			},
		},
	}
}

func TestSourceMetadata(t *testing.T) {
	srv := newFake(t, metadataRoutes()...)
	th, grip, _, _ := openFake(t, srv)

	md, err := grip.SourceMetadata(th.Context, false)
	require.NoError(t, err)

	require.Equal(t, "@dbx_fake", md.Handle)
	require.Equal(t, databricks.Type, md.Driver)
	require.Equal(t, databricks.Type, md.DBDriver)
	require.Equal(t, sparkVersion, md.DBVersion)
	require.Equal(t, "v3.5.0", md.DBSemver)
	require.Equal(t, "Databricks "+sparkVersion, md.DBProduct)
	require.Equal(t, "main", md.Catalog)
	require.Equal(t, "sakila", md.Schema)
	require.Equal(t, "main", md.Name)
	require.Equal(t, "main.sakila", md.FQName)
	require.Equal(t, "alice@example.com", md.User)
	require.Equal(t, "main", md.DBProperties["catalog"])
	require.Equal(t, sparkVersion, md.DBProperties["version"])

	require.Equal(t, int64(2), md.TableCount)
	require.Equal(t, int64(2), md.ViewCount)
	require.Equal(t, []string{"actor", "film", "mv_actor", "v_actor"}, md.TableNames())

	actor := md.Table("actor")
	require.NotNil(t, actor)
	require.Equal(t, "main.sakila.actor", actor.FQName)
	require.Equal(t, "table", actor.TableType)
	require.Equal(t, "MANAGED", actor.DBTableType)
	require.Equal(t, "People who act", actor.Comment)
	require.Equal(t, int64(200), actor.RowCount)
	require.Empty(t, actor.ViewDefinition)
	require.Len(t, actor.Columns, 3)

	id := actor.Columns[0]
	require.Equal(t, "actor_id", id.Name)
	require.Equal(t, int64(0), id.Position)
	require.Equal(t, kind.Int, id.Kind)
	require.Equal(t, "LONG", id.BaseType)
	require.Equal(t, "bigint", id.ColumnType)
	require.False(t, id.Nullable)
	require.True(t, id.PrimaryKey)
	require.Equal(t, "The PK", id.Comment)

	require.Equal(t, kind.Text, actor.Columns[1].Kind)
	require.True(t, actor.Columns[1].Nullable)
	require.False(t, actor.Columns[1].PrimaryKey)

	require.Equal(t, kind.Datetime, actor.Columns[2].Kind)
	require.Equal(t, "current_timestamp()", actor.Columns[2].DefaultValue)

	film := md.Table("film")
	require.NotNil(t, film)
	require.Equal(t, "EXTERNAL", film.DBTableType)
	require.Equal(t, int64(1000), film.RowCount)
	require.Equal(t, kind.Decimal, film.Columns[1].Kind)
	require.Equal(t, "decimal(4,2)", film.Columns[1].ColumnType)
	require.Equal(t, kind.Text, film.Columns[2].Kind, "an ARRAY column is surfaced as text")
	require.True(t, film.Columns[0].PrimaryKey)

	view := md.Table("v_actor")
	require.NotNil(t, view)
	require.Equal(t, "view", view.TableType)
	require.Equal(t, "SELECT actor_id, first_name FROM actor", view.ViewDefinition)
	require.Len(t, view.Columns, 2)
	require.Zero(t, view.RowCount, "the fake denies SELECT on this view, so the row count is left at zero")

	mv := md.Table("mv_actor")
	require.NotNil(t, mv)
	require.Equal(t, "materialized_view", mv.TableType)
	require.Equal(t, "SELECT actor_id FROM actor", mv.ViewDefinition)

	// All information_schema queries bind the schema as a parameter.
	for _, req := range srv.Requests() {
		if strings.Contains(req.Statement, "information_schema") {
			require.Equal(t, []string{"sakila"}, req.Params, req.Statement)
		}
	}
}

func TestSourceMetadata_NoSchema(t *testing.T) {
	srv := newFake(t, metadataRoutes()...)
	th, grip, _, _ := openFake(t, srv)

	md, err := grip.SourceMetadata(th.Context, true)
	require.NoError(t, err)
	require.Equal(t, "main.sakila", md.FQName)
	require.Empty(t, md.Tables)

	for _, stmt := range srv.Statements() {
		require.NotContains(t, stmt, "information_schema", "noSchema must not read the schema")
	}
}

func TestTableMetadata(t *testing.T) {
	srv := newFake(t, metadataRoutes()...)
	th, grip, _, _ := openFake(t, srv)

	tbl, err := grip.TableMetadata(th.Context, "film")
	require.NoError(t, err)
	require.Equal(t, "film", tbl.Name)
	require.Len(t, tbl.Columns, 3)

	// Only the requested table is counted, not every table in the schema.
	var counts []string
	for _, stmt := range srv.Statements() {
		if strings.Contains(stmt, "SELECT COUNT(*) FROM") {
			counts = append(counts, stmt)
		}
	}
	require.Equal(t, []string{"SELECT COUNT(*) FROM `main`.`sakila`.`film`"}, counts)

	_, err = grip.TableMetadata(th.Context, "nope")
	require.Error(t, err)
	var notExistErr *driver.NotExistError
	require.True(t, errors.As(err, &notExistErr), "got %T: %v", err, err)
}

func TestSourceMetadata_HiveMetastore(t *testing.T) {
	srv := newFake(t, route{
		match: stmtContains("current_user()"),
		res: result(thriftfake.Result{
			Columns: []thriftfake.Column{
				{Name: "a", Type: thriftfake.TypeString},
				{Name: "b", Type: thriftfake.TypeString},
				{Name: "c", Type: thriftfake.TypeString},
				{Name: "d", Type: thriftfake.TypeString},
			},
			Rows: [][]any{{sparkVersion, "hive_metastore", "default", "alice@example.com"}},
		}),
	})
	th, grip, _, _ := openFake(t, srv)

	// Without the schema, hive_metastore is fine.
	md, err := grip.SourceMetadata(th.Context, true)
	require.NoError(t, err)
	require.Equal(t, "hive_metastore", md.Catalog)

	_, err = grip.SourceMetadata(th.Context, false)
	require.Error(t, err)
	require.ErrorContains(t, err, "hive_metastore")
}

// TestSLQ2SQL renders SLQ to Databricks SQL through the real query pipeline,
// against a source on the fake.
func TestSLQ2SQL(t *testing.T) {
	srv := newFake(t)
	th := testh.New(t)

	coll := &source.Collection{}
	require.NoError(t, coll.Add(fakeSource(srv)))
	_, err := coll.SetActive("@dbx_fake", false)
	require.NoError(t, err)
	qc := &libsq.QueryContext{Collection: coll, Grips: th.Grips()}

	testCases := []struct {
		in   string
		want string
	}{
		{`.actor`, "SELECT * FROM `actor`"},
		{`.actor | .first_name, .last_name`, "SELECT `first_name`, `last_name` FROM `actor`"},
		{`.actor | .[3:6]`, "SELECT * FROM `actor` LIMIT 3 OFFSET 3"},
		{`.actor | .[:5]`, "SELECT * FROM `actor` LIMIT 5 OFFSET 0"},
		{`.actor | .[3:]`, "SELECT * FROM `actor` OFFSET 3"},
		{`.actor | where(.actor_id > 5)`, "SELECT * FROM `actor` WHERE `actor_id` > 5"},
		{`.actor | where(.first_name == "BOB")`, "SELECT * FROM `actor` WHERE `first_name` = 'BOB'"},
		{`.actor | order_by(.last_name)`, "SELECT * FROM `actor` ORDER BY `last_name`"},
		{`.actor | count`, "SELECT count(*) AS `count` FROM `actor`"},
		{`.actor | .first_name | unique`, "SELECT DISTINCT `first_name` FROM `actor`"},

		// String literals use Spark's backslash escaping.
		{`.actor | where(.first_name == "O'Brien")`, "SELECT * FROM `actor` WHERE `first_name` = 'O\\'Brien'"},
		{`.actor | where(.first_name == "a\\b")`, "SELECT * FROM `actor` WHERE `first_name` = 'a\\\\b'"},
		{`.actor | where(.first_name == "\\' OR 1=1 --")`, "SELECT * FROM `actor` WHERE `first_name` = '\\\\\\' OR 1=1 --'"},
		{
			`.actor | where(contains(.first_name, "50%"))`,
			"SELECT * FROM `actor` WHERE `first_name` LIKE '%50|%%' ESCAPE '|'",
		},
		{
			`.actor | where(startswith(.first_name, "it's"))`,
			"SELECT * FROM `actor` WHERE `first_name` LIKE 'it\\'s%' ESCAPE '|'",
		},

		// Functions.
		{`.payment | sum(.amount)`, "SELECT CAST(sum(`amount`) AS DECIMAL(38, 6)) AS `sum(.amount)` FROM `payment`"},
		{`.actor | .first_name | schema()`, "SELECT current_schema() AS `schema()` FROM `actor`"},
		{`.actor | .first_name | catalog()`, "SELECT current_catalog() AS `catalog()` FROM `actor`"},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			res, err := libsq.SLQ2SQL(th.Context, qc, tc.in)
			require.NoError(t, err)
			require.Equal(t, databricks.Type, res.Dialect)
			require.Equal(t, tc.want, res.SQL)
		})
	}
}

// TestSLQ2SQL_Args tests that a query variable (--arg), which is rendered by
// a different path from a literal in the query, is quoted for Spark too.
func TestSLQ2SQL_Args(t *testing.T) {
	srv := newFake(t)
	th := testh.New(t)

	coll := &source.Collection{}
	require.NoError(t, coll.Add(fakeSource(srv)))
	_, err := coll.SetActive("@dbx_fake", false)
	require.NoError(t, err)

	qc := &libsq.QueryContext{
		Collection: coll,
		Grips:      th.Grips(),
		Args:       map[string]string{"name": `O'Brien\`},
	}

	res, err := libsq.SLQ2SQL(th.Context, qc, `.actor | where(.first_name == $name)`)
	require.NoError(t, err)
	require.Equal(t, "SELECT * FROM `actor` WHERE `first_name` = 'O\\'Brien\\\\'", res.SQL)
}

// TestExecSLQ runs a SLQ query end to end: render, execute against the fake,
// and write the records.
func TestExecSLQ(t *testing.T) {
	srv := newFake(t, route{
		match: stmtContains("SELECT * FROM `actor` LIMIT 2 OFFSET 0"),
		res: result(thriftfake.Result{
			Columns: []thriftfake.Column{
				{Name: "actor_id", Type: thriftfake.TypeBigInt},
				{Name: "first_name", Type: thriftfake.TypeString},
				{Name: "last_update", Type: thriftfake.TypeTimestamp},
			},
			Rows: [][]any{
				{int64(1), "PENELOPE", "2024-01-02 03:04:05"},
				{int64(2), nil, nil},
			},
		}),
	})
	th := testh.New(t)
	th.Add(fakeSource(srv))

	sink, err := th.QuerySLQ(`@dbx_fake.actor | .[:2]`, nil)
	require.NoError(t, err)
	require.Len(t, sink.Recs, 2)
	require.Equal(t, []string{"actor_id", "first_name", "last_update"}, sink.RecMeta.Names())
	require.Equal(t, int64(1), sink.Recs[0][0])
	require.Equal(t, "PENELOPE", sink.Recs[0][1])
	require.Equal(t, time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), sink.Recs[0][2])
	require.Equal(t, int64(2), sink.Recs[1][0])
	require.Nil(t, sink.Recs[1][1])
	require.Nil(t, sink.Recs[1][2])
}

// TestReadOnly tests that every method that would modify the source returns an
// error, without sending anything to the warehouse.
func TestReadOnly(t *testing.T) {
	srv := newFake(t)
	th, _, drvr, db := openFake(t, srv)
	ctx := th.Context
	before := len(srv.Statements())

	src := fakeSource(srv)
	tbl := tablefq.T{Table: "actor"}

	checks := map[string]func() error{
		"PrepareInsertStmt": func() error {
			_, err := drvr.PrepareInsertStmt(ctx, db, "actor", []string{"a"}, 1)
			return err
		},
		"NewBatchInsert": func() error {
			_, err := drvr.NewBatchInsert(ctx, "msg", db, src, "actor", []string{"a"})
			return err
		},
		"PrepareUpdateStmt": func() error {
			_, err := drvr.PrepareUpdateStmt(ctx, db, "actor", []string{"a"}, "id = ?")
			return err
		},
		"CreateTable":  func() error { return drvr.CreateTable(ctx, db, nil) },
		"CreateSchema": func() error { return drvr.CreateSchema(ctx, db, "s") },
		"DropSchema":   func() error { return drvr.DropSchema(ctx, db, "s") },
		"Truncate": func() error {
			_, err := drvr.Truncate(ctx, src, "actor", false)
			return err
		},
		"CopyTable": func() error {
			_, err := drvr.CopyTable(ctx, db, tbl, tablefq.T{Table: "actor2"}, true)
			return err
		},
		"DropTable":              func() error { return drvr.DropTable(ctx, db, tbl, true) },
		"AlterTableRename":       func() error { return drvr.AlterTableRename(ctx, db, "actor", "actor2") },
		"AlterTableAddColumn":    func() error { return drvr.AlterTableAddColumn(ctx, db, "actor", "c", kind.Int) },
		"AlterTableRenameColumn": func() error { return drvr.AlterTableRenameColumn(ctx, db, "actor", "a", "b") },
		"AlterTableColumnKinds": func() error {
			return drvr.AlterTableColumnKinds(ctx, db, "actor", []string{"a"}, []kind.Kind{kind.Int})
		},
	}

	for name, fn := range checks {
		t.Run(name, func(t *testing.T) {
			err := fn()
			require.Error(t, err)
			require.ErrorContains(t, err, "read-only")
		})
	}

	require.Len(t, srv.Statements(), before, "no statement may be sent")
}

// TestExecSQL tests that native SQL passes through, so DDL and DML can be run
// with "sq sql" even though the driver itself is read-only.
func TestExecSQL(t *testing.T) {
	srv := newFake(t, route{
		match: stmtContains("CREATE TABLE"),
		res:   result(thriftfake.Result{}),
	})
	th, grip, _, _ := openFake(t, srv)

	affected, err := libsq.ExecSQL(th.Context, grip, nil, "CREATE TABLE t (a INT)")
	require.NoError(t, err)
	require.Zero(t, affected, "the driver doesn't report rows affected")
	require.Contains(t, srv.Statements(), "CREATE TABLE t (a INT)")
}

func TestGrip_Close(t *testing.T) {
	srv := newFake(t)
	_, grip, _, db := openFake(t, srv)

	require.NoError(t, grip.Close())
	require.NoError(t, grip.Close(), "Close must be idempotent")
	require.Error(t, db.PingContext(context.Background()))
}

// TestLive runs against a real Databricks workspace. It is skipped unless
// envar SQ_TEST_SRC__DATABRICKS_TEST holds the location of the source,
// for example:
//
//	databricks://token:dapi...@HOST/sql/1.0/warehouses/abc123?catalog=main&schema=default
//
// It does not need any particular data, only a warehouse that can run
// "SELECT 1". It is the integration check for what the fake can't verify,
// such as the dialect and the information_schema queries.
func TestLive(t *testing.T) {
	tu.SkipShort(t, true)

	th := testh.New(t)
	src := th.Source("@databricks_test")
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	var one int64
	require.NoError(t, db.QueryRowContext(th.Context, "SELECT 1").Scan(&one))
	require.Equal(t, int64(1), one)

	semver, err := grip.DBSemver(th.Context)
	require.NoError(t, err)
	require.NotEmpty(t, semver)

	catalog, err := drvr.CurrentCatalog(th.Context, db)
	require.NoError(t, err)
	require.NotEmpty(t, catalog)

	catalogs, err := drvr.ListCatalogs(th.Context, db)
	require.NoError(t, err)
	require.Equal(t, catalog, catalogs[0])

	schemas, err := drvr.ListSchemas(th.Context, db)
	require.NoError(t, err)
	require.NotEmpty(t, schemas)

	md, err := grip.SourceMetadata(th.Context, false)
	require.NoError(t, err)
	require.NotEmpty(t, md.DBVersion)
	require.Equal(t, catalog, md.Catalog)

	// A query whose columns have a variety of types.
	const query = `SELECT
		true AS c_bool, CAST(1 AS TINYINT) AS c_tinyint, CAST(2 AS BIGINT) AS c_bigint,
		CAST(1.5 AS DOUBLE) AS c_double, CAST('12.34' AS DECIMAL(10,2)) AS c_decimal,
		'x' AS c_string, DATE'2024-02-29' AS c_date, TIMESTAMP'2024-02-29 13:14:15' AS c_ts,
		array(1,2) AS c_array, map('a',1) AS c_map, named_struct('a',1) AS c_struct,
		CAST(NULL AS STRING) AS c_null_string`
	rows, err := db.QueryContext(th.Context, query)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

	colTypes, err := rows.ColumnTypes()
	require.NoError(t, err)
	recMeta, newRecFn, err := drvr.RecordMeta(th.Context, colTypes, nil)
	require.NoError(t, err)
	require.Equal(t, []kind.Kind{
		kind.Bool, kind.Int, kind.Int, kind.Float, kind.Decimal, kind.Text,
		kind.Date, kind.Datetime, kind.Text, kind.Text, kind.Text, kind.Text,
	}, recMeta.Kinds())

	require.True(t, rows.Next())
	scanRow := recMeta.NewScanRow()
	require.NoError(t, rows.Scan(scanRow...))
	rec, err := newRecFn(scanRow)
	require.NoError(t, err)
	_, err = record.Valid(rec)
	require.NoError(t, err)
	require.Nil(t, rec[len(rec)-1])
	require.False(t, rows.Next())
	require.NoError(t, rows.Err())
}
