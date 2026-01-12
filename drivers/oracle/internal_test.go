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
