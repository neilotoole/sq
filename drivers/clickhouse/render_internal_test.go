package clickhouse

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
)

// TestDbTypeNameFromKind tests type mapping from sq kinds to ClickHouse types.
func TestDbTypeNameFromKind(t *testing.T) {
	testCases := []struct {
		kind     kind.Kind
		wantType string
	}{
		{kind.Text, "String"},
		{kind.Int, "Int64"},
		{kind.Float, "Float64"},
		{kind.Decimal, "Decimal(18,4)"},
		{kind.Bool, "UInt8"},
		{kind.Datetime, "DateTime"},
		{kind.Date, "Date"},
		{kind.Time, "DateTime"},
		{kind.Bytes, "String"},
		{kind.Unknown, "String"},
	}

	for _, tc := range testCases {
		t.Run(tc.kind.String(), func(t *testing.T) {
			gotType := dbTypeNameFromKind(tc.kind)
			require.Equal(t, tc.wantType, gotType)
		})
	}
}

// TestBuildCreateTableStmt tests CREATE TABLE statement generation.
func TestBuildCreateTableStmt(t *testing.T) {
	// Test with all nullable columns (NotNull = false, the default).
	// When all columns are nullable, ORDER BY should use tuple() since
	// ClickHouse doesn't allow nullable columns in the sorting key.
	tblDef := schema.NewTable("test_table",
		[]string{"id", "name", "value"},
		[]kind.Kind{kind.Int, kind.Text, kind.Float})

	stmt := buildCreateTableStmt(tblDef)

	// Verify statement contains expected components
	require.Contains(t, stmt, "CREATE TABLE")
	require.Contains(t, stmt, "`test_table`")
	require.Contains(t, stmt, "`id`")
	require.Contains(t, stmt, "`name`")
	require.Contains(t, stmt, "`value`")
	require.Contains(t, stmt, "ENGINE = MergeTree()")
	require.Contains(t, stmt, "ORDER BY tuple()") // All columns nullable, use tuple()

	// By default, columns should be nullable (wrapped with Nullable)
	require.Contains(t, stmt, "Nullable(Int64)")
	require.Contains(t, stmt, "Nullable(String)")
	require.Contains(t, stmt, "Nullable(Float64)")

	// Test with a NOT NULL column - should use it as ORDER BY key
	tblDef2 := schema.NewTable("test_table2",
		[]string{"id", "name"},
		[]kind.Kind{kind.Int, kind.Text})
	tblDef2.Cols[0].NotNull = true // id is NOT NULL
	// name remains nullable (NotNull = false)

	stmt2 := buildCreateTableStmt(tblDef2)

	// id should NOT be wrapped with Nullable
	require.Contains(t, stmt2, "`id` Int64")
	require.NotContains(t, stmt2, "`id` Nullable")

	// name should be wrapped with Nullable
	require.Contains(t, stmt2, "`name` Nullable(String)")

	// ORDER BY should use the NOT NULL column
	require.Contains(t, stmt2, "ORDER BY `id`")

	// Test with NOT NULL column not in first position
	tblDef3 := schema.NewTable("test_table3",
		[]string{"nullable_col", "not_null_col", "another_nullable"},
		[]kind.Kind{kind.Text, kind.Int, kind.Float})
	tblDef3.Cols[1].NotNull = true // not_null_col is NOT NULL

	stmt3 := buildCreateTableStmt(tblDef3)

	// ORDER BY should use the first NOT NULL column, even if not first in table
	require.Contains(t, stmt3, "ORDER BY `not_null_col`")
}

// TestBuildUpdateStmt tests UPDATE statement generation.
func TestBuildUpdateStmt(t *testing.T) {
	tblName := "test_table"
	cols := []string{"name", "value"}

	// Test without WHERE clause
	stmt := buildUpdateStmt(tblName, cols, "")
	require.Contains(t, stmt, "ALTER TABLE")
	require.Contains(t, stmt, "`test_table`")
	require.Contains(t, stmt, "UPDATE")
	require.Contains(t, stmt, "`name` = ?")
	require.Contains(t, stmt, "`value` = ?")
	require.NotContains(t, stmt, "WHERE")

	// Test with WHERE clause
	stmt = buildUpdateStmt(tblName, cols, "id = 123")
	require.Contains(t, stmt, "WHERE id = 123")
}
