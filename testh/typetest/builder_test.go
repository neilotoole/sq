package typetest_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/testh/typetest"
)

func TestNewCase(t *testing.T) {
	c := typetest.NewCase("int", "INT", int64(0), int64(7), "int", "int(11)", kind.Int)
	require.Equal(t, "int", c.Name)
	require.Equal(t, "INT", c.Decl)
	require.Equal(t, int64(0), c.Zero)
	require.Equal(t, int64(7), c.Val)
	require.Equal(t, typetest.Is("int"), c.Want.BaseType)
	require.Equal(t, typetest.Is("int(11)"), c.Want.ColumnType)
	require.Equal(t, typetest.Is(kind.Int), c.Want.Kind)

	cols := mustResolve(t, c)
	require.Len(t, cols, 2)
}

func TestCase_WithWant(t *testing.T) {
	base := typetest.NewCase("year", "YEAR", int64(0), int64(7), "year", "year", kind.Int)
	c := base.WithWant(typetest.Want{Val: typetest.Is[any](int64(2007))})

	require.Equal(t, typetest.Is[any](int64(2007)), c.Want.Val)
	require.Equal(t, typetest.Is("year"), c.Want.BaseType, "fields not set in the arg are kept")
	require.False(t, base.Want.Val.Set, "the receiver is not modified")
}

func TestCase_WithPerVariant(t *testing.T) {
	c := intCase("x").
		WithPerVariant(typetest.PerVariant{NotNull: typetest.Want{Val: typetest.Is[any](int64(-1))}}).
		WithPerVariant(typetest.PerVariant{Nullable: typetest.Want{QueryErr: typetest.Is("boom")}}).
		WithPerVariant(typetest.PerVariant{NotNull: typetest.Want{Zero: typetest.Is[any](int64(1))}})

	require.Equal(t, typetest.Is[any](int64(-1)), c.PerVariant.NotNull.Val, "earlier calls are kept")
	require.Equal(t, typetest.Is[any](int64(1)), c.PerVariant.NotNull.Zero)
	require.Equal(t, typetest.Is("boom"), c.PerVariant.Nullable.QueryErr)
}

func TestCase_WithVersionOverrides(t *testing.T) {
	o1 := typetest.Override{Before: "v5.7.0", Want: typetest.Want{ColumnType: typetest.Is("a")}}
	o2 := typetest.Override{Before: "v8.0.0", Want: typetest.Want{ColumnType: typetest.Is("b")}}
	o3 := typetest.Override{Before: "v8.0.19", Want: typetest.Want{ColumnType: typetest.Is("c")}}

	base := intCase("x").WithVersionOverrides(o1)
	a := base.WithVersionOverrides(o2)
	b := base.WithVersionOverrides(o3)

	require.Equal(t, []typetest.Override{o1}, base.VersionOverrides, "the receiver is not modified")
	require.Equal(t, []typetest.Override{o1, o2}, a.VersionOverrides, "appends")
	require.Equal(t, []typetest.Override{o1, o3}, b.VersionOverrides, "copies don't share a backing array")
}

func TestCase_WithMisc(t *testing.T) {
	c := intCase("x").
		WithIssue("#1365").
		WithMinVer("v5.7.8").
		WithVariants(typetest.NotNull)

	require.Equal(t, "#1365", c.Issue)
	require.Equal(t, "v5.7.8", c.MinVer)
	require.Equal(t, []typetest.Variant{typetest.NotNull}, c.Variants)
}

func TestCase_WithWantZeroVal(t *testing.T) {
	c := intCase("x").WithWantZero(nil).WithWantVal(int64(2007))
	require.Equal(t, typetest.WantNull(), c.Want.Zero, "nil means an expected NULL")
	require.Equal(t, typetest.Is[any](int64(2007)), c.Want.Val)
	require.Equal(t, typetest.Is("int"), c.Want.BaseType)
}
