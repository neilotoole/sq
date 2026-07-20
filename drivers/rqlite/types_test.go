package rqlite_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// These tests round-trip the SQLite type classes that rqlite's JSON
// wire format mishandles without driver-side conversion (gh775):
// BOOLEAN, DATE/DATETIME, and BLOB. Each test writes values both via
// raw SQL literals (covering data created by other clients, e.g. a
// sqlite3-to-rqlite migration) and via sq's own insert path, then
// reads them back through the record pipeline and asserts value-exact
// results.

// makeTypeTestTable creates a single-use table with the given column
// clause, registering cleanup. It returns the table name and an open
// helper environment.
func makeTypeTestTable(t *testing.T, colsClause string) (th *testh.Helper, src *source.Source,
	grip driver.Grip, tblName string,
) {
	t.Helper()
	th = testh.New(t)
	src = th.Source(sakila.RQ)
	grip = th.Open(src)
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName = "types_" + stringz.Uniq8()
	_, err = db.ExecContext(th.Context,
		fmt.Sprintf("CREATE TABLE %q (%s)", tblName, colsClause))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = grip.SQLDriver().DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})
	return th, src, grip, tblName
}

// insertViaSq inserts a single record through sq's prepared-insert
// path (the same path used by `sq tbl copy` and ingest), exercising
// the driver's parameter encoding.
func insertViaSq(t *testing.T, th *testh.Helper, grip driver.Grip,
	tblName string, colNames []string, vals ...any,
) {
	t.Helper()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	// PrepareInsertStmt requires a single-conn db.
	conn, err := db.Conn(th.Context)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	execer, err := grip.SQLDriver().PrepareInsertStmt(th.Context, conn, tblName, colNames, 1)
	require.NoError(t, err)
	defer func() { _ = execer.Close() }()

	munged := make([]any, len(vals))
	copy(munged, vals)
	require.NoError(t, execer.Munge(munged))
	affected, err := execer.Exec(th.Context, munged...)
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
}

// TestRoundTrip_Bool verifies that BOOLEAN columns survive the wire in
// both directions: SQLite stores booleans as integers 0/1, and rqlite's
// JSON API delivers them back as numbers (older rqlite) or JSON booleans
// (rqlite v10+). Both shapes must scan and surface as record bools.
func TestRoundTrip_Bool(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, grip, tblName := makeTypeTestTable(t, "id INTEGER, b BOOLEAN")
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	// Rows 1-3: raw SQL literals, the gh775 repro shape.
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		"INSERT INTO %q (id, b) VALUES (1, 1), (2, 0), (3, NULL)", tblName,
	))
	require.NoError(t, err)

	// Rows 4-5: sq's insert path with Go bool args.
	insertViaSq(t, th, grip, tblName, []string{"id", "b"}, int64(4), true)
	insertViaSq(t, th, grip, tblName, []string{"id", "b"}, int64(5), false)

	sink, err := th.QuerySQL(src, nil, fmt.Sprintf(
		"SELECT id, b FROM %q ORDER BY id", tblName,
	))
	require.NoError(t, err)
	require.Len(t, sink.Recs, 5)
	require.Equal(t, kind.Bool, sink.RecMeta[1].Kind())

	require.Equal(t, true, sink.Recs[0][1])
	require.Equal(t, false, sink.Recs[1][1])
	require.Nil(t, sink.Recs[2][1])
	require.Equal(t, true, sink.Recs[3][1])
	require.Equal(t, false, sink.Recs[4][1])
}

// TestRoundTrip_Datetime verifies DATE and DATETIME columns for the
// storage formats SQLite itself blesses, including the canonical
// mattn/go-sqlite3 fractional-seconds format and bare dates. Older
// rqlite servers return these strings raw; gorqlite's two-format
// toTime conversion would abort the whole result set for them, so the
// driver must take delivery of the raw value and parse it with the
// full SQLite format list (drivers/rqlite/nulltime.go).
func TestRoundTrip_Datetime(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, grip, tblName := makeTypeTestTable(t, "id INTEGER, d DATE, dt DATETIME")
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	// Raw SQL literals: bare date, mattn fractional seconds, minutes-only.
	_, err = db.ExecContext(th.Context, fmt.Sprintf(`INSERT INTO %q (id, d, dt) VALUES
(1, '2024-06-01', '2024-06-01 12:00:00.123'),
(2, '2024-06-01', '2024-06-01 12:00'),
(3, NULL, NULL)`, tblName))
	require.NoError(t, err)

	// sq's insert path with time.Time args.
	wantTS := time.Date(2024, 6, 1, 12, 0, 0, 123_000_000, time.UTC)
	wantDate := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	insertViaSq(t, th, grip, tblName, []string{"id", "d", "dt"}, int64(4), wantDate, wantTS)

	sink, err := th.QuerySQL(src, nil, fmt.Sprintf(
		"SELECT id, d, dt FROM %q ORDER BY id", tblName,
	))
	require.NoError(t, err)
	require.Len(t, sink.Recs, 4)
	require.Equal(t, kind.Date, sink.RecMeta[1].Kind())
	require.Equal(t, kind.Datetime, sink.RecMeta[2].Kind())

	requireTimeEqual := func(want time.Time, got any) {
		t.Helper()
		gotTime, ok := got.(time.Time)
		require.True(t, ok, "expected time.Time, got %T (%v)", got, got)
		require.True(t, want.Equal(gotTime), "want %s, got %s", want, gotTime)
	}

	// Row 1: bare date + fractional seconds.
	requireTimeEqual(wantDate, sink.Recs[0][1])
	requireTimeEqual(wantTS, sink.Recs[0][2])
	// Row 2: minutes-only datetime.
	requireTimeEqual(time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC), sink.Recs[1][2])
	// Row 3: NULLs.
	require.Nil(t, sink.Recs[2][1])
	require.Nil(t, sink.Recs[2][2])
	// Row 4: written via sq's own insert path.
	requireTimeEqual(wantDate, sink.Recs[3][1])
	requireTimeEqual(wantTS, sink.Recs[3][2])
}

// TestRoundTrip_Blob verifies BLOB columns in both directions. rqlite's
// JSON API returns BLOB cells base64-encoded, which the driver must
// decode; and []byte parameters must reach rqlite as true blobs (byte
// arrays in the JSON parameter encoding), not as base64 text.
func TestRoundTrip_Blob(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, grip, tblName := makeTypeTestTable(t, "id INTEGER, bl BLOB")
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	// Rows 1-3: raw SQL literals.
	_, err = db.ExecContext(th.Context, fmt.Sprintf(
		"INSERT INTO %q (id, bl) VALUES (1, X'DEADBEEF'), (2, X''), (3, NULL)", tblName,
	))
	require.NoError(t, err)

	// Rows 4-5: sq's insert path with []byte args.
	wantBytes := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	insertViaSq(t, th, grip, tblName, []string{"id", "bl"}, int64(4), wantBytes)
	insertViaSq(t, th, grip, tblName, []string{"id", "bl"}, int64(5), []byte{})

	// The sq-written rows must be stored as genuine blobs, not as the
	// base64 TEXT that gorqlite's default []byte JSON encoding produces.
	rows, err := db.QueryContext(th.Context, fmt.Sprintf(
		"SELECT id, typeof(bl) FROM %q WHERE id IN (1, 4, 5) ORDER BY id", tblName,
	))
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var typeOf string
		require.NoError(t, rows.Scan(&id, &typeOf))
		require.Equal(t, "blob", typeOf, "row %d: expected blob storage class", id)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())

	sink, err := th.QuerySQL(src, nil, fmt.Sprintf(
		"SELECT id, bl FROM %q ORDER BY id", tblName,
	))
	require.NoError(t, err)
	require.Len(t, sink.Recs, 5)
	require.Equal(t, kind.Bytes, sink.RecMeta[1].Kind())

	require.Equal(t, wantBytes, sink.Recs[0][1], "X'DEADBEEF' literal must read back byte-exact")
	require.Equal(t, []byte{}, sink.Recs[1][1], "X'' literal must read back as empty bytes")
	require.Nil(t, sink.Recs[2][1])
	require.Equal(t, wantBytes, sink.Recs[3][1], "sq-written blob must read back byte-exact")
	require.Equal(t, []byte{}, sink.Recs[4][1], "sq-written empty blob must read back as empty bytes")
}
