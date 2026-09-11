package oracle

import (
	"errors"
	"fmt"
	"testing"

	goora "github.com/sijms/go-ora/v2/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
)

// TestPlaceholders tests the placeholder generation function.
func TestPlaceholders(t *testing.T) {
	testCases := []struct {
		numCols  int
		numRows  int
		expected string
	}{
		{
			numCols:  1,
			numRows:  1,
			expected: "(:1)",
		},
		{
			numCols:  2,
			numRows:  1,
			expected: "(:1, :2)",
		},
		{
			numCols:  3,
			numRows:  1,
			expected: "(:1, :2, :3)",
		},
		{
			numCols:  2,
			numRows:  2,
			expected: "(:1, :2), (:3, :4)",
		},
		{
			numCols:  3,
			numRows:  2,
			expected: "(:1, :2, :3), (:4, :5, :6)",
		},
		{
			numCols:  1,
			numRows:  3,
			expected: "(:1), (:2), (:3)",
		},
	}

	for _, tc := range testCases {
		actual := placeholders(tc.numCols, tc.numRows)
		assert.Equal(t, tc.expected, actual,
			"placeholders(%d, %d) failed", tc.numCols, tc.numRows)
	}
}

// TestHasErrCode tests the hasErrCode function for nil, plain errors,
// matching go-ora errors, and go-ora errors wrapped via fmt.Errorf.
func TestHasErrCode(t *testing.T) {
	assert.False(t, hasErrCode(nil, 942))

	stdErr := errors.New("standard error")
	assert.False(t, hasErrCode(stdErr, 942))

	oraErr := goora.NewOracleError(942)
	assert.True(t, hasErrCode(oraErr, 942), "should match raw go-ora OracleError")
	assert.False(t, hasErrCode(oraErr, 904), "should not match a different code")

	// Wire drivers commonly wrap their underlying errors; HasErrCode must
	// still find the code via errors.As traversal.
	wrapped := fmt.Errorf("exec: %w", oraErr)
	assert.True(t, hasErrCode(wrapped, 942), "should match through fmt.Errorf wrapping")
}

// TestIsErrTableNotExist tests the isErrTableNotExist function.
func TestIsErrTableNotExist(t *testing.T) {
	assert.False(t, isErrTableNotExist(nil))

	stdErr := errors.New("some error")
	assert.False(t, isErrTableNotExist(stdErr))

	// ORA-00942: this is the case that DropTable(ifExists=true) relies on.
	assert.True(t, isErrTableNotExist(goora.NewOracleError(942)),
		"ORA-00942 must be detected so DropTable can honor ifExists=true")
	assert.True(t, isErrTableNotExist(fmt.Errorf("exec: %w", goora.NewOracleError(942))),
		"ORA-00942 must be detected through fmt.Errorf wrapping")

	assert.False(t, isErrTableNotExist(goora.NewOracleError(904)),
		"ORA-00904 is a different error and must not match")
}

// TestDbTypeNameFromKind tests the type mapping from kind to Oracle types.
func TestDbTypeNameFromKind(t *testing.T) {
	require.Equal(t, "VARCHAR2(4000)", dbTypeNameFromKind(kind.Text))
	require.Equal(t, "NUMBER(19,0)", dbTypeNameFromKind(kind.Int))
	require.Equal(t, "BINARY_DOUBLE", dbTypeNameFromKind(kind.Float))
	require.Equal(t, "NUMBER", dbTypeNameFromKind(kind.Decimal))
	require.Equal(t, "NUMBER(1,0)", dbTypeNameFromKind(kind.Bool))
	require.Equal(t, "TIMESTAMP", dbTypeNameFromKind(kind.Datetime))
	require.Equal(t, "TIMESTAMP", dbTypeNameFromKind(kind.Time))
	require.Equal(t, "DATE", dbTypeNameFromKind(kind.Date))
	require.Equal(t, "BLOB", dbTypeNameFromKind(kind.Bytes))
	require.Equal(t, "VARCHAR2(4000)", dbTypeNameFromKind(kind.Unknown))
}

// TestKindFromOracleNumber tests precision/scale inference for NUMBER columns.
func TestKindFromOracleNumber(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		typeName string
		want     kind.Kind
	}{
		{"NUMBER(1,0)", kind.Int},
		{"NUMBER(10,0)", kind.Int},
		{"NUMBER(18,0)", kind.Int},
		{"NUMBER(19,0)", kind.Int},
		{"NUMBER(20,0)", kind.Decimal}, // exceeds int64 range
		{"NUMBER(10)", kind.Int},       // no explicit scale → treated as scale=0
		{"NUMBER(1,1)", kind.Decimal},  // scale != 0
		{"NUMBER(10,2)", kind.Decimal}, // fractional
		{"NUMBER(0,0)", kind.Decimal},  // precision 0 not integer range
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.typeName, func(t *testing.T) {
			t.Parallel()
			got := kindFromOracleNumber(tc.typeName)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestKindFromDBTypeName_NumberPrefix tests that NUMBER(...) type names are
// correctly dispatched to kindFromOracleNumber.
func TestKindFromDBTypeName_NumberPrefix(t *testing.T) {
	require.Equal(t, kind.Int, kindFromDBTypeName(nil, "col", "NUMBER(19,0)"))
	require.Equal(t, kind.Int, kindFromDBTypeName(nil, "col", "NUMBER(10,0)"))
	require.Equal(t, kind.Decimal, kindFromDBTypeName(nil, "col", "NUMBER(20,0)"))
	require.Equal(t, kind.Decimal, kindFromDBTypeName(nil, "col", "NUMBER(10,2)"))
	// Bare NUMBER (no precision) stays Decimal; callers refine via DecimalSize().
	require.Equal(t, kind.Decimal, kindFromDBTypeName(nil, "col", "NUMBER"))
}

// TestRefineBareNumberKind covers the bare-NUMBER refinement applied in
// RecordMeta when the wire-level type name lacks precision/scale info.
func TestRefineBareNumberKind(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name             string
		precision, scale int64
		ok               bool
		want             kind.Kind
	}{
		// The floating-scale form (scale 255) is ambiguous: COUNT(*), SUM, AVG,
		// integer literals, and division all report it. It maps to kind.Decimal
		// so fractional values (e.g. division) don't crash an int64 scan (#844).
		{"floating_38_255", 38, oracleScaleFloating, true, kind.Decimal},
		// Same shape, different precision — still floating, still decimal.
		{"floating_0_255", 0, oracleScaleFloating, true, kind.Decimal},
		// NUMBER(p,0) with p in [1..19] is int range.
		{"int_5_0", 5, 0, true, kind.Int},
		{"int_19_0", 19, 0, true, kind.Int},
		// Out of int64 range or non-zero scale stays decimal.
		{"decimal_20_0", 20, 0, true, kind.Decimal},
		{"decimal_10_2", 10, 2, true, kind.Decimal},
		{"decimal_0_0", 0, 0, true, kind.Decimal},
		// DecimalSize() not available — leave as decimal.
		{"not_ok", 19, 0, false, kind.Decimal},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, refineBareNumberKind(tc.precision, tc.scale, tc.ok))
		})
	}
}

// TestStripTypeParams verifies the paren-stripping helper handles bare names,
// trailing parens, and parens embedded in multi-word Oracle type names.
func TestStripTypeParams(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		in, want string
	}{
		// Bare names pass through.
		{"VARCHAR2", "VARCHAR2"},
		{"DATE", "DATE"},
		{"BLOB", "BLOB"},
		{"LONG RAW", "LONG RAW"},

		// Single-param trailing parens.
		{"VARCHAR2(91)", "VARCHAR2"},
		{"VARCHAR2(4000)", "VARCHAR2"},
		{"RAW(16)", "RAW"},
		{"FLOAT(126)", "FLOAT"},
		{"CHAR(10)", "CHAR"},

		// Two-param trailing parens.
		{"NUMBER(10,2)", "NUMBER"},
		{"NUMBER(19,0)", "NUMBER"},

		// Interior parens within multi-word type names.
		{"TIMESTAMP(6)", "TIMESTAMP"},
		{"TIMESTAMP(6) WITH TIME ZONE", "TIMESTAMP WITH TIME ZONE"},
		{"TIMESTAMP(9) WITH LOCAL TIME ZONE", "TIMESTAMP WITH LOCAL TIME ZONE"},
		{"INTERVAL DAY(2) TO SECOND(6)", "INTERVAL DAY TO SECOND"},
		{"INTERVAL YEAR(2) TO MONTH", "INTERVAL YEAR TO MONTH"},

		// Whitespace handling: redundant spaces collapse, leading/trailing trim.
		{"  VARCHAR2  ", "VARCHAR2"},
		{"TIMESTAMP  WITH  TIME  ZONE", "TIMESTAMP WITH TIME ZONE"},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, stripTypeParams(tc.in))
		})
	}
}

// TestKindFromDBTypeName covers the full set of Oracle type names we expect
// to see from the data dictionary, including the parameterized forms that
// were previously falling through to kind.Unknown.
func TestKindFromDBTypeName(t *testing.T) {
	t.Parallel()
	testCases := map[string]kind.Kind{
		// Parameterized character types observed in the bug report.
		"VARCHAR2(91)":   kind.Text,
		"VARCHAR2(50)":   kind.Text,
		"VARCHAR2(10)":   kind.Text,
		"VARCHAR2(20)":   kind.Text,
		"VARCHAR2(6)":    kind.Text,
		"VARCHAR2(128)":  kind.Text,
		"VARCHAR2(4000)": kind.Text,
		"VARCHAR2(25)":   kind.Text,
		"NVARCHAR2(50)":  kind.Text,
		"CHAR(1)":        kind.Text,
		"NCHAR(10)":      kind.Text,

		// Bare character types.
		"VARCHAR2":  kind.Text,
		"NVARCHAR2": kind.Text,
		"CHAR":      kind.Text,
		"NCHAR":     kind.Text,
		"CLOB":      kind.Text,
		"NCLOB":     kind.Text,

		// Binary types.
		"RAW(16)":   kind.Bytes,
		"RAW(2000)": kind.Bytes,
		"RAW":       kind.Bytes,
		"BLOB":      kind.Bytes,
		"LONG RAW":  kind.Bytes,

		// Numeric types.
		"FLOAT(126)":    kind.Float,
		"FLOAT":         kind.Float,
		"BINARY_FLOAT":  kind.Float,
		"BINARY_DOUBLE": kind.Float,
		"NUMBER":        kind.Decimal,
		"NUMBER(19,0)":  kind.Int,
		"NUMBER(20,0)":  kind.Decimal,
		"NUMBER(10,2)":  kind.Decimal,

		// Date/time types.
		"DATE":                              kind.Datetime,
		"TIMESTAMP":                         kind.Datetime,
		"TIMESTAMP(6)":                      kind.Datetime,
		"TIMESTAMP(9)":                      kind.Datetime,
		"TIMESTAMP WITH TIME ZONE":          kind.Datetime,
		"TIMESTAMP(6) WITH TIME ZONE":       kind.Datetime,
		"TIMESTAMP WITH LOCAL TIME ZONE":    kind.Datetime,
		"TIMESTAMP(6) WITH LOCAL TIME ZONE": kind.Datetime,

		// go-ora wire-driver type names (returned by
		// sql.ColumnType.DatabaseTypeName() during query execution rather
		// than the data-dictionary form). Mixed-case input here would arrive
		// after ToUpper inside kindFromDBTypeName.
		"TIMESTAMPDTY":     kind.Datetime,
		"TIMESTAMPTZ_DTY":  kind.Datetime,
		"TIMESTAMPLTZ_DTY": kind.Datetime,
		"IBFLOAT":          kind.Float,
		"IBDOUBLE":         kind.Float,
		"INTERVALDS_DTY":   kind.Text,
		"INTERVALYM_DTY":   kind.Text,

		// Interval types (mapped to Text).
		"INTERVAL DAY TO SECOND":       kind.Text,
		"INTERVAL DAY(2) TO SECOND(6)": kind.Text,
		"INTERVAL YEAR TO MONTH":       kind.Text,
		"INTERVAL YEAR(2) TO MONTH":    kind.Text,

		// Genuinely unknown types still default to Unknown.
		"MYSTERIOUS_TYPE": kind.Unknown,
	}

	for dbTypeName, want := range testCases {
		t.Run(dbTypeName, func(t *testing.T) {
			t.Parallel()
			got := kindFromDBTypeName(nil, "col", dbTypeName)
			require.Equal(t, want, got, "%q should map to %s, got %s", dbTypeName, want, got)
		})
	}
}

func TestParseSemver(t *testing.T) {
	testCases := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{raw: "23.26.1.0.0", want: "v23.26.1"}, // five-part; regex caps at three
		{raw: "19.0.0.0.0", want: "v19.0.0"},
		{raw: "23.5.0.24.07", want: "v23.5.0"},
		{raw: "not-a-version", wantErr: true},
		{raw: "", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := parseSemver(tc.raw)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
