package duckdb_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// TestSmokeStaticBundle verifies that we can open an in-memory DuckDB, query
// the version, and that the set of statically linked extensions is the one
// the driver docs describe as available offline. Everything else is
// installed and loaded on demand by DuckDB; see the comment in
// driveri.doOpen (duckdb.go). If this set changes after a duckdb-go
// upgrade, update the driver docs (site/content/en/docs/drivers/duckdb.md)
// and the doOpen comment to match.
func TestSmokeStaticBundle(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	require.NoError(t, err)
	defer db.Close()

	var version string
	require.NoError(t, db.QueryRow("SELECT version()").Scan(&version))
	t.Logf("DuckDB version: %s", version)
	require.True(t, strings.HasPrefix(version, "v"))

	// Statically linked extensions are loaded at startup; nothing else can
	// be loaded on a fresh in-memory database that has run no LOAD. (The
	// install_mode column is not usable here: it reports REPOSITORY for a
	// static extension whenever a copy also exists in ~/.duckdb.)
	rows, err := db.Query(`SELECT extension_name FROM duckdb_extensions()
		WHERE loaded ORDER BY extension_name`)
	require.NoError(t, err)
	static, err := sqlz.RowsScanColumn[string](context.Background(), rows)
	require.NoError(t, err)
	require.Equal(t, []string{"autocomplete", "core_functions", "icu", "json", "parquet"}, static)
}
