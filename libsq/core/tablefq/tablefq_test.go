package tablefq_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
)

func TestT_Render(t *testing.T) {
	testCases := []struct {
		name string
		in   tablefq.T
		want string
	}{
		{name: "table_only", in: tablefq.T{Table: "actor"}, want: `"actor"`},
		{name: "schema_table", in: tablefq.T{Schema: "public", Table: "actor"}, want: `"public"."actor"`},
		{
			name: "catalog_schema_table",
			in:   tablefq.T{Catalog: "sakila", Schema: "public", Table: "actor"},
			want: `"sakila"."public"."actor"`,
		},
		{name: "empty", in: tablefq.T{}, want: `""`},
		// Malformed: catalog without schema. Documented behavior: schema slot
		// is omitted, so it renders as catalog.table.
		{name: "catalog_no_schema", in: tablefq.T{Catalog: "sakila", Table: "actor"}, want: `"sakila"."actor"`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.in.Render(stringz.DoubleQuote))
		})
	}
}

func TestT_Render_quoteFn(t *testing.T) {
	tbl := tablefq.T{Schema: "db", Table: "t"}
	require.Equal(t, "`db`.`t`", tbl.Render(stringz.BacktickQuote))
}

func TestT_String(t *testing.T) {
	tbl := tablefq.T{Catalog: "sakila", Schema: "public", Table: "actor"}
	require.Equal(t, `"sakila"."public"."actor"`, tbl.String())
}

func TestT_Comparable(t *testing.T) {
	a := tablefq.T{Schema: "public", Table: "actor"}
	b := tablefq.T{Schema: "public", Table: "actor"}
	c := tablefq.T{Schema: "public", Table: "film"}
	require.True(t, a == b)
	require.False(t, a == c)

	// Usable as a map key.
	m := map[tablefq.T]int{a: 1}
	require.Equal(t, 1, m[b])
}

func TestNew(t *testing.T) {
	got := tablefq.New("actor")
	require.Equal(t, tablefq.T{Table: "actor"}, got)
	require.Empty(t, got.Catalog)
	require.Empty(t, got.Schema)
}

func TestFrom(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		require.Equal(t, tablefq.T{Table: "actor"}, tablefq.From("actor"))
	})

	t.Run("T", func(t *testing.T) {
		in := tablefq.T{Catalog: "sakila", Schema: "public", Table: "actor"}
		require.Equal(t, in, tablefq.From(in))
	})
}

func TestFormat(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		require.Equal(t, `"actor"`, tablefq.Format("actor", stringz.DoubleQuote))
	})

	t.Run("T", func(t *testing.T) {
		in := tablefq.T{Schema: "public", Table: "actor"}
		require.Equal(t, `"public"."actor"`, tablefq.Format(in, stringz.DoubleQuote))
	})
}
