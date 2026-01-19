package clickhouse

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDialectPlaceholders tests that the dialect uses ? placeholders.
func TestDialectPlaceholders(t *testing.T) {
	d := &driveri{}
	dialect := d.Dialect()

	// ClickHouse should use ? placeholders
	// Test single column, single row
	require.Equal(t, "(?)", dialect.Placeholders(1, 1))

	// Test multiple columns, single row
	require.Equal(t, "(?, ?, ?)", dialect.Placeholders(3, 1))

	// Test single column, multiple rows
	require.Equal(t, "(?), (?), (?)", dialect.Placeholders(1, 3))

	// Test multiple columns, multiple rows
	require.Equal(t, "(?, ?), (?, ?)", dialect.Placeholders(2, 2))
}

// TestDialectEnquote tests backtick quoting.
func TestDialectEnquote(t *testing.T) {
	d := &driveri{}
	dialect := d.Dialect()

	testCases := []struct {
		input string
		want  string
	}{
		{"simple", "`simple`"},
		{"table_name", "`table_name`"},
		{"column", "`column`"},
		{"CamelCase", "`CamelCase`"},
		{"with space", "`with space`"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			got := dialect.Enquote(tc.input)
			require.Equal(t, tc.want, got)
		})
	}
}
