package sqlite3

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	mattn "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
)

// lockedDB copies the fixture file to a temp dir, and returns a connection
// to the copy that holds an exclusive lock, and a second DB on the same file
// that doesn't wait for locks. Every read through the second DB fails with
// SQLITE_BUSY until the test ends.
func lockedDB(t *testing.T, file string) (ddlOf func(tbl string) string, db *sql.DB) {
	t.Helper()
	ctx := context.Background()

	dbFile := filepath.Join(t.TempDir(), file)
	require.NoError(t, ioz.CopyFile(dbFile, filepath.Join("testdata", file), false))

	db, err := sql.Open(dbDrvr, dbFile+"?_busy_timeout=0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	ddls := map[string]string{}
	rows, err := db.QueryContext(ctx, `SELECT name, sql FROM sqlite_master WHERE type = 'table'`)
	require.NoError(t, err)
	for rows.Next() {
		var name, ddl string
		require.NoError(t, rows.Scan(&name, &ddl))
		ddls[name] = ddl
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	// Discard the pooled connection, so that the next read through db opens
	// a new connection that must read the schema under the lock.
	db.SetMaxIdleConns(0)
	db.SetMaxIdleConns(2)

	lockDB, err := sql.Open(dbDrvr, dbFile)
	require.NoError(t, err)
	t.Cleanup(func() { _ = lockDB.Close() })
	conn, err := lockDB.Conn(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	_, err = conn.ExecContext(ctx, `BEGIN EXCLUSIVE`)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = conn.ExecContext(ctx, `ROLLBACK`) })

	return func(tbl string) string { return ddls[tbl] }, db
}

// requireBusy requires that err is a SQLITE_BUSY error.
func requireBusy(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var sqliteErr mattn.Error
	require.ErrorAs(t, err, &sqliteErr)
	require.Equal(t, mattn.ErrBusy, sqliteErr.Code)
}

// TestProbeVirtualTable_BusyIsError verifies that a transient SQLite error,
// such as SQLITE_BUSY, is returned as an error rather than reported as the
// table being unopenable.
func TestProbeVirtualTable_BusyIsError(t *testing.T) {
	_, db := lockedDB(t, "vtab_unopenable.db")

	errText, err := probeVirtualTable(context.Background(), db, "fx_other")
	requireBusy(t, err)
	require.Empty(t, errText)
}

// TestVec0TableHints_BusyIsError verifies that a transient SQLite error,
// such as SQLITE_BUSY, fails vec0TableHints rather than silently dropping
// the vector hints.
func TestVec0TableHints_BusyIsError(t *testing.T) {
	ddlOf, db := lockedDB(t, "vec.db")

	vectors, cols, err := vec0TableHints(context.Background(), db, "doc_vec", ddlOf("doc_vec"))
	requireBusy(t, err)
	require.Nil(t, vectors)
	require.Nil(t, cols)
}

func TestUnopenableTable_IndexedByError(t *testing.T) {
	const ddl = `CREATE VIRTUAL TABLE v USING vec0(a FLOAT[4] INDEXED BY rescore(quantizer=bit), ` +
		`"B" FLOAT[4] INDEXED BY ivf())`

	testCases := []struct {
		name    string
		errText string
		want    string
	}{
		{
			name: "first_column",
			errText: "vec0 constructor error: could not parse vector column " +
				"'a FLOAT[4] INDEXED BY rescore(quantizer=bit)'",
			want: `vec0 column "a": unsupported index type: rescore(quantizer=bit)`,
		},
		{
			name: "quoted_second_column",
			errText: "vec0 constructor error: could not parse vector column " +
				`'"B" FLOAT[4] INDEXED BY ivf()'`,
			want: `vec0 column "B": unsupported index type: ivf()`,
		},
		{
			name:    "other_error",
			errText: "vec0 constructor error: could not parse table option 'chunk_size=7'",
			want:    "vec0 constructor error: could not parse table option 'chunk_size=7'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tbl := unopenableTable(context.Background(), "main", "v", ddl, tc.errText)
			require.Equal(t, tc.want, tbl.Error)
			require.Len(t, tbl.Columns, 2)
		})
	}
}
