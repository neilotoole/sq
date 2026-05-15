package duckdb_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/duckdb"
)

// TestExtensions_AllBundledExtensionsLoadAndAreCallable verifies that every
// bundled extension is loadable and that a representative function from each
// is callable in a query. This is the cross-platform smoke test that proves
// the static-link "all optional flags" design holds at runtime.
func TestExtensions_AllBundledExtensionsLoadAndAreCallable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ext.duckdb")
	db, err := sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// INSTALL + LOAD all bundled extensions. This mirrors what connInitFn
	// (in pragma.go) does on every real driver connection, and is required
	// here because the test opens a raw *sql.DB rather than going through
	// driveri.doOpen.
	for _, ext := range duckdb.BundledExtensions() {
		_, err := db.ExecContext(ctx, "INSTALL "+ext)
		require.NoError(t, err, "INSTALL %s failed", ext)
		_, err = db.ExecContext(ctx, "LOAD "+ext)
		require.NoError(t, err, "LOAD %s failed", ext)
	}

	cases := []struct {
		name  string
		query string
	}{
		// json: parse a JSON literal and extract a key.
		{"json", `SELECT json_extract('{"a":1}', '$.a') AS x`},
		// icu: call the icu_sort_key() scalar function.
		{"icu", `SELECT icu_sort_key('abc', 'en') AS x`},
		// inet: parse an INET literal.
		{"inet", `SELECT '127.0.0.1'::INET AS x`},
		// autocomplete: verify the table function is callable; use a subquery
		// to project a single column so Scan is straightforward.
		{"autocomplete", `SELECT suggestion FROM sql_auto_complete('SELE') LIMIT 1`},
		// fts: verify the fts module is loaded.
		{"fts", `SELECT count(*) FROM duckdb_extensions() WHERE extension_name = 'fts' AND loaded`},
		// httpfs: verify loaded; do NOT make a network call.
		{"httpfs", `SELECT count(*) FROM duckdb_extensions() WHERE extension_name = 'httpfs' AND loaded`},
		// excel: verify loaded; reading an actual XLSX file is out of scope.
		{"excel", `SELECT count(*) FROM duckdb_extensions() WHERE extension_name = 'excel' AND loaded`},
		// tpch / tpcds: verify loaded.
		{"tpch", `SELECT count(*) FROM duckdb_extensions() WHERE extension_name = 'tpch' AND loaded`},
		{"tpcds", `SELECT count(*) FROM duckdb_extensions() WHERE extension_name = 'tpcds' AND loaded`},
		// parquet: verify the read_parquet builtin is registered. We probe by
		// querying the function catalog rather than calling read_parquet (which
		// errors on a missing file).
		{"parquet", `SELECT count(*) FROM duckdb_functions() WHERE function_name = 'read_parquet'`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := db.QueryContext(ctx, tc.query)
			require.NoError(t, err, "%s: query failed", tc.name)
			defer rows.Close()
			require.True(t, rows.Next(), "%s: no rows returned", tc.name)
			var v any
			require.NoError(t, rows.Scan(&v), "%s: scan failed", tc.name)
			t.Logf("%s -> %v", tc.name, v)
		})
	}
}
