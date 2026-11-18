package typetest_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh/typetest"
)

func TestDiffMeta(t *testing.T) {
	cols := mustResolve(t, intCase("int"))
	want := cols[1] // col_int_n

	matching := func() *metadata.Column {
		return &metadata.Column{
			Name:       "col_int_n",
			BaseType:   "int",
			ColumnType: "int",
			Kind:       kind.Int,
			Nullable:   true,
		}
	}

	require.Empty(t, typetest.DiffMeta("TableMetadata", "v9.1.0", matching(), want))

	testCases := []struct {
		field  string
		mutate func(c *metadata.Column)
	}{
		{"base_type", func(c *metadata.Column) { c.BaseType = "bigint" }},
		{"column_type", func(c *metadata.Column) { c.ColumnType = "int(11)" }},
		{"kind", func(c *metadata.Column) { c.Kind = kind.Decimal }},
		{"nullable", func(c *metadata.Column) { c.Nullable = false }},
		{"default_value", func(c *metadata.Column) { c.DefaultValue = "0" }},
	}
	for _, tc := range testCases {
		t.Run(tc.field, func(t *testing.T) {
			got := matching()
			tc.mutate(got)
			msgs := typetest.DiffMeta("SourceMetadata", "v9.1.0", got, want)
			require.Len(t, msgs, 1)
			for _, s := range []string{"SourceMetadata", "col_int_n", "INT", "v9.1.0", tc.field} {
				require.Contains(t, msgs[0], s)
			}
		})
	}
}

func TestMatchQueryErr(t *testing.T) {
	c := intCase("x")
	c.Vars = typetest.Vars{Nullable: typetest.Want{QueryErr: typetest.Is("sql.Null[uint64]")}}
	col := mustResolve(t, c)[1]
	require.Equal(t, "col_x_n", col.Name)

	require.NoError(t, typetest.MatchQueryErr(
		errors.New("column 1 col_x_n: unknown type db(*sql.Null[uint64])"), col,
	))

	require.Error(t, typetest.MatchQueryErr(nil, col), "no error at all")
	require.Error(t, typetest.MatchQueryErr(
		errors.New("column 1 col_y_n: unknown type db(*sql.Null[uint64])"), col,
	), "another column's error")
	require.Error(t, typetest.MatchQueryErr(errors.New("connection refused"), col), "unrelated error")
}
