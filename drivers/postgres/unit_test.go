package postgres

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// pgLoc is a sample postgres connection location for unit tests
// that don't actually open a connection.
const pgLoc = "postgres://alice:secret@localhost:5432/sakila"

// TestProvider_DriverFor verifies that the Provider returns a driver
// for the postgres type and rejects other types.
func TestProvider_DriverFor(t *testing.T) {
	p := &Provider{Log: lg.Discard()}

	drvr, err := p.DriverFor(drivertype.Pg)
	require.NoError(t, err)
	require.NotNil(t, drvr)

	_, err = p.DriverFor(drivertype.Type("mysql"))
	require.Error(t, err)
}

// TestDriver_StaticMetadata exercises the pure, connection-free
// accessors on the driver.
func TestDriver_StaticMetadata(t *testing.T) {
	d := &driveri{log: lg.Discard()}

	md := d.DriverMetadata()
	require.Equal(t, drivertype.Pg, md.Type)
	require.True(t, md.IsSQL)
	require.False(t, md.IsEmbeddedSQL)
	require.Equal(t, 5432, md.DefaultPort)

	dlct := d.Dialect()
	require.Equal(t, drivertype.Pg, dlct.Type)
	require.Equal(t, 1000, dlct.MaxBatchValues)
	require.True(t, dlct.Catalog)

	shape := d.LocationShape()
	require.Equal(t, drivertype.Pg, shape.Type)
	require.Equal(t, []string{"postgres"}, shape.Schemes)
	require.NotEmpty(t, shape.Segments)

	params := d.ConnParams()
	require.Contains(t, params, "sslmode")
	require.Contains(t, params, "connect_timeout")

	require.NotNil(t, d.ErrWrapFunc())
}

// TestDriver_Renderer verifies the postgres-specific renderer overrides.
func TestDriver_Renderer(t *testing.T) {
	d := &driveri{log: lg.Discard()}
	r := d.Renderer()
	require.NotNil(t, r)
	require.Equal(t, "current_schema", r.FunctionNames[ast.FuncNameSchema])
	require.Equal(t, "current_database", r.FunctionNames[ast.FuncNameCatalog])
	require.Contains(t, r.FunctionOverrides, ast.FuncNameAvg)
	require.Contains(t, r.FunctionOverrides, ast.FuncNameSum)
}

// TestDriver_ValidateSource verifies the type guard.
func TestDriver_ValidateSource(t *testing.T) {
	d := &driveri{log: lg.Discard()}

	src := &source.Source{Handle: "@h", Type: drivertype.Pg, Location: pgLoc}
	got, err := d.ValidateSource(src)
	require.NoError(t, err)
	require.Same(t, src, got)

	_, err = d.ValidateSource(&source.Source{Handle: "@h", Type: drivertype.Type("mysql")})
	require.Error(t, err)
}

// TestDriver_AlterTableColumnKinds confirms the not-implemented stub.
func TestDriver_AlterTableColumnKinds(t *testing.T) {
	d := &driveri{log: lg.Discard()}
	err := d.AlterTableColumnKinds(t.Context(), nil, "tbl", nil, nil)
	require.Error(t, err)
}

func TestDbTypeNameFromKind(t *testing.T) {
	testCases := map[kind.Kind]string{ //nolint:exhaustive // unsupported kinds panic; verified separately.
		kind.Unknown:  "TEXT",
		kind.Text:     "TEXT",
		kind.Int:      "BIGINT",
		kind.Float:    "DOUBLE PRECISION",
		kind.Decimal:  "DECIMAL",
		kind.Bool:     "BOOLEAN",
		kind.Datetime: "TIMESTAMP",
		kind.Time:     "TIME",
		kind.Date:     "DATE",
		kind.Bytes:    "BYTEA",
	}
	for knd, want := range testCases {
		require.Equal(t, want, dbTypeNameFromKind(knd), "kind %s", knd)
	}

	// An unsupported kind must panic.
	require.Panics(t, func() {
		_ = dbTypeNameFromKind(kind.Null)
	})
}

func TestBuildCreateTableStmt(t *testing.T) {
	tblDef := schema.NewTable(
		"person",
		[]string{"id", "name"},
		[]kind.Kind{kind.Int, kind.Text},
	)
	got := buildCreateTableStmt(tblDef)
	require.Contains(t, got, `CREATE TABLE "person"`)
	require.Contains(t, got, `"id" BIGINT`)
	require.Contains(t, got, `"name" TEXT`)
	require.NotContains(t, got, "NOT NULL")

	// With NotNull set, the DEFAULT clause and NOT NULL are emitted.
	for _, col := range tblDef.Cols {
		col.NotNull = true
	}
	got = buildCreateTableStmt(tblDef)
	require.Contains(t, got, `"id" BIGINT DEFAULT 0 NOT NULL`)
	require.Contains(t, got, `"name" TEXT DEFAULT '' NOT NULL`)

	// With PKColName set, an inline PRIMARY KEY is emitted after the
	// DEFAULT / NOT NULL block (#1029).
	tblDef.PKColName = "id"
	got = buildCreateTableStmt(tblDef)
	require.Contains(t, got, `"id" BIGINT DEFAULT 0 NOT NULL PRIMARY KEY`)
	require.NotContains(t, got, `"name" TEXT DEFAULT '' NOT NULL PRIMARY KEY`)
}

func TestBuildUpdateStmt(t *testing.T) {
	got, err := buildUpdateStmt("person", []string{"name", "age"}, "id = 1")
	require.NoError(t, err)
	require.Equal(t, `UPDATE "person" SET "name" = $1, "age" = $2 WHERE id = 1`, got)

	// No WHERE clause.
	got, err = buildUpdateStmt("person", []string{"name"}, "")
	require.NoError(t, err)
	require.Equal(t, `UPDATE "person" SET "name" = $1`, got)

	// Empty cols is an error.
	_, err = buildUpdateStmt("person", nil, "")
	require.Error(t, err)
}

func TestKindFromDBTypeName(t *testing.T) {
	log := lg.Discard()
	testCases := map[string]kind.Kind{
		"":                            kind.Unknown,
		"INT":                         kind.Int,
		"int4":                        kind.Int,
		"BIGINT":                      kind.Int,
		"VARCHAR":                     kind.Text,
		"character varying":           kind.Text,
		"TEXT":                        kind.Text,
		"BYTEA":                       kind.Bytes,
		"BOOL":                        kind.Bool,
		"TIMESTAMP":                   kind.Datetime,
		"timestamp without time zone": kind.Datetime,
		"TIME":                        kind.Time,
		"DATE":                        kind.Date,
		"INTERVAL":                    kind.Text,
		"FLOAT8":                      kind.Float,
		"DOUBLE PRECISION":            kind.Float,
		"UUID":                        kind.Text,
		"NUMERIC":                     kind.Decimal,
		"MONEY":                       kind.Decimal,
		"JSONB":                       kind.Text,
		"VARBIT":                      kind.Text,
		"XML":                         kind.Text,
		"POINT":                       kind.Text,
		"INET":                        kind.Text,
		"USER-DEFINED":                kind.Text,
		"TSVECTOR":                    kind.Text,
		"ARRAY":                       kind.Text,
		"SOME_UNKNOWN_TYPE":           kind.Unknown,
	}
	for dbType, want := range testCases {
		require.Equal(t, want, kindFromDBTypeName(log, "col", dbType), "dbType %q", dbType)
	}
}

func TestToNullableScanType(t *testing.T) {
	log := lg.Discard()
	testCases := []struct {
		scanType reflect.Type
		dbType   string
		knd      kind.Kind
		want     reflect.Type
	}{
		{sqlz.RTypeInt64, "INT8", kind.Int, sqlz.RTypeNullInt64},
		{sqlz.RTypeFloat64, "FLOAT8", kind.Float, sqlz.RTypeNullFloat64},
		{sqlz.RTypeString, "TEXT", kind.Text, sqlz.RTypeNullString},
		{sqlz.RTypeBool, "BOOL", kind.Bool, sqlz.RTypeNullBool},
		{sqlz.RTypeTime, "TIMESTAMP", kind.Datetime, sqlz.RTypeNullTime},
		{sqlz.RTypeBytes, "BYTEA", kind.Bytes, sqlz.RTypeBytes},
		{sqlz.RTypeDecimal, "NUMERIC", kind.Decimal, sqlz.RTypeNullDecimal},
		// Unrecognized scan type (reflect type of `any`) falls through to
		// the db-type-name switch.
		{reflect.TypeFor[any](), "NUMERIC", kind.Decimal, sqlz.RTypeNullDecimal},
		{reflect.TypeFor[any](), "UUID", kind.Text, sqlz.RTypeNullString},
		{reflect.TypeFor[any](), "", kind.Unknown, sqlz.RTypeNullString},
		{reflect.TypeFor[any](), "WIDGET", kind.Unknown, sqlz.RTypeNullString},
	}
	for _, tc := range testCases {
		got := toNullableScanType(log, "col", tc.dbType, tc.knd, tc.scanType)
		require.Equal(t, tc.want, got, "dbType %q scanType %s", tc.dbType, tc.scanType)
	}
}

func TestTblfmt(t *testing.T) {
	require.Equal(t, `"actor"`, tblfmt("actor"))
}

func TestGetPoolConfig(t *testing.T) {
	// Basic parse.
	src := &source.Source{Handle: "@h", Type: drivertype.Pg, Location: pgLoc}
	cfg, err := getPoolConfig(src, false)
	require.NoError(t, err)
	require.Equal(t, "sakila", cfg.ConnConfig.Database)

	// Catalog override rewrites the database in the connection string.
	src2 := &source.Source{Handle: "@h", Type: drivertype.Pg, Location: pgLoc, Catalog: "other_db"}
	cfg2, err := getPoolConfig(src2, false)
	require.NoError(t, err)
	require.Equal(t, "other_db", cfg2.ConnConfig.Database)

	// includeConnTimeout sets connect_timeout in the conn string.
	src3 := &source.Source{Handle: "@h", Type: drivertype.Pg, Location: pgLoc}
	src3.Options = options.Options{driver.OptConnOpenTimeout.Key(): 7 * time.Second}
	cfg3, err := getPoolConfig(src3, true)
	require.NoError(t, err)
	require.Equal(t, 7*time.Second, cfg3.ConnConfig.ConnectTimeout)

	// A malformed location is an error.
	_, err = getPoolConfig(&source.Source{Handle: "@h", Type: drivertype.Pg, Location: "://bad"}, false)
	require.Error(t, err)
}
