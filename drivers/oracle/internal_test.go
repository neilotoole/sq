package oracle

import (
	"errors"
	"testing"

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

// TestHasErrCode tests the hasErrCode function.
func TestHasErrCode(t *testing.T) {
	// Test with nil error
	assert.False(t, hasErrCode(nil, 942))

	// Test with standard error (no code)
	stdErr := errors.New("standard error")
	assert.False(t, hasErrCode(stdErr, 942))
}

// TestIsErrTableNotExist tests the isErrTableNotExist function.
func TestIsErrTableNotExist(t *testing.T) {
	// Test with nil error
	assert.False(t, isErrTableNotExist(nil))

	// Test with standard error
	stdErr := errors.New("some error")
	assert.False(t, isErrTableNotExist(stdErr))
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
