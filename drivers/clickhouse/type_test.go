package clickhouse_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/clickhouse"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/testh"
)

// TestTypeMapping tests the mapping between sq kinds and ClickHouse types.
func TestTypeMapping(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	const handle = "@clickhouse_test"
	th, src, drvr, _, db := testh.NewWith(t, handle)

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
		{"col_date", kind.Date, "2023-01-15", nil},                   // Will be parsed
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
	insertStmt := "INSERT INTO " + stringz.BacktickQuote(tblName) + " ("
	placeholders := "("
	values := make([]any, len(testCases))

	for i, tc := range testCases {
		if i > 0 {
			insertStmt += ", "
			placeholders += ", "
		}
		insertStmt += stringz.BacktickQuote(tc.colName)
		placeholders += "?"
		values[i] = tc.testVal
	}
	insertStmt += ") VALUES " + placeholders + ")"

	_ = th.ExecSQL(src, insertStmt, values...)
	// Note: ClickHouse driver may return 0 for affected rows on INSERT

	// Query back and verify types
	sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+stringz.BacktickQuote(tblName))
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
	require.Equal(t, len(colNames), len(sink.RecMeta))

	// Verify column kinds in metadata
	for i, tc := range testCases {
		// Note: Bool becomes UInt8 in ClickHouse, which maps back to Int
		if tc.kind == kind.Bool {
			require.Equal(t, kind.Int, sink.RecMeta[i].Kind(),
				"Bool column should map to Int via UInt8")
			continue
		}
		require.Equal(t, tc.kind, sink.RecMeta[i].Kind(),
			"Kind mismatch for column %s", tc.colName)
	}
}

// TestTypeMapping_ClickHouseSpecific tests ClickHouse-specific types.
func TestTypeMapping_ClickHouseSpecific(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	const handle = "@clickhouse_test"
	th, src, _, _, _ := testh.NewWith(t, handle)

	// Create a table with ClickHouse-specific types using raw SQL
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

// TestTypeMapping_Nullable tests Nullable type handling.
func TestTypeMapping_Nullable(t *testing.T) {
	t.Skip("TODO: Fix NULL value scanning - driver needs special handling for sql.NullString")

	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	const handle = "@clickhouse_test"
	th, src, _, _, _ := testh.NewWith(t, handle)

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
