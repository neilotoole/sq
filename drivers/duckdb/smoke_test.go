package duckdb_test

import (
	"database/sql"
	"strings"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/require"
)

// TestSmokeStaticBundle verifies that we can open an in-memory DuckDB,
// query the version, and load each in-tree extension without network access.
// If this fails, the entire driver design's "all optional flags" story fails.
func TestSmokeStaticBundle(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	require.NoError(t, err)
	defer db.Close()

	var version string
	require.NoError(t, db.QueryRow("SELECT version()").Scan(&version))
	t.Logf("DuckDB version: %s", version)
	require.True(t, strings.HasPrefix(version, "v"))

	// Required bundled extensions per spec.
	exts := []string{"json", "parquet", "icu", "fts", "httpfs", "excel", "inet", "autocomplete", "tpch", "tpcds"}
	for _, ext := range exts {
		t.Run(ext, func(t *testing.T) {
			_, err := db.Exec("INSTALL " + ext)
			require.NoError(t, err, "INSTALL %s failed", ext)
			_, err = db.Exec("LOAD " + ext)
			require.NoError(t, err, "LOAD %s failed", ext)
		})
	}
}
