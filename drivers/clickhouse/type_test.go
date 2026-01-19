// Package clickhouse_test contains external tests for the ClickHouse driver.
//
// This file (type_test.go) contains integration tests that verify type mapping
// and roundtrip behavior between sq's kind.Kind system and ClickHouse's native
// types. These tests require a live ClickHouse instance and are skipped in
// short mode.
//
// Test categories:
//   - TestTypeMapping: Verifies sq kinds roundtrip through ClickHouse correctly
//   - TestTypeMapping_ClickHouseSpecific: Tests native ClickHouse types (UInt*, Int*, etc.)
//   - TestTypeMapping_Nullable: Tests Nullable(T) type handling and NULL values
package clickhouse_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/clickhouse"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// TestTypeMapping verifies that sq kind.Kind values correctly roundtrip through
// ClickHouse. This is an integration test that:
//
//  1. Creates a table with columns for each testable kind
//  2. Inserts a row with test values
//  3. Queries the data back
//  4. Verifies that column metadata reports the correct kind
//
// Note: Some kinds cannot fully roundtrip due to ClickHouse limitations:
//   - kind.Time -> DateTime (no time-only type) -> kind.Datetime
//   - kind.Bytes -> String (binary as String) -> kind.Text
//
// These limitations are documented in the README's "Known Limitations" section.
func TestTypeMapping(t *testing.T) {
	tu.SkipShort(t, true)

	th, src, drvr, _, db := testh.NewWith(t, sakila.CH)

	tblName := stringz.UniqTableName(t.Name())

	// Test various kind mappings
	testCases := []struct {
		colName  string
		kind     kind.Kind
		testVal  any
		expected any
	}{
		{"col_int", kind.Int, int64(42), int64(42)},
		{"col_text", kind.Text, "hello", "hello"},
		{"col_float", kind.Float, 3.14, 3.14},
		{"col_bool", kind.Bool, true, true},
		{"col_decimal", kind.Decimal, "123.45", "123.45"},
		{"col_datetime", kind.Datetime, "2023-01-15 10:30:00", nil}, // Will be parsed
		{"col_date", kind.Date, "2023-01-15", nil},                  // Will be parsed
	}

	colNames := make([]string, len(testCases))
	colKinds := make([]kind.Kind, len(testCases))
	for i, tc := range testCases {
		colNames[i] = tc.colName
		colKinds[i] = tc.kind
	}

	tblDef := schema.NewTable(tblName, colNames, colKinds)
	err := drvr.CreateTable(th.Context, db, tblDef)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	// Build insert statement
	var colParts, placeholderParts []string
	values := make([]any, len(testCases))

	for i, tc := range testCases {
		colParts = append(colParts, stringz.BacktickQuote(tc.colName))
		placeholderParts = append(placeholderParts, "?")
		values[i] = tc.testVal
	}

	insertStmt := "INSERT INTO " + stringz.BacktickQuote(tblName) +
		" (" + strings.Join(colParts, ", ") + ") VALUES (" +
		strings.Join(placeholderParts, ", ") + ")"

	_ = th.ExecSQL(src, insertStmt, values...)
	// Note: ClickHouse driver may return 0 for affected rows on INSERT

	// Query back and verify types
	sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+stringz.BacktickQuote(tblName))
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
	require.Equal(t, len(colNames), len(sink.RecMeta))

	// Verify column kinds in metadata
	for i, tc := range testCases {
		require.Equal(t, tc.kind, sink.RecMeta[i].Kind(),
			"Kind mismatch for column %s", tc.colName)
	}
}

// TestTypeMapping_ClickHouseSpecific tests the mapping of ClickHouse's native
// types to sq kinds. Unlike TestTypeMapping which starts with sq kinds, this
// test creates a table using raw ClickHouse types and verifies they map to
// the correct sq kinds when read back.
//
// This is important because ClickHouse has many types that map to the same
// sq kind:
//   - Int8, Int16, Int32, Int64, UInt8, UInt16, UInt32, UInt64 -> kind.Int
//   - Float32, Float64 -> kind.Float
//   - String, FixedString(N), UUID -> kind.Text
//   - Date, Date32 -> kind.Date
//   - DateTime, DateTime64 -> kind.Datetime
//
// The test verifies that all these types are correctly recognized and mapped.
func TestTypeMapping_ClickHouseSpecific(t *testing.T) {
	tu.SkipShort(t, true)

	th, src, _, _, _ := testh.NewWith(t, sakila.CH)

	// Create a table with all major ClickHouse types using raw SQL
	tblName := stringz.UniqTableName(t.Name())
	createStmt := `
		CREATE TABLE ` + stringz.BacktickQuote(tblName) + ` (
			col_uint8 UInt8,
			col_uint16 UInt16,
			col_uint32 UInt32,
			col_uint64 UInt64,
			col_int8 Int8,
			col_int16 Int16,
			col_int32 Int32,
			col_int64 Int64,
			col_float32 Float32,
			col_float64 Float64,
			col_string String,
			col_fixed_string FixedString(10),
			col_date Date,
			col_datetime DateTime,
			col_uuid UUID
		) ENGINE = MergeTree()
		ORDER BY col_int64
	`

	affected := th.ExecSQL(src, createStmt)
	require.Equal(t, int64(0), affected)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	// Insert test data
	insertStmt := `
		INSERT INTO ` + stringz.BacktickQuote(tblName) + ` VALUES (
			1, 1000, 100000, 1000000000,
			-1, -1000, -100000, -1000000000,
			3.14, 2.71828,
			'test string', 'fixed',
			'2023-01-15', '2023-01-15 10:30:00',
			'550e8400-e29b-41d4-a716-446655440000'
		)
	`

	_ = th.ExecSQL(src, insertStmt)
	// Note: ClickHouse driver may return 0 for affected rows on INSERT

	// Query back and verify
	sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+stringz.BacktickQuote(tblName))
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))

	// Verify kinds are correctly mapped
	expectedKinds := []kind.Kind{
		kind.Int,      // UInt8
		kind.Int,      // UInt16
		kind.Int,      // UInt32
		kind.Int,      // UInt64
		kind.Int,      // Int8
		kind.Int,      // Int16
		kind.Int,      // Int32
		kind.Int,      // Int64
		kind.Float,    // Float32
		kind.Float,    // Float64
		kind.Text,     // String
		kind.Text,     // FixedString
		kind.Date,     // Date
		kind.Datetime, // DateTime
		kind.Text,     // UUID (mapped to text)
	}

	for i, expectedKind := range expectedKinds {
		actualKind := sink.RecMeta[i].Kind()
		require.Equal(t, expectedKind, actualKind,
			"Kind mismatch for column %s", sink.RecMeta[i].Name())
	}
}

// TestTypeMapping_Nullable tests ClickHouse's Nullable(T) type wrapper handling.
//
// Unlike most SQL databases where columns are nullable by default, ClickHouse
// columns are non-nullable by default. The Nullable(T) wrapper must be used
// to allow NULL values. This test verifies:
//
//  1. Non-nullable columns are correctly identified as non-nullable in metadata
//  2. Nullable columns are correctly identified as nullable in metadata
//  3. NULL values can be inserted and retrieved from nullable columns
//  4. The kind is correctly determined regardless of nullability
//
// The test creates a table with one non-nullable String column and one
// Nullable(String) column, inserts a row with a NULL value, and verifies
// the metadata and data are correct.
func TestTypeMapping_Nullable(t *testing.T) {
	tu.SkipShort(t, true)

	th, src, _, _, _ := testh.NewWith(t, sakila.CH)

	tblName := stringz.UniqTableName(t.Name())
	createStmt := `
		CREATE TABLE ` + stringz.BacktickQuote(tblName) + ` (
			col_not_null String,
			col_nullable Nullable(String)
		) ENGINE = MergeTree()
		ORDER BY col_not_null
	`

	affected := th.ExecSQL(src, createStmt)
	require.Equal(t, int64(0), affected)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	// Insert with NULL value
	insertStmt := "INSERT INTO " + stringz.BacktickQuote(tblName) + " VALUES (?, ?)"
	_ = th.ExecSQL(src, insertStmt, "test", nil)
	// Note: ClickHouse driver may return 0 for affected rows on INSERT

	// Query back
	sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+stringz.BacktickQuote(tblName))
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))

	// Verify nullable metadata
	nullable0, ok := sink.RecMeta[0].Nullable()
	require.True(t, ok)
	require.False(t, nullable0, "col_not_null should not be nullable")

	nullable1, ok := sink.RecMeta[1].Nullable()
	require.True(t, ok)
	require.True(t, nullable1, "col_nullable should be nullable")

	// Verify NULL value
	require.NotNil(t, sink.Recs[0][0])
	require.Nil(t, sink.Recs[0][1])
}

var _ clickhouse.Provider // Ensure package is imported
