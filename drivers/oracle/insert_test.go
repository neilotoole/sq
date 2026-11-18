package oracle_test

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/tu"
)

// insertTestCols are the columns of the table created by createInsertTestTable.
var insertTestCols = []string{"ID", "NAME", "AMOUNT", "CREATED", "DATA", "NOTES"}

// insertRow is one row of the insert test table. A nil field is NULL.
type insertRow struct {
	id      int64
	name    any
	amount  any
	created any
	data    any
	notes   any
}

func (r insertRow) vals() []any {
	return []any{r.id, r.name, r.amount, r.created, r.data, r.notes}
}

// createInsertTestTable creates a table with a column for each kind of value
// the array-bind insert path must handle, including BLOB and CLOB (sq's
// CreateTable never produces CLOB). It returns the table name.
func createInsertTestTable(t *testing.T, th *testh.Helper, s *sakilaSource) string {
	t.Helper()
	tbl := strings.ToUpper(stringz.UniqTableName("ins_ab"))
	_, err := s.db.ExecContext(th.Context, `CREATE TABLE "`+tbl+`" (
		"ID" NUMBER(19,0) NOT NULL,
		"NAME" VARCHAR2(100),
		"AMOUNT" NUMBER,
		"CREATED" TIMESTAMP,
		"DATA" BLOB,
		"NOTES" CLOB)`)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(s.src, tablefq.From(tbl)) })
	return tbl
}

// requireInsertTestRows asserts that tbl holds exactly want, matched by ID.
func requireInsertTestRows(t *testing.T, th *testh.Helper, s *sakilaSource, tbl string, want []insertRow) {
	t.Helper()
	rows, err := s.db.QueryContext(th.Context, `SELECT "ID", "NAME", TO_CHAR("AMOUNT"), "CREATED", "DATA", "NOTES"
		FROM "`+tbl+`" ORDER BY "ID"`)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rows.Close() })

	var i int
	for rows.Next() {
		require.Less(t, i, len(want), "more rows than inserted")
		w := want[i]

		var (
			id      int64
			name    sql.NullString
			amount  sql.NullString
			created sql.NullTime
			data    []byte
			notes   sql.NullString
		)
		require.NoError(t, rows.Scan(&id, &name, &amount, &created, &data, &notes))

		require.Equal(t, w.id, id)
		// NAME and AMOUNT may be inserted as a number or a string, and
		// Oracle converts to the column type, so compare as text.
		var wantName, wantAmount any
		if w.name != nil {
			wantName = fmt.Sprint(w.name)
		}
		requireNullable(t, wantName, name.Valid, name.String, "NAME of row %d", id)
		if w.amount != nil {
			wantAmount = fmt.Sprint(w.amount)
		}
		requireNullable(t, wantAmount, amount.Valid, amount.String, "AMOUNT of row %d", id)
		if w.created == nil {
			require.False(t, created.Valid, "CREATED of row %d", id)
		} else {
			require.True(t, created.Valid, "CREATED of row %d", id)
			require.Equal(t, w.created.(time.Time).Format(time.DateTime), created.Time.Format(time.DateTime),
				"CREATED of row %d", id)
		}
		if w.data == nil {
			require.Nil(t, data, "DATA of row %d", id)
		} else {
			require.Equal(t, w.data, data, "DATA of row %d", id)
		}
		requireNullable(t, w.notes, notes.Valid, notes.String, "NOTES of row %d", id)
		i++
	}
	require.NoError(t, rows.Err())
	require.Equal(t, len(want), i, "row count")
}

// requireNullable asserts that a scanned nullable string matches want, where
// a nil want means NULL.
func requireNullable(t *testing.T, want any, valid bool, got string, msgAndArgs ...any) {
	t.Helper()
	if want == nil {
		require.False(t, valid, msgAndArgs...)
		return
	}
	require.True(t, valid, msgAndArgs...)
	require.Equal(t, want, got, msgAndArgs...)
}

// TestPrepareInsertStmt_ArrayBind covers multi-row inserts, which the driver
// performs with array binding because Oracle before 23ai rejects a
// multi-row VALUES list.
func TestPrepareInsertStmt_ArrayBind(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()
	th, s := requireOracle(t)

	created := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	bigBlob := bytes.Repeat([]byte{0xAB}, 5000)
	bigClob := strings.Repeat("x", 10000)
	hugeBlob := bytes.Repeat([]byte{0xCD}, 40000)
	hugeClob := strings.Repeat("y", 40000)

	testCases := []struct {
		name string
		rows []insertRow
	}{
		{
			name: "uniform",
			rows: []insertRow{
				{1, "a", int64(10), created, []byte{1}, "n1"},
				{2, "b", int64(20), created, []byte{2}, "n2"},
				{3, "c", int64(30), created, []byte{3}, "n3"},
			},
		},
		{
			// NULLs in the first and last rows: go-ora takes a parameter's
			// type from its last non-nil value.
			name: "nulls_first_and_last",
			rows: []insertRow{
				{1, nil, nil, nil, nil, nil},
				{2, "b", int64(20), created, []byte{2}, "n2"},
				{3, nil, nil, nil, nil, nil},
			},
		},
		{
			name: "all_null_columns",
			rows: []insertRow{
				{1, nil, nil, nil, nil, nil},
				{2, nil, nil, nil, nil, nil},
			},
		},
		{
			// int64 and float64 in one column: bound one row at a time.
			name: "mixed_go_types",
			rows: []insertRow{
				{1, "a", int64(10), created, nil, nil},
				{2, "b", 2.5, created, nil, nil},
			},
		},
		{
			// A string then a number in a NUMBER column. Array-bound, this
			// inserts without error but stores a value that can't be read
			// back (ORA-03090). Bound one row at a time instead.
			name: "string_and_number",
			rows: []insertRow{
				{1, "a", "2.5", created, nil, nil},
				{2, "b", int64(10), created, nil, nil},
			},
		},
		{
			// A number then a string in a VARCHAR2 column. Array-bound, this
			// inserts without error and stores 42's NUMBER bytes as the text
			// "\xc1+". Bound one row at a time instead, storing "42".
			name: "number_and_string_in_text_column",
			rows: []insertRow{
				{1, int64(42), nil, nil, nil, nil},
				{2, "abc", nil, nil, nil, nil},
			},
		},
		{
			// Longer than the 4000-byte bind limit of MAX_STRING_SIZE
			// STANDARD, next to short values in the same array.
			name: "long_lob_values",
			rows: []insertRow{
				{1, "a", nil, nil, []byte{1}, "short"},
				{2, "b", nil, nil, bigBlob, bigClob},
				{3, "c", nil, nil, []byte{3}, "short"},
			},
		},
		{
			// Longer than the 32767-byte bind limit of MAX_STRING_SIZE
			// EXTENDED, so go-ora binds them as LONG and LONG RAW on every
			// server, next to short values in the same array.
			name: "very_long_lob_values",
			rows: []insertRow{
				{1, "a", nil, nil, []byte{1}, "short"},
				{2, "b", nil, nil, hugeBlob, hugeClob},
				{3, "c", nil, nil, []byte{3}, "short"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tbl := createInsertTestTable(t, th, s)
			execer := prepareInsertTestStmt(t, th, s, tbl, len(tc.rows))
			execInsertTestRows(t, th, execer, tc.rows)
			requireInsertTestRows(t, th, s, tbl, tc.rows)
		})
	}
}

// prepareInsertTestStmt prepares an insert of numRows rows to tbl. It
// prepares on a dedicated connection, as a batch insert does. On a *sql.DB,
// database/sql prepares the statement again on whichever pooled connection
// an Exec gets, which can hide bugs in reusing a prepared statement.
func prepareInsertTestStmt(t *testing.T, th *testh.Helper, s *sakilaSource, tbl string,
	numRows int,
) *driver.StmtExecer {
	t.Helper()
	conn, err := s.db.Conn(th.Context)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, conn.Close()) })

	execer, err := s.drvr.PrepareInsertStmt(th.Context, conn, tbl, insertTestCols, numRows)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, execer.Close()) })
	return execer
}

// execInsertTestRows inserts rows in one execution of execer.
func execInsertTestRows(t *testing.T, th *testh.Helper, execer *driver.StmtExecer, rows []insertRow) {
	t.Helper()
	var args []any
	for _, row := range rows {
		vals := row.vals()
		require.NoError(t, execer.Munge(vals))
		args = append(args, vals...)
	}

	affected, err := execer.Exec(th.Context, args...)
	require.NoError(t, err)
	require.Equal(t, int64(len(rows)), affected)
}

// TestPrepareInsertStmt_TypeChangeAcrossExecs executes one prepared insert
// several times, with a column's value type changing between executions.
// go-ora doesn't resend parameter definitions for a type change alone, so
// these executions must not reuse the prepared statement.
func TestPrepareInsertStmt_TypeChangeAcrossExecs(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()
	th, s := requireOracle(t)

	tbl := createInsertTestTable(t, th, s)
	execer := prepareInsertTestStmt(t, th, s, tbl, 2)

	execs := [][]insertRow{
		// NAME as long strings first, so a later NUMBER definition is
		// shorter and go-ora doesn't resend it.
		{
			{1, "a string longer than a NUMBER", nil, nil, nil, nil},
			{2, "another long string value", nil, nil, nil, nil},
		},
		// NAME as numbers.
		{
			{3, int64(7), nil, nil, nil, nil},
			{4, int64(8), nil, nil, nil, nil},
		},
		// DATA all NULL, then DATA as bytes no longer than the NULLs'
		// definition.
		{
			{5, nil, nil, nil, nil, nil},
			{6, nil, nil, nil, nil, nil},
		},
		{
			{7, nil, nil, nil, []byte{1}, nil},
			{8, nil, nil, nil, []byte{2}, nil},
		},
		// Back to the first execution's types, which reuses the statement.
		{
			{9, "back to strings", nil, nil, nil, nil},
			{10, "x", nil, nil, nil, nil},
		},
	}

	var want []insertRow
	for _, rows := range execs {
		execInsertTestRows(t, th, execer, rows)
		want = append(want, rows...)
	}

	requireInsertTestRows(t, th, s, tbl, want)
}

// TestNewBatchInsert_MultiBatch inserts enough rows for several full
// array-bound batches plus a final partial one, reusing each batch's
// prepared statement.
func TestNewBatchInsert_MultiBatch(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()
	th, s := requireOracle(t)

	tbl := createInsertTestTable(t, th, s)

	// With MaxBatchValues of 1000 and six columns, a full batch is 166
	// rows, so 400 rows is two full batches and a partial one of 68.
	const numRows = 400
	created := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	want := make([]insertRow, numRows)
	for i := range want {
		row := insertRow{id: int64(i + 1), name: "name", amount: int64(i), created: created}
		if i%7 == 0 {
			row.name, row.amount, row.created = nil, nil, nil
		}
		if i%5 == 0 {
			row.data, row.notes = []byte{byte(i)}, "note"
		}
		want[i] = row
	}

	conn, err := s.db.Conn(th.Context)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	bi, err := s.drvr.NewBatchInsert(th.Context, "insert", conn, s.src, tbl, insertTestCols)
	require.NoError(t, err)

	for _, row := range want {
		vals := row.vals()
		require.NoError(t, bi.Munge(vals))
		select {
		case err = <-bi.ErrCh:
			close(bi.RecordCh)
			t.Fatal(err)
		case bi.RecordCh <- vals:
		}
	}
	close(bi.RecordCh)
	require.NoError(t, <-bi.ErrCh)
	require.Equal(t, int64(numRows), bi.Written())

	requireInsertTestRows(t, th, s, tbl, want)
}
