package duckdb_test

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// openDuckDB opens a fresh file-backed DuckDB source via the sq driver,
// which is the path real users take (connector init fn included). The
// optional dsnQuery is appended verbatim, e.g. "?threads=1".
func openDuckDB(t *testing.T, th *testh.Helper, name, dsnQuery string) *sql.DB {
	t.Helper()
	src := &source.Source{
		Handle:   "@ext_" + name,
		Type:     drivertype.DuckDB,
		Location: "duckdb://" + filepath.Join(t.TempDir(), name+".duckdb") + dsnQuery,
	}
	grip, err := th.DriverFor(src).Open(th.Context, src, driver.ModeReadWrite)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, grip.Close()) })
	db, err := grip.DB(th.Context)
	require.NoError(t, err)
	return db
}

// TestExtensions_OpenWithoutExtensionRepository verifies that opening a
// DuckDB source does not depend on the extension repository being
// reachable: the driver must not INSTALL anything up front. The
// statically linked extensions (json etc.) keep working, and a query
// that needs a non-static extension fails at that query with DuckDB's
// autoload error rather than failing the open.
//
// custom_extension_repository is pointed at an empty local directory so
// any INSTALL (explicit or autoinstall) fails deterministically, without
// touching the network or the user's real ~/.duckdb cache.
func TestExtensions_OpenWithoutExtensionRepository(t *testing.T) {
	th := testh.New(t)
	repo := filepath.ToSlash(t.TempDir())
	extDir := filepath.ToSlash(t.TempDir())
	db := openDuckDB(t, th, "norepo",
		"?custom_extension_repository="+repo+"&extension_directory="+extDir)

	var got string
	err := db.QueryRowContext(th.Context, `SELECT json_extract('{"a":1}', '$.a')::VARCHAR`).Scan(&got)
	require.NoError(t, err, "statically linked json extension must work without a repository")
	require.Equal(t, "1", got)

	err = db.QueryRowContext(th.Context, `SELECT '127.0.0.1'::INET::VARCHAR`).Scan(&got)
	require.Error(t, err, "inet is not statically linked; autoinstall must fail against an empty repository")
	require.ErrorContains(t, err, "inet")
}

// TestExtensions_AutoloadOnDemand verifies that every extension the driver
// docs list is usable through the sq driver with no explicit INSTALL or
// LOAD, relying on DuckDB's autoinstall_known_extensions and
// autoload_known_extensions (both default true). Each case runs a real
// query that only works if the extension is actually loaded.
//
// Non-static extensions are downloaded into ~/.duckdb on first use, so
// this test needs network access on a machine with a cold extension
// cache (as did the previous eager INSTALL). It is the only test in the
// repo with that dependency, hence the -short gate.
func TestExtensions_AutoloadOnDemand(t *testing.T) {
	tu.SkipShort(t, true)
	th := testh.New(t)
	xlsxPath := filepath.ToSlash(proj.Abs(sakila.PathXLSXActorHeader))

	cases := []struct {
		name  string
		setup []string
		query string
		want  string
	}{
		{"json", nil, `SELECT json_extract('{"a":1}', '$.a')::VARCHAR`, "1"},
		{
			"parquet",
			[]string{`COPY (SELECT 7 AS a) TO '{dir}/x.parquet' (FORMAT parquet)`},
			`SELECT a::VARCHAR FROM read_parquet('{dir}/x.parquet')`, "7",
		},
		{"icu", nil, `SELECT (icu_sort_key('abc', 'en') IS NOT NULL)::VARCHAR`, "true"},
		{"autocomplete", nil, `SELECT (count(*) > 0)::VARCHAR FROM sql_auto_complete('SELE')`, "true"},
		{"inet", nil, `SELECT host('127.0.0.1/24'::INET)`, "127.0.0.1"},
		{
			"fts",
			[]string{
				`CREATE TABLE docs (id INT, body VARCHAR)`,
				`INSERT INTO docs VALUES (1, 'quack quack'), (2, 'moo')`,
				`PRAGMA create_fts_index('docs', 'id', 'body')`,
			},
			`SELECT id::VARCHAR FROM docs WHERE fts_main_docs.match_bm25(id, 'quack') IS NOT NULL`, "1",
		},
		// DuckDB autoloads excel for reads (read_xlsx and the '.xlsx' file
		// suffix) but not for COPY ... TO 'x.xlsx'; only parquet, json, avro
		// and iceberg copy functions are in its autoload table.
		{
			"excel", nil,
			`SELECT (count(*) > 0)::VARCHAR FROM read_xlsx('{xlsx}')`, "true",
		},
		{
			"tpch",
			[]string{`CALL dbgen(sf = 0)`},
			`SELECT count(*)::VARCHAR FROM lineitem`, "0",
		},
		{
			"tpcds",
			[]string{`CALL dsdgen(sf = 0)`},
			`SELECT count(*)::VARCHAR FROM store_sales`, "0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.ToSlash(t.TempDir())
			db := openDuckDB(t, th, tc.name, "")
			for _, stmt := range tc.setup {
				_, err := db.ExecContext(th.Context, expand(stmt, dir, xlsxPath))
				require.NoError(t, err, "setup: %s", stmt)
			}
			var got string
			require.NoError(t, db.QueryRowContext(th.Context, expand(tc.query, dir, xlsxPath)).Scan(&got))
			require.Equal(t, tc.want, got)

			var loaded bool
			require.NoError(t, db.QueryRowContext(th.Context,
				`SELECT loaded FROM duckdb_extensions() WHERE extension_name = ?`, tc.name).Scan(&loaded))
			require.True(t, loaded, "%s should be loaded after use", tc.name)
		})
	}

	// httpfs has no offline entry point, so probe it separately: a request
	// to an unroutable address must fail (proving httpfs handled the URL)
	// and leave httpfs loaded. Retries and the timeout are pinned down
	// first: with DuckDB's defaults the failed request spends ~0.5 s in
	// retry backoff, and if HTTP_PROXY points at an unresponsive proxy the
	// default 30 s timeout applies per attempt.
	t.Run("httpfs", func(t *testing.T) {
		db := openDuckDB(t, th, "httpfs", "")
		_, err := db.ExecContext(th.Context, `SET http_retries = 0`)
		require.NoError(t, err)
		_, err = db.ExecContext(th.Context, `SET http_timeout = 2`)
		require.NoError(t, err)
		var got string
		err = db.QueryRowContext(th.Context,
			`SELECT * FROM read_csv('https://127.0.0.1:1/x.csv')`).Scan(&got)
		require.Error(t, err)
		var loaded bool
		require.NoError(t, db.QueryRowContext(th.Context,
			`SELECT loaded FROM duckdb_extensions() WHERE extension_name = 'httpfs'`).Scan(&loaded))
		require.True(t, loaded, "httpfs should be autoloaded for an https:// path")
	})
}

// expand substitutes the {dir} and {xlsx} placeholders used in the case
// table above.
func expand(s, dir, xlsx string) string {
	return strings.NewReplacer("{dir}", dir, "{xlsx}", xlsx).Replace(s)
}
