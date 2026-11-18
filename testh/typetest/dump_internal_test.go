package typetest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
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

	got := formatActual(col, md, kind.Decimal, []any{int64(0), "7", nil})
	require.Equal(t,
		`col_int_n (INT, nullable): base_type "int4", column_type "integer", kind int, nullable true, `+
			`default_value "", query kind decimal, values [int64(0) "7" <nil>]`,
		got)

	got = formatActual(col, md, kind.Unknown, nil)
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
