package typetest

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

func TestFormatActual(t *testing.T) {
	c := Case{Name: "int", Decl: "INT"}
	col := Column{Case: &c, Name: "col_int_n", Variant: Nullable}
	md := &metadata.Column{
		Name:         "col_int_n",
		BaseType:     "int4",
		ColumnType:   "integer",
		Kind:         kind.Int,
		Nullable:     true,
		DefaultValue: "",
	}

	got := formatActual(col, md, "", kind.Decimal, []any{int64(0), "7", nil}, nil)
	require.Equal(t,
		`col_int_n (INT, nullable): base_type "int4", column_type "integer", kind int, nullable true, `+
			`default_value "", query kind decimal, values [int64(0) "7" <nil>]`,
		got)

	got = formatActual(col, md, "", kind.Unknown, nil, nil)
	require.Contains(t, got, "values <query failed>")
}

func TestExpandTableToken(t *testing.T) {
	cols := []Column{
		{Name: "col_serial", Default: "nextval('" + TableToken + "_col_serial_seq'::regclass)"},
		{Name: "col_int", Default: ""},
	}
	got := expandTableToken(cols, "type_test__abc")
	require.Equal(t, "nextval('type_test__abc_col_serial_seq'::regclass)", got[0].Default)
	require.Equal(t, "", got[1].Default)
	require.Equal(t, "nextval('"+TableToken+"_col_serial_seq'::regclass)", cols[0].Default,
		"the input is not modified")
}

func TestFormatActual_QueryErr(t *testing.T) {
	c := Case{Name: "int", Decl: "INT"}
	col := Column{Case: &c, Name: "col_int", Variant: NotNull}
	md := &metadata.Column{Name: "col_int", BaseType: "int", ColumnType: "int", Kind: kind.Int}

	got := formatActual(col, md, "", kind.Unknown, nil, errors.New("boom"))
	require.Contains(t, got, "values <query failed: boom>")
}

func TestFormatActual_TableToken(t *testing.T) {
	c := Case{Name: "serial", Decl: "SERIAL"}
	col := Column{Case: &c, Name: "col_serial", Variant: NotNull}
	md := &metadata.Column{
		Name: "col_serial", BaseType: "int4", ColumnType: "integer", Kind: kind.Int,
		DefaultValue: "nextval('type_test__abc_col_serial_seq'::regclass)",
	}

	got := formatActual(col, md, "type_test__abc", kind.Int, []any{int64(0)}, nil)
	require.Contains(t, got, `default_value "nextval('`+TableToken+`_col_serial_seq'::regclass)"`,
		"the generated table name is replaced by TableToken, so the default can be pasted")
}

func TestDiffRowIDs(t *testing.T) {
	ok := []record.Record{{int64(1)}, {int64(2)}, {int64(3)}}
	require.NoError(t, diffRowIDs(ok))

	for name, recs := range map[string][]record.Record{
		"out_of_order": {{int64(2)}, {int64(1)}, {int64(3)}},
		"wrong_type":   {{"1"}, {"2"}, {"3"}},
		"null":         {{nil}, {int64(2)}, {int64(3)}},
	} {
		t.Run(name, func(t *testing.T) {
			require.Error(t, diffRowIDs(recs))
		})
	}
}

func TestWithDumpPlaceholderDefaults(t *testing.T) {
	// Dump mode is how a case's expected default_value is drafted, so a
	// Defaulted column with none set mustn't stop Resolve before the dump.
	base := Case{
		Name: "x", Decl: "INT", Zero: int64(0), Val: int64(7),
		Want:     Want{BaseType: Is("int"), ColumnType: Is("int"), Kind: Is(kind.Int)},
		Variants: []Variant{NotNull, Defaulted},
	}
	set := base.WithWantDefault("7")
	set.Name = "y"
	got := withDumpPlaceholderDefaults([]Case{base, set})

	cols, err := Resolve(got, "v9.1.0")
	require.NoError(t, err)
	require.Equal(t, dumpPlaceholderDefault, cols[1].Default)
	require.Equal(t, "7", cols[3].Default, "a set expectation is kept")

	_, err = Resolve([]Case{base}, "v9.1.0")
	require.Error(t, err, "the input is not modified")
}

func TestDumpConfig(t *testing.T) {
	// In dump mode, Run uses dumpConfig's Config for every path, the ddl
	// subtest's RenderStatements included, so a Defaulted column with no
	// expected default doesn't stop the ddl subtest before it loads.
	cfg := Config{
		RowIDDecl: "INT",
		Cases: []Case{{
			Name: "x", Decl: "INT", Zero: int64(0), Val: int64(7),
			Want: Want{BaseType: Is("int"), ColumnType: Is("int"), Kind: Is(kind.Int)},
		}},
	}
	_, err := RenderStatements(cfg, "t1", "v9.1.0")
	require.Error(t, err)

	_, err = RenderStatements(dumpConfig(cfg), "t1", "v9.1.0")
	require.NoError(t, err)
	require.False(t, cfg.Cases[0].PerVariant.Defaulted.Default.Set, "the input is not modified")
}
