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
  1, -- row_id
  0, -- col_int
  0, -- col_int_n
  ST_GeomFromText('POINT(1 1)'), -- col_g
  ST_GeomFromText('POINT(1 1)'), -- col_g_n
  0, -- col_gated
  0 -- col_gated_n
)`,
		`INSERT INTO t1 VALUES (
  2, -- row_id
  SEVEN, -- col_int
  SEVEN, -- col_int_n
  ST_GeomFromText('POINT(1 1)'), -- col_g
  ST_GeomFromText('POINT(1 1)'), -- col_g_n
  SEVEN, -- col_gated
  SEVEN -- col_gated_n
)`,
		`INSERT INTO t1 VALUES (
  3, -- row_id
  SEVEN, -- col_int
  NULL, -- col_int_n
  ST_GeomFromText('POINT(1 1)'), -- col_g
  NULL, -- col_g_n
  SEVEN, -- col_gated
  NULL -- col_gated_n
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
	require.Contains(t, got, "\n);\n\nINSERT INTO coltypes VALUES (\n  1, -- row_id\n")
	require.True(t, strings.HasSuffix(got, "  NULL -- col_int_n\n);\n"))

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
