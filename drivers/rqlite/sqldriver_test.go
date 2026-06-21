package rqlite

import (
	"database/sql/driver"
	"io"
	"reflect"
	"testing"
	"unsafe"

	"github.com/rqlite/gorqlite"
	"github.com/rqlite/gorqlite/stdlib"
	"github.com/stretchr/testify/require"
)

// setQueryResultField sets an unexported field of gorqlite.QueryResult
// so tests can fabricate wire-format results without a live server.
// The same JSON-decoded value shapes rqlite produces over HTTP (nil,
// bool, float64, string) are injected directly.
func setQueryResultField(t *testing.T, qr *gorqlite.QueryResult, name string, val any) {
	t.Helper()
	f := reflect.ValueOf(qr).Elem().FieldByName(name)
	require.True(t, f.IsValid(), "gorqlite.QueryResult field {%s} not found", name)
	reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem().Set(reflect.ValueOf(val))
}

// newTestRows fabricates a driver.Rows wrapping a gorqlite QueryResult
// populated with the given columns, rqlite types, and raw JSON-decoded
// values, mirroring what a server response would produce.
func newTestRows(t *testing.T, cols, types []string, values [][]any) driver.Rows {
	t.Helper()
	qr := gorqlite.QueryResult{}
	setQueryResultField(t, &qr, "columns", cols)
	setQueryResultField(t, &qr, "types", types)
	setQueryResultField(t, &qr, "values", values)
	// gorqlite's parseQueryResult initializes rowNumber to -1.
	setQueryResultField(t, &qr, "rowNumber", int64(-1))
	return wrapRows(&stdlib.Rows{QueryResult: qr})
}

// nextRow advances rows by one and returns the row values.
func nextRow(t *testing.T, rows driver.Rows, numCols int) []driver.Value {
	t.Helper()
	dest := make([]driver.Value, numCols)
	require.NoError(t, rows.Next(dest))
	return dest
}

// Test_sqRows_Next_BooleanWire verifies that BOOLEAN cells delivered
// as raw JSON numbers (older rqlite servers) become bools, while JSON
// bools (rqlite v10+ server-side conversion), strings, and NULLs pass
// through. Without driver conversion, the float64 shape fails
// downstream: sql.NullBool.Scan(float64) errors (gh775).
func Test_sqRows_Next_BooleanWire(t *testing.T) {
	rows := newTestRows(t,
		[]string{"b"}, []string{"boolean"},
		[][]any{
			{float64(1)},
			{float64(0)},
			{float64(2)}, // SQLite truthiness: any non-zero is true.
			{true},
			{"true"},
			{nil},
		})

	require.Equal(t, []driver.Value{true}, nextRow(t, rows, 1))
	require.Equal(t, []driver.Value{false}, nextRow(t, rows, 1))
	require.Equal(t, []driver.Value{true}, nextRow(t, rows, 1))
	require.Equal(t, []driver.Value{true}, nextRow(t, rows, 1))
	require.Equal(t, []driver.Value{"true"}, nextRow(t, rows, 1))
	require.Equal(t, []driver.Value{nil}, nextRow(t, rows, 1))
	require.ErrorIs(t, rows.Next(make([]driver.Value, 1)), io.EOF)
}

// Test_sqRows_Next_DatetimeWire verifies that DATE/DATETIME cells
// reach the scan layer as raw values, bypassing gorqlite's toTime
// pre-conversion. toTime knows only two layouts and errors on
// anything else, which aborts the whole result set via Rows.Next
// (gh775); the driver's nullTime scanner (nulltime.go) holds the full
// SQLite format list, so the raw string must be passed through.
func Test_sqRows_Next_DatetimeWire(t *testing.T) {
	rows := newTestRows(t,
		[]string{"d", "dt"}, []string{"date", "datetime"},
		[][]any{
			// Bare date + canonical mattn/go-sqlite3 fractional seconds:
			// both make gorqlite's toTime error.
			{"2024-06-01", "2024-06-01 12:00:00.123"},
			// Formats toTime happens to know must also pass through raw.
			{"2024-06-01 00:00:00", "2024-06-01T12:00:00Z"},
			// REAL (e.g. Julian day) and NULL storage pass through.
			{2460462.5, nil},
		})

	require.Equal(t,
		[]driver.Value{"2024-06-01", "2024-06-01 12:00:00.123"},
		nextRow(t, rows, 2))
	require.Equal(t,
		[]driver.Value{"2024-06-01 00:00:00", "2024-06-01T12:00:00Z"},
		nextRow(t, rows, 2))
	require.Equal(t,
		[]driver.Value{2460462.5, nil},
		nextRow(t, rows, 2))
	require.ErrorIs(t, rows.Next(make([]driver.Value, 2)), io.EOF)
}

// Test_sqRows_Next_BlobWire verifies that BLOB cells, which rqlite's
// JSON API returns base64-encoded, are decoded to raw bytes. A string
// that isn't valid base64 can only be TEXT stored in a BLOB-typed
// column, and is surfaced as its literal bytes; other storage classes
// pass through.
func Test_sqRows_Next_BlobWire(t *testing.T) {
	rows := newTestRows(t,
		[]string{"bl"}, []string{"blob"},
		[][]any{
			{"3q2+7w=="}, // X'DEADBEEF'
			{""},         // X''
			{"not base64!!"},
			{nil},
			{float64(42)}, // INTEGER stored in a BLOB column.
		})

	require.Equal(t, []driver.Value{[]byte{0xDE, 0xAD, 0xBE, 0xEF}}, nextRow(t, rows, 1))
	require.Equal(t, []driver.Value{[]byte{}}, nextRow(t, rows, 1))
	require.Equal(t, []driver.Value{[]byte("not base64!!")}, nextRow(t, rows, 1))
	require.Equal(t, []driver.Value{nil}, nextRow(t, rows, 1))
	require.Equal(t, []driver.Value{float64(42)}, nextRow(t, rows, 1))
	require.ErrorIs(t, rows.Next(make([]driver.Value, 1)), io.EOF)
}

// Test_rawValues_GorqliteLayout guards the reflect+unsafe access that
// rawValues performs on gorqlite.QueryResult's unexported values
// field. If gorqlite renames or retypes the field, this test fails,
// flagging that sqRows.Next can no longer read raw row values.
func Test_rawValues_GorqliteLayout(t *testing.T) {
	vals, ok := rawValues(&gorqlite.QueryResult{})
	require.True(t, ok, "gorqlite.QueryResult no longer has a values [][]any field")
	require.Nil(t, vals)

	want := [][]any{{float64(1), "a"}}
	qr := gorqlite.QueryResult{}
	setQueryResultField(t, &qr, "values", want)
	vals, ok = rawValues(&qr)
	require.True(t, ok)
	require.Equal(t, want, vals)
}

// Test_sqStmt_CheckNamedValue verifies parameter conversion,
// especially the []byte-to-JSON-byte-array re-encoding that keeps
// blobs from being stored as base64 TEXT (gh775).
func Test_sqStmt_CheckNamedValue(t *testing.T) {
	testCases := []struct {
		in   any
		want any
	}{
		{in: []byte{0xDE, 0xAD, 0xBE, 0xEF}, want: []int{222, 173, 190, 239}},
		{in: []byte{}, want: []int{}},
		{in: []byte(nil), want: nil},
		{in: nil, want: nil},
		{in: int64(7), want: int64(7)},
		{in: "hello", want: "hello"},
		{in: true, want: true},
		{in: 1.5, want: 1.5},
		// DefaultParameterConverter normalizes sub-int64 ints.
		{in: 7, want: int64(7)},
	}

	s := &sqStmt{}
	for _, tc := range testCases {
		nv := &driver.NamedValue{Ordinal: 1, Value: tc.in}
		require.NoError(t, s.CheckNamedValue(nv), "in: %#v", tc.in)
		require.Equal(t, tc.want, nv.Value, "in: %#v", tc.in)
	}
}

// Test_sqStmt_CheckNamedValue_ConvertError covers the branch where
// driver.DefaultParameterConverter rejects an unconvertible argument.
func Test_sqStmt_CheckNamedValue_ConvertError(t *testing.T) {
	s := &sqStmt{}
	nv := &driver.NamedValue{Ordinal: 1, Value: make(chan int)}
	require.Error(t, s.CheckNamedValue(nv))
}

// Test_sqRows_ColumnTypeDatabaseTypeName covers in-range lookups and the
// out-of-range guards (which mirror the database/sql default of "").
func Test_sqRows_ColumnTypeDatabaseTypeName(t *testing.T) {
	rows := newTestRows(t,
		[]string{"a", "b"}, []string{"INTEGER", "TEXT"},
		[][]any{{float64(1), "x"}})
	sr, ok := rows.(*sqRows)
	require.True(t, ok)

	require.Equal(t, "INTEGER", sr.ColumnTypeDatabaseTypeName(0))
	require.Equal(t, "TEXT", sr.ColumnTypeDatabaseTypeName(1))
	require.Equal(t, "", sr.ColumnTypeDatabaseTypeName(-1))
	require.Equal(t, "", sr.ColumnTypeDatabaseTypeName(99))
}

// Test_sqRows_Next_PassThrough verifies that columns outside the three
// affected type classes are delivered unchanged, including columns with
// parameterized type names and expression columns with no type at all.
func Test_sqRows_Next_PassThrough(t *testing.T) {
	rows := newTestRows(t,
		[]string{"i", "txt", "expr"}, []string{"integer", "varchar(255)", ""},
		[][]any{
			{float64(7), "hello", "x"},
			{nil, nil, nil},
		})

	require.Equal(t, []driver.Value{float64(7), "hello", "x"}, nextRow(t, rows, 3))
	require.Equal(t, []driver.Value{nil, nil, nil}, nextRow(t, rows, 3))
	require.ErrorIs(t, rows.Next(make([]driver.Value, 3)), io.EOF)
}
