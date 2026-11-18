package typetest_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/testh/typetest"
)

// intWant returns the minimal valid expectations for an INT-like case.
func intWant() typetest.Want {
	return typetest.Want{
		BaseType:   typetest.Is("int"),
		ColumnType: typetest.Is("int"),
		Kind:       typetest.Is(kind.Int),
	}
}

func intCase(name string) typetest.Case {
	return typetest.Case{
		Name: name,
		Decl: "INT",
		Zero: int64(0),
		Val:  int64(7),
		Want: intWant(),
	}
}

func TestResolve_Variants(t *testing.T) {
	cols, err := typetest.Resolve([]typetest.Case{intCase("int")}, "v9.1.0")
	require.NoError(t, err)
	require.Len(t, cols, 2)

	nn, n := cols[0], cols[1]
	require.Equal(t, "col_int", nn.Name)
	require.Equal(t, typetest.NotNull, nn.Variant)
	require.False(t, nn.Nullable)
	require.Equal(t, "col_int_n", n.Name)
	require.Equal(t, typetest.Nullable, n.Variant)
	require.True(t, n.Nullable)

	for _, c := range cols {
		require.Equal(t, "int", c.BaseType)
		require.Equal(t, "int", c.ColumnType)
		require.Equal(t, kind.Int, c.Kind)
		require.Equal(t, kind.Int, c.QueryKind)
		require.Equal(t, "", c.Default)
		require.False(t, c.QueryErr.Set)
		require.NotNil(t, c.Case)
		require.Equal(t, "INT", c.Case.Decl)
	}

	require.Equal(t, [3]any{int64(0), int64(7), int64(7)}, nn.In)
	require.Equal(t, nn.In, nn.Want)
	require.Equal(t, [3]any{int64(0), int64(7), nil}, n.In)
	require.Equal(t, n.In, n.Want)
}

func TestResolve_VariantsSubset(t *testing.T) {
	c := intCase("int")
	c.Variants = []typetest.Variant{typetest.Nullable}
	cols, err := typetest.Resolve([]typetest.Case{c}, "v9.1.0")
	require.NoError(t, err)
	require.Len(t, cols, 1)
	require.Equal(t, "col_int_n", cols[0].Name)
}

func TestResolve_MinVer(t *testing.T) {
	c := intCase("json")
	c.MinVer = "v5.7.8"

	testCases := []struct {
		ver  string
		want int
	}{
		{"v5.6.51", 0},
		{"v5.7.8", 2},
		{"v9.1.0", 2},
	}
	for _, tc := range testCases {
		t.Run(tc.ver, func(t *testing.T) {
			cols, err := typetest.Resolve([]typetest.Case{c}, tc.ver)
			require.NoError(t, err)
			require.Len(t, cols, tc.want)
		})
	}
}

func TestResolve_Layering(t *testing.T) {
	c := intCase("x")
	c.PerVariant = typetest.PerVariant{
		Nullable: typetest.Want{ColumnType: typetest.Is("int_v")},
	}
	c.VersionOverrides = []typetest.Override{{
		Before: "v8.0.19",
		Want:   typetest.Want{ColumnType: typetest.Is("int(11)")},
		PerVariant: typetest.PerVariant{
			Nullable: typetest.Want{Val: typetest.WantNull()},
		},
	}}

	cols, err := typetest.Resolve([]typetest.Case{c}, "v9.1.0")
	require.NoError(t, err)
	require.Equal(t, "int", cols[0].ColumnType)
	require.Equal(t, "int_v", cols[1].ColumnType)
	require.Equal(t, int64(7), cols[1].Want[1])

	cols, err = typetest.Resolve([]typetest.Case{c}, "v5.7.44")
	require.NoError(t, err)
	require.Equal(t, "int(11)", cols[0].ColumnType)
	require.Equal(t, "int(11)", cols[1].ColumnType,
		"the override's Want layers after Case.PerVariant")
	require.Equal(t, int64(7), cols[0].Want[1])
	require.Nil(t, cols[1].Want[1])
	require.Equal(t, int64(7), cols[1].In[1], "inputs never change")
}

func TestResolve_VersionOverridesFirstMatchWins(t *testing.T) {
	c := intCase("x")
	c.VersionOverrides = []typetest.Override{
		{Before: "v5.7.0", Want: typetest.Want{ColumnType: typetest.Is("from_v5.7.0")}},
		{Before: "v8.0.19", Want: typetest.Want{ColumnType: typetest.Is("from_v8.0.19")}},
	}

	testCases := []struct {
		ver  string
		want string
	}{
		{"v5.6.51", "from_v5.7.0"},
		{"v8.0.0", "from_v8.0.19"},
		{"v8.0.19", "int"},
	}
	for _, tc := range testCases {
		t.Run(tc.ver, func(t *testing.T) {
			cols, err := typetest.Resolve([]typetest.Case{c}, tc.ver)
			require.NoError(t, err)
			require.Equal(t, tc.want, cols[0].ColumnType)
		})
	}
}

func TestResolve_QueryKindUnknown(t *testing.T) {
	c := intCase("x")
	c.Want.Kind = typetest.Is(kind.Text)
	c.Want.QueryKind = typetest.Is(kind.Unknown)

	cols, err := typetest.Resolve([]typetest.Case{c}, "v9.1.0")
	require.NoError(t, err)
	require.Equal(t, kind.Text, cols[0].Kind)
	require.Equal(t, kind.Unknown, cols[0].QueryKind)
}

func TestResolve_WantNull(t *testing.T) {
	c := intCase("x")
	c.Want.Zero = typetest.WantNull()

	cols, err := typetest.Resolve([]typetest.Case{c}, "v9.1.0")
	require.NoError(t, err)
	require.Equal(t, int64(0), cols[0].In[0])
	require.Nil(t, cols[0].Want[0])
}

func TestResolve_QueryErrPerVariant(t *testing.T) {
	c := intCase("x")
	c.PerVariant = typetest.PerVariant{
		Nullable: typetest.Want{QueryErr: typetest.Is("sql.Null[uint64]")},
	}

	cols, err := typetest.Resolve([]typetest.Case{c}, "v9.1.0")
	require.NoError(t, err)
	require.False(t, cols[0].QueryErr.Set)
	require.True(t, cols[1].QueryErr.Set)
	require.Equal(t, "sql.Null[uint64]", cols[1].QueryErr.Val)
}

func TestResolve_Errors(t *testing.T) {
	testCases := []struct {
		name  string
		cases func() []typetest.Case
	}{
		{"missing_base_type", func() []typetest.Case {
			c := intCase("bad")
			c.Want.BaseType = typetest.Opt[string]{}
			return []typetest.Case{c}
		}},
		{"missing_column_type", func() []typetest.Case {
			c := intCase("bad")
			c.Want.ColumnType = typetest.Opt[string]{}
			return []typetest.Case{c}
		}},
		{"missing_kind", func() []typetest.Case {
			c := intCase("bad")
			c.Want.Kind = typetest.Opt[kind.Kind]{}
			return []typetest.Case{c}
		}},
		{"sql_input_without_expectation", func() []typetest.Case {
			c := intCase("bad")
			c.Val = typetest.SQL("1 + 1")
			return []typetest.Case{c}
		}},
		{"sql_input_with_expectation_for_one_variant_only", func() []typetest.Case {
			c := intCase("bad")
			c.Zero = typetest.SQL("1 - 1")
			c.PerVariant = typetest.PerVariant{
				NotNull: typetest.Want{Zero: typetest.Is[any](int64(0))},
			}
			return []typetest.Case{c}
		}},
		{"column_name_collision", func() []typetest.Case {
			// "bad" yields col_bad_n for its nullable variant, as does
			// "bad_n" for its NOT NULL variant.
			return []typetest.Case{intCase("bad"), intCase("bad_n")}
		}},
		{"invalid_name", func() []typetest.Case {
			return []typetest.Case{intCase("bad-name")}
		}},
		{"version_overrides_not_ascending", func() []typetest.Case {
			c := intCase("bad")
			c.VersionOverrides = []typetest.Override{{Before: "v8.0.19"}, {Before: "v5.7.0"}}
			return []typetest.Case{c}
		}},
		{"version_overrides_duplicate_before", func() []typetest.Case {
			c := intCase("bad")
			c.VersionOverrides = []typetest.Override{{Before: "v5.7.0"}, {Before: "v5.7.0"}}
			return []typetest.Case{c}
		}},
		{"version_overrides_invalid_semver", func() []typetest.Case {
			c := intCase("bad")
			c.VersionOverrides = []typetest.Override{{Before: "8.0.19"}}
			return []typetest.Case{c}
		}},
		{"invalid_min_ver", func() []typetest.Case {
			c := intCase("bad")
			c.MinVer = "5.7"
			return []typetest.Case{c}
		}},
		{"empty_variants", func() []typetest.Case {
			c := intCase("bad")
			c.Variants = []typetest.Variant{}
			return []typetest.Case{c}
		}},
		{"empty_query_err", func() []typetest.Case {
			c := intCase("bad")
			c.Want.QueryErr = typetest.Is("")
			return []typetest.Case{c}
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cols, err := typetest.Resolve(tc.cases(), "v9.1.0")
			require.Error(t, err)
			require.Nil(t, cols)
			require.Contains(t, err.Error(), "bad")
		})
	}
}

func TestResolve_InvalidServerVer(t *testing.T) {
	for _, ver := range []string{"", "8.0", "nope"} {
		t.Run(ver, func(t *testing.T) {
			_, err := typetest.Resolve([]typetest.Case{intCase("int")}, ver)
			require.Error(t, err)
		})
	}
}

func TestResolve_DuplicateVariant(t *testing.T) {
	c := intCase("dup")
	c.Variants = []typetest.Variant{typetest.NotNull, typetest.Nullable, typetest.NotNull}

	cols, err := typetest.Resolve([]typetest.Case{c}, "v9.1.0")
	require.Error(t, err)
	require.Nil(t, cols)
	require.Contains(t, err.Error(), "case {dup}")
	require.Contains(t, err.Error(), "variant {not_null} is listed twice")
}

func TestResolve_NullInput(t *testing.T) {
	c := intCase("x").WithNullInput([]byte(nil))

	cols, err := typetest.Resolve([]typetest.Case{c}, "v9.1.0")
	require.NoError(t, err)
	require.Equal(t, int64(7), cols[0].In[2], "the NOT NULL column's row 3 is still the Val input")
	require.Equal(t, []byte(nil), cols[1].In[2], "the nullable column's row 3 binds NullInput")
	require.IsType(t, []byte(nil), cols[1].In[2])
	require.Nil(t, cols[1].Want[2], "row 3 of the nullable column still expects NULL")
}
