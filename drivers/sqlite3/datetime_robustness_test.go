package sqlite3_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// TestQueryRailsDatetime is a regression test for #471: a DATETIME column
// declared with a parameterized type ("datetime(6)", as written by Rails)
// stores microsecond text that mattn/go-sqlite3 does not natively convert.
// The query must succeed, parsing valid values to time.Time and preserving
// genuinely unparseable text as a string.
func TestQueryRailsDatetime(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)
	db := th.OpenDB(src)

	// Unique table name so the test is robust against a stale table left
	// by a crashed prior run sharing this source (cf. createTypeTestTbls).
	tbl := stringz.UniqTableName("rails_dt_gh471")
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+tbl+" (id INTEGER PRIMARY KEY, created_at datetime(6))")
	require.NoError(t, err)
	th.Cleanup.Add(func() { th.DropTable(src, tablefq.From(tbl)) })

	_, err = db.ExecContext(th.Context,
		"INSERT INTO "+tbl+" (id, created_at) VALUES (1, '2024-01-15 12:34:56.123456'), (2, 'not-a-date')")
	require.NoError(t, err)

	sink := &testh.RecordSink{}
	recw := output.NewRecordWriterAdapter(th.Context, sink)
	err = libsq.QuerySQL(th.Context, th.Open(src), nil, recw, nil,
		"SELECT id, created_at FROM "+tbl+" ORDER BY id")
	require.NoError(t, err)
	_, err = recw.Wait()
	require.NoError(t, err)

	require.Len(t, sink.Recs, 2)

	gotTime, ok := sink.Recs[0][1].(time.Time)
	require.Truef(t, ok, "row 0 created_at: want time.Time, got %T", sink.Recs[0][1])
	require.True(t, time.Date(2024, 1, 15, 12, 34, 56, 123456000, time.UTC).Equal(gotTime))

	gotStr, ok := sink.Recs[1][1].(string)
	require.Truef(t, ok, "row 1 created_at: want string, got %T", sink.Recs[1][1])
	require.Equal(t, "not-a-date", gotStr)
}
