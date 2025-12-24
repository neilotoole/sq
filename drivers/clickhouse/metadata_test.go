package clickhouse_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/clickhouse"
	"github.com/neilotoole/sq/testh"
)

var _ clickhouse.Provider // Ensure package is imported

// TestMetadata_SourceMetadata tests source metadata retrieval.
func TestMetadata_SourceMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	const handle = "@clickhouse_test"
	th := testh.New(t)
	src := th.Source(handle)

	md, err := th.SourceMetadata(src)
	require.NoError(t, err)
	require.NotNil(t, md)
	require.Equal(t, handle, md.Handle)
	require.NotEmpty(t, md.DBVersion)
	t.Logf("Database version: %s", md.DBVersion)
	t.Logf("Database name: %s", md.Name)
	t.Logf("Number of tables: %d", len(md.Tables))
}

// TestMetadata_TableMetadata tests table metadata retrieval.
func TestMetadata_TableMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	const handle = "@clickhouse_test"
	th := testh.New(t)
	src := th.Source(handle)

	// Create a test table so metadata has something to retrieve
	th.ExecSQL(src, "CREATE TABLE IF NOT EXISTS test_table_meta (id Int64, name String) ENGINE = MergeTree() ORDER BY id")
	t.Cleanup(func() { th.ExecSQL(src, "DROP TABLE IF EXISTS test_table_meta") })

	md, err := th.SourceMetadata(src)
	require.NoError(t, err)
	require.NotNil(t, md)
	require.NotEmpty(t, md.Tables, "Expected at least one table")

	// Check first table metadata
	tbl := md.Tables[0]
	require.NotEmpty(t, tbl.Name)
	require.NotEmpty(t, tbl.Columns)

	t.Logf("Table: %s", tbl.Name)
	t.Logf("Columns: %d", len(tbl.Columns))
	for _, col := range tbl.Columns {
		t.Logf("  - %s: %s (kind: %s, nullable: %v)",
			col.Name, col.ColumnType, col.Kind, col.Nullable)
	}
}

// TestMetadata_Catalogs tests catalog/database listing.
func TestMetadata_Catalogs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	const handle = "@clickhouse_test"
	th, src, drvr, _, db := testh.NewWith(t, handle)

	catalogs, err := drvr.ListCatalogs(th.Context, db)
	require.NoError(t, err)
	require.NotEmpty(t, catalogs)
	t.Logf("Catalogs: %v", catalogs)

	// Test catalog exists
	currentCatalog, err := drvr.CurrentCatalog(th.Context, db)
	require.NoError(t, err)
	require.NotEmpty(t, currentCatalog)
	t.Logf("Current catalog: %s", currentCatalog)

	exists, err := drvr.CatalogExists(th.Context, db, currentCatalog)
	require.NoError(t, err)
	require.True(t, exists)

	// Test non-existent catalog
	exists, err = drvr.CatalogExists(th.Context, db, "nonexistent_db_12345")
	require.NoError(t, err)
	require.False(t, exists)

	_ = src
}

// TestMetadata_Schemas tests schema listing.
func TestMetadata_Schemas(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	const handle = "@clickhouse_test"
	th, src, drvr, _, db := testh.NewWith(t, handle)

	schemas, err := drvr.ListSchemas(th.Context, db)
	require.NoError(t, err)
	require.NotEmpty(t, schemas)
	t.Logf("Schemas: %v", schemas)

	// Test schema exists
	currentSchema, err := drvr.CurrentSchema(th.Context, db)
	require.NoError(t, err)
	require.NotEmpty(t, currentSchema)
	t.Logf("Current schema: %s", currentSchema)

	exists, err := drvr.SchemaExists(th.Context, db, currentSchema)
	require.NoError(t, err)
	require.True(t, exists)

	_ = src
}

// TestMetadata_TableNames tests table name listing.
func TestMetadata_TableNames(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	const handle = "@clickhouse_test"
	th, src, drvr, _, db := testh.NewWith(t, handle)

	// List all tables
	tables, err := drvr.ListTableNames(th.Context, db, "", true, false)
	require.NoError(t, err)
	t.Logf("Tables: %v", tables)

	// List all views
	views, err := drvr.ListTableNames(th.Context, db, "", false, true)
	require.NoError(t, err)
	t.Logf("Views: %v", views)

	// List both tables and views
	all, err := drvr.ListTableNames(th.Context, db, "", true, true)
	require.NoError(t, err)
	t.Logf("All objects: %v", all)
	require.GreaterOrEqual(t, len(all), len(tables))
	require.GreaterOrEqual(t, len(all), len(views))

	_ = src
}
