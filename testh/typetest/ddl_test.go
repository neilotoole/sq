package typetest_test

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/typetest"
)

func TestDefaultLiteral(t *testing.T) {
	testCases := []struct {
		in   any
		want string
	}{
		{nil, "NULL"},
		{[]byte(nil), "NULL"},
		{typetest.SQL("ST_GeomFromText('POINT(1 1)')"), "ST_GeomFromText('POINT(1 1)')"},
		{"", "''"},
		{"it's", "'it''s'"},
		{int64(-7), "-7"},
		{uint64(math.MaxUint64), "18446744073709551615"},
		{float64(0), "0"},
		{7.7, "7.7"},
		{true, "TRUE"},
		{false, "FALSE"},
		{decimal.RequireFromString("77.77"), "77.77"},
		{time.Date(1989, 11, 9, 0, 0, 0, 0, time.UTC), "'1989-11-09'"},
		{time.Date(2017, 7, 7, 7, 7, 7, 0, time.UTC), "'2017-07-07 07:07:07'"},
		{time.Date(2017, 7, 7, 7, 7, 7, 123000000, time.UTC), "'2017-07-07 07:07:07.123'"},
		{time.Date(2017, 7, 7, 0, 7, 7, 0, time.FixedZone("", 7*3600)), "'2017-07-06 17:07:07'"},
	}
	for _, tc := range testCases {
		got, err := typetest.DefaultLiteral(tc.in)
		require.NoError(t, err, "%T(%v)", tc.in, tc.in)
		require.Equal(t, tc.want, got, "%T(%v)", tc.in, tc.in)
	}

	// Bytes have no portable literal: each dialect writes them differently.
	_, err := typetest.DefaultLiteral([]byte("seven"))
	require.Error(t, err)
	_, err = typetest.DefaultLiteral(struct{}{})
	require.Error(t, err)
}

func TestRenderStatements(t *testing.T) {
	var gotDecls []string
	cfg := typetest.Config{
		RowIDDecl: "INT NOT NULL PRIMARY KEY",
		Cases: []typetest.Case{
			intCase("int"),
			geomCase(),
			intCase("gated").WithMinVer("v8.0.0"),
		},
		// The driver's func handles int64 7; everything else falls back to
		// DefaultLiteral. It's given the column, since some dialects write a
		// value differently by column type.
		Literal: func(col typetest.Column, v any) (string, bool) {
			gotDecls = append(gotDecls, col.Case.Decl)
			if i, ok := v.(int64); ok && i == 7 {
				return "SEVEN", true
			}
			return "", false
		},
	}

	stmts, err := typetest.RenderStatements(cfg, "t1", "v9.1.0")
	require.NoError(t, err)
	require.Equal(t, []string{
		`CREATE TABLE t1 (
  row_id INT NOT NULL PRIMARY KEY,
  col_int INT NOT NULL,
  col_int_n INT NULL,
  col_g GEOMETRY NOT NULL,
  col_g_n GEOMETRY NULL,
  col_gated INT NOT NULL,
  col_gated_n INT NULL
)`,
		`INSERT INTO t1 VALUES (
  -- row_id
  1,
  -- col_int
  0,
  -- col_int_n
  0,
  -- col_g
  ST_GeomFromText('POINT(1 1)'),
  -- col_g_n
  ST_GeomFromText('POINT(1 1)'),
  -- col_gated
  0,
  -- col_gated_n
  0
)`,
		`INSERT INTO t1 VALUES (
  -- row_id
  2,
  -- col_int
  SEVEN,
  -- col_int_n
  SEVEN,
  -- col_g
  ST_GeomFromText('POINT(1 1)'),
  -- col_g_n
  ST_GeomFromText('POINT(1 1)'),
  -- col_gated
  SEVEN,
  -- col_gated_n
  SEVEN
)`,
		`INSERT INTO t1 VALUES (
  -- row_id
  3,
  -- col_int
  SEVEN,
  -- col_int_n
  NULL,
  -- col_g
  ST_GeomFromText('POINT(1 1)'),
  -- col_g_n
  NULL,
  -- col_gated
  SEVEN,
  -- col_gated_n
  NULL
)`,
	}, stmts)
	require.Contains(t, gotDecls, "GEOMETRY", "the literal func is given each value's column")

	// On an older server, a case gated by MinVer is left out, as in Run.
	stmts, err = typetest.RenderStatements(cfg, "t1", "v5.7.44")
	require.NoError(t, err)
	require.NotContains(t, stmts[0], "col_gated")
}

func TestRenderStatements_Suffix(t *testing.T) {
	cfg := typetest.Config{
		RowIDDecl:   "Int32",
		ColumnDecl:  func(decl string, v typetest.Variant) string { return decl + "/" + v.String() },
		TableSuffix: "ENGINE = MergeTree() ORDER BY row_id",
		Cases:       []typetest.Case{intCase("int")},
	}
	stmts, err := typetest.RenderStatements(cfg, "t1", "v9.1.0")
	require.NoError(t, err)
	require.Equal(t, `CREATE TABLE t1 (
  row_id Int32,
  col_int INT/not_null,
  col_int_n INT/nullable
) ENGINE = MergeTree() ORDER BY row_id`, stmts[0])
}

func TestRenderStatements_UnrenderableInput(t *testing.T) {
	c := intCase("blob")
	c.Val = []byte("seven")
	_, err := typetest.RenderStatements(typetest.Config{RowIDDecl: "INT", Cases: []typetest.Case{c}}, "t1", "v9.1.0")
	require.Error(t, err)
	require.Contains(t, err.Error(), "col_blob", "the error names the column")
}

func TestRenderDDL(t *testing.T) {
	cfg := typetest.Config{RowIDDecl: "INT NOT NULL PRIMARY KEY", Cases: []typetest.Case{intCase("int")}}
	got, err := typetest.RenderDDL("-- header\n", typetest.DDLTable{Name: "coltypes", Config: cfg})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(got, "-- header\n\nCREATE TABLE coltypes (\n"))
	require.Contains(t, got, "\n);\n\nINSERT INTO coltypes VALUES (\n  -- row_id\n  1,\n")
	require.True(t, strings.HasSuffix(got, "  -- col_int_n\n  NULL\n);\n"))

	// Every MinVer-gated case is included: the file is for the newest
	// supported server version.
	cfg.Cases = append(cfg.Cases, intCase("gated").WithMinVer("v23.4.0"))
	got, err = typetest.RenderDDL("", typetest.DDLTable{Name: "coltypes", Config: cfg})
	require.NoError(t, err)
	require.Contains(t, got, "col_gated")
}

func TestCompareDDL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "coltypes.ddl")

	require.Error(t, typetest.CompareDDL(path, "CREATE TABLE x (a INT);\n", false), "a missing file is a mismatch")

	require.NoError(t, typetest.CompareDDL(path, "CREATE TABLE x (a INT);\n", true), "update writes the file")
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "CREATE TABLE x (a INT);\n", string(b))

	require.NoError(t, typetest.CompareDDL(path, "CREATE TABLE x (a INT);\n", false))

	err = typetest.CompareDDL(path, "CREATE TABLE x (b INT);\n", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), typetest.UpdateEnvar, "the error says how to regenerate")
}

func TestTimestampTZLiteral(t *testing.T) {
	require.Equal(t, "'1989-11-09 00:00:00+00:00'",
		typetest.TimestampTZLiteral(time.Date(1989, 11, 9, 0, 0, 0, 0, time.UTC)))
	require.Equal(t, "'2017-07-07 07:07:07.123+00:00'",
		typetest.TimestampTZLiteral(time.Date(2017, 7, 7, 0, 7, 7, 123000000, time.FixedZone("", -7*3600))))
}

func TestCompareDDL_CreatesDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testdata", "coltypes.ddl")
	require.NoError(t, typetest.CompareDDL(path, "x\n", true), "update creates a missing testdata directory")
	require.NoError(t, typetest.CompareDDL(path, "x\n", false))
}

func TestHasTimeZone(t *testing.T) {
	for _, decl := range []string{
		"TIMESTAMPTZ", "timestamptz(3)", "TIMETZ", "TIMESTAMP WITH TIME ZONE", "TIME(3) WITH TIME ZONE",
		"TIMESTAMP(6) WITH LOCAL TIME ZONE", "DATETIMEOFFSET", "datetimeoffset(7)",
	} {
		require.True(t, typetest.HasTimeZone(decl), decl)
	}
	for _, decl := range []string{
		"TIMESTAMP", "TIMESTAMP WITHOUT TIME ZONE", "TIME WITHOUT TIME ZONE", "DATETIME2", "DateTime('UTC')",
		"TIME",
	} {
		require.False(t, typetest.HasTimeZone(decl), decl)
	}
}

func TestDefaultLiteral_NonFinite(t *testing.T) {
	// NaN and infinities have no portable literal, and a bare NaN is invalid
	// SQL in most dialects: an error here fails TestColumnTypesDDL, with no
	// database, rather than only the live load.
	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := typetest.DefaultLiteral(f)
		require.Error(t, err, "%v", f)
	}
}

func TestCompareDDL_MismatchNamesLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "coltypes.ddl")
	require.NoError(t, typetest.CompareDDL(path, "a\nb\nc\n", true))

	err := typetest.CompareDDL(path, "a\nB\nc\n", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "line 2")
	require.Contains(t, err.Error(), `"b"`, "the file's line")
	require.Contains(t, err.Error(), `"B"`, "the rendered line")

	err = typetest.CompareDDL(path, "a\nb\nc\nd\n", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "line 4", "a file that ends early differs at its end")
}

func TestRenderStatements_Prelude(t *testing.T) {
	cfg := typetest.Config{
		RowIDDecl:  "INT",
		DDLPrelude: []string{"SET time_zone = '+00:00'"},
		Cases:      []typetest.Case{intCase("int")},
	}
	stmts, err := typetest.RenderStatements(cfg, "t1", "v9.1.0")
	require.NoError(t, err)
	require.Len(t, stmts, 5)
	require.Equal(t, "SET time_zone = '+00:00'", stmts[0], "the prelude runs first")
	require.True(t, strings.HasPrefix(stmts[1], "CREATE TABLE t1 ("))

	got, err := typetest.RenderDDL("", typetest.DDLTable{Name: "t1", Config: cfg})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(got, "\nSET time_zone = '+00:00';\n\nCREATE TABLE t1 ("))
}
