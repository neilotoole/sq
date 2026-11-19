package databricks_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/databricks"
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/tablefq"
)

func TestQuoteString(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{"", `''`},
		{"hello", `'hello'`},
		{"it's", `'it\'s'`},
		{"O'Brien", `'O\'Brien'`},
		{`a\b`, `'a\\b'`},
		// A backslash before a quote must not be able to cancel the
		// quote's escape and end the literal early.
		{`\'`, `'\\\''`},
		{`x\' OR 1=1 --`, `'x\\\' OR 1=1 --'`},
		{"two''quotes", `'two\'\'quotes'`},
		{"50%_off", `'50%_off'`},
		{"new\nline", "'new\nline'"},
		{"naïve 日本", "'naïve 日本'"},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, databricks.QuoteString(tc.in))
		})
	}
}

func TestTblfmt(t *testing.T) {
	require.Equal(t, "`actor`", databricks.Tblfmt("actor"))
	require.Equal(t, "`we``ird`", databricks.Tblfmt("we`ird"))
	require.Equal(t, "`actor`", databricks.TblfmtFQ(tablefq.T{Table: "actor"}))
	require.Equal(t, "`sakila`.`actor`", databricks.TblfmtFQ(tablefq.T{Schema: "sakila", Table: "actor"}))
	require.Equal(t, "`main`.`sakila`.`actor`",
		databricks.TblfmtFQ(tablefq.T{Catalog: "main", Schema: "sakila", Table: "actor"}))
}

func TestPlaceholders(t *testing.T) {
	require.Equal(t, "(?)", databricks.Placeholders(1, 1))
	require.Equal(t, "(?, ?, ?)", databricks.Placeholders(3, 1))
	require.Equal(t, "(?), (?), (?)", databricks.Placeholders(1, 3))
	require.Equal(t, "(?, ?), (?, ?)", databricks.Placeholders(2, 2))
}

// TestRenderRange_LimitOverInt32 tests that a LIMIT beyond int32, which Spark
// would reject as a BIGINT, is omitted.
func TestRenderRange_LimitOverInt32(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("int is 32 bits")
	}

	rr := &ast.RowRangeNode{Limit: math.MaxInt, Offset: 7}
	got, err := databricks.RenderRange(nil, rr)
	require.NoError(t, err)
	require.Equal(t, "OFFSET 7", got)
}

func TestRenderRange(t *testing.T) {
	testCases := []struct {
		name string
		rr   *ast.RowRangeNode
		want string
	}{
		{"nil", nil, ""},
		{"none", &ast.RowRangeNode{Limit: -1, Offset: -1}, ""},
		{"limit_only", &ast.RowRangeNode{Limit: 5, Offset: -1}, "LIMIT 5"},
		{"limit_zero", &ast.RowRangeNode{Limit: 0, Offset: -1}, "LIMIT 0"},
		{"limit_offset", &ast.RowRangeNode{Limit: 3, Offset: 3}, "LIMIT 3 OFFSET 3"},
		// OFFSET without LIMIT is valid in Databricks SQL, so there's no need
		// for the huge LIMIT that the default renderer adds for MySQL.
		{"offset_only", &ast.RowRangeNode{Limit: -1, Offset: 10}, "OFFSET 10"},
		{"limit_max_int32", &ast.RowRangeNode{Limit: math.MaxInt32, Offset: 7}, "LIMIT 2147483647 OFFSET 7"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := databricks.RenderRange(nil, tc.rr)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
