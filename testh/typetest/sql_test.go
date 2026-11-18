package typetest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/testh/typetest"
)

// qmarkPlaceholders mimics a "?" dialect, e.g. MySQL. Only numRows=1 is
// needed here.
func qmarkPlaceholders(numCols, _ int) string {
	return "(" + strings.TrimSuffix(strings.Repeat("?, ", numCols), ", ") + ")"
}

// dollarPlaceholders mimics a "$n" dialect, e.g. Postgres.
func dollarPlaceholders(numCols, _ int) string {
	toks := make([]string, numCols)
	for i := range toks {
		toks[i] = fmt.Sprintf("$%d", i+1)
	}
	return "(" + strings.Join(toks, ", ") + ")"
}

func mustResolve(t *testing.T, cases ...typetest.Case) []typetest.Column {
	t.Helper()
	cols, err := typetest.Resolve(cases, "v9.1.0")
	require.NoError(t, err)
	return cols
}

// geomCase is a case whose inputs are SQL expressions.
func geomCase() typetest.Case {
	const expr = typetest.SQL("ST_GeomFromText('POINT(1 1)')")
	return typetest.Case{
		Name: "g",
		Decl: "GEOMETRY",
		Zero: expr,
		Val:  expr,
		Want: typetest.Want{
			BaseType:   typetest.Is("geometry"),
			ColumnType: typetest.Is("geometry"),
			Kind:       typetest.Is(kind.Bytes),
			Zero:       typetest.Is[any]("g0"),
			Val:        typetest.Is[any]("g1"),
		},
	}
}

func TestDefaultColumnDecl(t *testing.T) {
	require.Equal(t, "TIMESTAMP NOT NULL", typetest.DefaultColumnDecl("TIMESTAMP", typetest.NotNull))
	require.Equal(t, "TIMESTAMP NULL", typetest.DefaultColumnDecl("TIMESTAMP", typetest.Nullable))
}

func TestCreateTable(t *testing.T) {
	cols := mustResolve(t, intCase("int"))
	got := typetest.CreateTable("t1", "INT NOT NULL PRIMARY KEY", cols, typetest.DefaultColumnDecl)
	require.Equal(t,
		"CREATE TABLE t1 (row_id INT NOT NULL PRIMARY KEY, col_int INT NOT NULL, col_int_n INT NULL)",
		got)
}

func TestInsert_QmarkPlaceholders(t *testing.T) {
	cols := mustResolve(t, intCase("int"), geomCase())

	query, args := typetest.Insert("t1", cols, 1, qmarkPlaceholders)
	require.Equal(t,
		"INSERT INTO t1 (row_id, col_int, col_int_n, col_g, col_g_n) VALUES "+
			"(?, ?, ?, ST_GeomFromText('POINT(1 1)'), ST_GeomFromText('POINT(1 1)'))",
		query)
	require.Equal(t, []any{2, int64(7), int64(7)}, args)
}

func TestInsert_DollarPlaceholders(t *testing.T) {
	cols := mustResolve(t, intCase("int"), geomCase())

	query, args := typetest.Insert("t1", cols, 1, dollarPlaceholders)
	require.Equal(t,
		"INSERT INTO t1 (row_id, col_int, col_int_n, col_g, col_g_n) VALUES "+
			"($1, $2, $3, ST_GeomFromText('POINT(1 1)'), ST_GeomFromText('POINT(1 1)'))",
		query)
	require.Equal(t, []any{2, int64(7), int64(7)}, args)

	// Row 3: the nullable columns get NULL, bound as a nil arg, including the
	// column whose other inputs are SQL expressions.
	query, args = typetest.Insert("t1", cols, 2, dollarPlaceholders)
	require.Equal(t,
		"INSERT INTO t1 (row_id, col_int, col_int_n, col_g, col_g_n) VALUES "+
			"($1, $2, $3, ST_GeomFromText('POINT(1 1)'), $4)",
		query)
	require.Equal(t, []any{3, int64(7), nil, nil}, args)
}

func TestSelectColumn(t *testing.T) {
	cols := mustResolve(t, intCase("int"))
	require.Equal(t, "SELECT row_id, col_int_n FROM t1 ORDER BY row_id", typetest.SelectColumn("t1", cols[1]))
}

func TestSelectAll(t *testing.T) {
	cols := mustResolve(t, intCase("int"), intCase("x"))
	require.Equal(t, "SELECT * FROM t1 ORDER BY row_id", typetest.SelectAll("t1", cols))

	c := intCase("int")
	c.PerVariant = typetest.PerVariant{Nullable: typetest.Want{QueryErr: typetest.Is("boom")}}
	cols = mustResolve(t, c, intCase("x"))
	require.Equal(t, "SELECT row_id, col_int, col_x, col_x_n FROM t1 ORDER BY row_id",
		typetest.SelectAll("t1", cols))
}

func TestSelectAllNames(t *testing.T) {
	c := intCase("int")
	c.PerVariant = typetest.PerVariant{Nullable: typetest.Want{QueryErr: typetest.Is("boom")}}
	cols := mustResolve(t, c, intCase("x"))
	require.Equal(t, []string{"row_id", "col_int", "col_x", "col_x_n"}, typetest.SelectAllNames(cols))
}
