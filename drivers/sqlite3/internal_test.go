package sqlite3

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/tu"
)

var (
	KindFromDBTypeName = kindFromDBTypeName
	GetTblRowCounts    = getTblRowCounts
	RTypeNullTime      = rtypeNullTime
)

func TestPlaceholders(t *testing.T) {
	testCases := []struct {
		numCols int
		numRows int
		want    string
	}{
		{numCols: 0, numRows: 0, want: ""},
		{numCols: 1, numRows: 1, want: "(?)"},
		{numCols: 2, numRows: 1, want: "(?, ?)"},
		{numCols: 1, numRows: 2, want: "(?), (?)"},
		{numCols: 2, numRows: 2, want: "(?, ?), (?, ?)"},
	}

	for _, tc := range testCases {
		got := placeholders(tc.numCols, tc.numRows)
		require.Equal(t, tc.want, got)
	}
}

func TestDsnFromLocation(t *testing.T) {
	testCases := []struct {
		loc     string
		want    string
		wantErr bool
	}{
		{loc: "", wantErr: true},
		{loc: "duckdb://x", wantErr: true},
		{loc: Prefix + "/path/to/foo.db", want: "/path/to/foo.db"},
		{loc: Prefix + "/path/to/foo.db?mode=ro", want: "/path/to/foo.db?mode=ro"},
		{loc: Prefix + "/path/to/foo.db?cache=shared&mode=rw", want: "/path/to/foo.db?cache=shared&mode=rw"},
		{loc: Prefix + "/path/to/foo.db?immutable=1", want: "/path/to/foo.db?immutable=1"},
	}

	for _, tc := range testCases {
		t.Run(tu.Name(tc.loc), func(t *testing.T) {
			got, err := dsnFromLocation(tc.loc)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestGetDBPropertiesNoSideEffects verifies gh699: getDBProperties must
// not execute pragmas that mutate the database or scan it end-to-end.
// The pragma table-valued function mechanism ("SELECT * FROM pragma_x")
// executes the pragma to produce its rows: notably "SELECT * FROM
// pragma_optimize" runs ANALYZE, writing sqlite_stat1 to the db file.
// That made the read-only metadata path a writer, so concurrent
// SourceMetadata calls contended for the file write lock and flaked
// with "database is locked" (SQLITE_BUSY) on loaded CI runners.
func TestGetDBPropertiesNoSideEffects(t *testing.T) {
	ctx := context.Background()
	dbFile := filepath.Join(t.TempDir(), "gh699.db")

	db, err := sql.Open(dbDrvr, dbFile)
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()

	// Single conn, so the priming query below and the pragma reads share
	// a connection: "PRAGMA optimize" only considers tables that the
	// current connection has queried.
	db.SetMaxOpenConns(1)

	_, err = db.ExecContext(ctx, `CREATE TABLE t (a INTEGER);
WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c WHERE x<5000)
INSERT INTO t SELECT x FROM c;
CREATE INDEX idx_t_a ON t(a);`)
	require.NoError(t, err)

	// Prime the connection's query history so that, were getDBProperties
	// to execute pragma_optimize, the ANALYZE would actually fire.
	var n int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM t WHERE a = 42`).Scan(&n))
	require.Equal(t, 1, n)

	props, err := getDBProperties(ctx, db)
	require.NoError(t, err)
	require.NotEmpty(t, props)
	require.Contains(t, props, "journal_mode")

	// Side-effecting and whole-db-scanning pragmas must not be executed,
	// and thus must not appear as properties.
	for _, banned := range []string{
		"foreign_key_check", "incremental_vacuum", "integrity_check",
		"optimize", "quick_check", "wal_checkpoint",
	} {
		require.NotContains(t, props, banned)
	}

	// The acid test: the db must not have been modified. If
	// pragma_optimize executed, ANALYZE would have created sqlite_stat1.
	var statCount int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE name LIKE 'sqlite_stat%'`).Scan(&statCount))
	require.Equal(t, 0, statCount, "getDBProperties must not write to the db (ANALYZE via pragma_optimize)")
}

func TestFilePathFromLocation(t *testing.T) {
	testCases := []struct {
		loc  string
		want string
	}{
		{loc: "", want: ""},
		{loc: "duckdb:///foo.db", want: ""},
		{loc: Prefix, want: ""},
		{loc: Prefix + "/path/to/foo.db", want: "/path/to/foo.db"},
		{loc: Prefix + "/path/to/foo.db?mode=ro", want: "/path/to/foo.db"},
		{loc: Prefix + "/path/to/foo.db?cache=shared&mode=rw", want: "/path/to/foo.db"},
		{loc: Prefix + "/path/to/foo.db?immutable=1", want: "/path/to/foo.db"},
	}

	for _, tc := range testCases {
		t.Run(tu.Name(tc.loc), func(t *testing.T) {
			require.Equal(t, tc.want, filePathFromLocation(tc.loc))
		})
	}
}
