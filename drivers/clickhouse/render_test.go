// Package clickhouse_test contains external tests for the ClickHouse driver.
//
// This file (render_test.go) tests the SQL rendering functions that generate
// ClickHouse-specific DDL and DML statements. These are unit tests that don't
// require a live ClickHouse instance.
//
// Key functions tested:
//   - dbTypeNameFromKind: Maps sq kind.Kind to ClickHouse type names
//   - buildCreateTableStmt: Generates CREATE TABLE with MergeTree engine
//   - buildUpdateStmt: Generates ALTER TABLE UPDATE syntax
package clickhouse_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/clickhouse"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
)

// TestDbTypeNameFromKind verifies the mapping from sq's kind.Kind values to
// ClickHouse type names. This mapping is used when generating CREATE TABLE
// statements to determine the appropriate ClickHouse column type.
//
// Type mapping reference:
//
//	kind.Text     -> String
//	kind.Int      -> Int64 (64-bit signed integer)
//	kind.Float    -> Float64 (double precision)
//	kind.Decimal  -> Decimal(18,4)
//	kind.Bool     -> Bool (native Bool, available since ClickHouse 21.12)
//	kind.Datetime -> DateTime
//	kind.Date     -> Date
//	kind.Time     -> DateTime (ClickHouse lacks a time-only type)
//	kind.Bytes    -> String (binary stored as String)
//	kind.Unknown  -> String (safe default)
func TestDbTypeNameFromKind(t *testing.T) {
	testCases := []struct {
		kind     kind.Kind
		wantType string
	}{
		{kind.Text, "String"},
		{kind.Int, "Int64"},
		{kind.Float, "Float64"},
		{kind.Decimal, "Decimal(18,4)"},
		{kind.Bool, "Bool"},
		{kind.Datetime, "DateTime"},
		{kind.Date, "Date"},
		{kind.Time, "DateTime"},
		{kind.Bytes, "String"},
		{kind.Unknown, "String"},
	}

	for _, tc := range testCases {
		t.Run(tc.kind.String(), func(t *testing.T) {
			gotType := clickhouse.DbTypeNameFromKind(tc.kind)
			require.Equal(t, tc.wantType, gotType)
		})
	}
}

// TestBuildCreateTableStmt tests CREATE TABLE statement generation for
// ClickHouse. The generated statements must comply with ClickHouse requirements:
//
//  1. ENGINE clause is required - this driver uses MergeTree()
//  2. ORDER BY clause is required for MergeTree - uses first NOT NULL column,
//     or tuple() if all columns are nullable
//  3. Nullable columns must be explicitly wrapped with Nullable(T)
//
// Test scenarios:
//   - All nullable columns: ORDER BY should use tuple()
//   - First column NOT NULL: ORDER BY should use that column
//   - NOT NULL column not first: ORDER BY should still find and use it
func TestBuildCreateTableStmt(t *testing.T) {
	// Scenario 1: All columns are nullable (NotNull = false, the default).
	// When all columns are nullable, ORDER BY should use tuple() since
	// ClickHouse doesn't allow nullable columns in the sorting key.
	tblDef := schema.NewTable("test_table",
		[]string{"id", "name", "value"},
		[]kind.Kind{kind.Int, kind.Text, kind.Float})

	stmt := clickhouse.BuildCreateTableStmt(tblDef)

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

	// Scenario 2: First column is NOT NULL - should use it as ORDER BY key
	tblDef2 := schema.NewTable("test_table2",
		[]string{"id", "name"},
		[]kind.Kind{kind.Int, kind.Text})
	tblDef2.Cols[0].NotNull = true // id is NOT NULL
	// name remains nullable (NotNull = false)

	stmt2 := clickhouse.BuildCreateTableStmt(tblDef2)

	// id should NOT be wrapped with Nullable
	require.Contains(t, stmt2, "`id` Int64")
	require.NotContains(t, stmt2, "`id` Nullable")

	// name should be wrapped with Nullable
	require.Contains(t, stmt2, "`name` Nullable(String)")

	// ORDER BY should use the NOT NULL column
	require.Contains(t, stmt2, "ORDER BY `id`")

	// Scenario 3: NOT NULL column is not the first column in the table
	tblDef3 := schema.NewTable("test_table3",
		[]string{"nullable_col", "not_null_col", "another_nullable"},
		[]kind.Kind{kind.Text, kind.Int, kind.Float})
	tblDef3.Cols[1].NotNull = true // not_null_col is NOT NULL

	stmt3 := clickhouse.BuildCreateTableStmt(tblDef3)

	// ORDER BY should use the first NOT NULL column, even if not first in table
	require.Contains(t, stmt3, "ORDER BY `not_null_col`")
}

// TestBuildUpdateStmt tests UPDATE statement generation using ClickHouse's
// ALTER TABLE UPDATE syntax.
//
// ClickHouse does not support standard SQL UPDATE statements. Instead, row-level
// updates are performed using ALTER TABLE ... UPDATE, which is an asynchronous
// mutation operation. The generated syntax is:
//
//	ALTER TABLE `table` UPDATE `col1` = ?, `col2` = ? [WHERE condition]
//
// Test scenarios:
//   - Update without WHERE clause: affects all rows
//   - Update with WHERE clause: filters rows to update
func TestBuildUpdateStmt(t *testing.T) {
	tblName := "test_table"
	cols := []string{"name", "value"}

	// Scenario 1: Update without WHERE clause - should affect all rows
	stmt, err := clickhouse.BuildUpdateStmt(tblName, cols, "")
	require.NoError(t, err)
	require.Contains(t, stmt, "ALTER TABLE")
	require.Contains(t, stmt, "`test_table`")
	require.Contains(t, stmt, "UPDATE")
	require.Contains(t, stmt, "`name` = ?")
	require.Contains(t, stmt, "`value` = ?")
	require.Contains(t, stmt, "WHERE 1")

	// Scenario 2: Update with WHERE clause - should filter rows
	stmt, err = clickhouse.BuildUpdateStmt(tblName, cols, "id = 123")
	require.NoError(t, err)
	require.Contains(t, stmt, "WHERE id = 123")

	// Scenario 3: Empty columns should error
	_, err = clickhouse.BuildUpdateStmt(tblName, nil, "")
	require.Error(t, err)
}
