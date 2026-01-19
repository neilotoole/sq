package clickhouse_test

import (
	"testing"

	"github.com/neilotoole/sq/testh/sakila"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/clickhouse"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
)

var _ clickhouse.Provider // Ensure package is imported

// TestSmoke is a basic smoke test for ClickHouse connectivity.
func TestSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	th := testh.New(t)
	src := th.Source(sakila.CH)

	// Test basic connectivity by querying version
	sink, err := th.QuerySQL(src, nil, "SELECT version()")
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
	t.Logf("ClickHouse version: %v", sink.Recs[0][0])
}

// TestDriver_CreateTable tests basic table creation.
func TestDriver_CreateTable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	const handle = "@clickhouse_test"
	th, src, drvr, _, db := testh.NewWith(t, handle)

	tblName := stringz.UniqTableName(t.Name())
	colNames := []string{"col_id", "col_name", "col_value"}
	colKinds := []kind.Kind{kind.Int, kind.Text, kind.Float}

	tblDef := schema.NewTable(tblName, colNames, colKinds)

	err := drvr.CreateTable(th.Context, db, tblDef)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	// Insert a row
	insertStmt := "INSERT INTO " + stringz.BacktickQuote(tblName) + " (col_id, col_name, col_value) VALUES (?, ?, ?)"
	_ = th.ExecSQL(src, insertStmt, 1, "test", 3.14)
	// Note: ClickHouse driver may return 0 for affected rows on INSERT

	// Query back
	sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+stringz.BacktickQuote(tblName))
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
	require.Equal(t, len(colNames), len(sink.RecMeta))
}

// TestDriver_TableColumnTypes tests retrieving column type information.
func TestDriver_TableColumnTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	const handle = "@clickhouse_test"
	th, src, drvr, _, db := testh.NewWith(t, handle)

	tblName := stringz.UniqTableName(t.Name())
	colNames, colKinds := fixt.ColNamePerKind(false, false, false)

	tblDef := schema.NewTable(tblName, colNames, colKinds)
	err := drvr.CreateTable(th.Context, db, tblDef)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	colTypes, err := drvr.TableColumnTypes(th.Context, db, tblName, nil)
	require.NoError(t, err)
	require.Equal(t, len(colNames), len(colTypes))

	// Verify each column has correct name
	for i, colType := range colTypes {
		require.Equal(t, colNames[i], colType.Name())
	}
}

// TestDriver_CopyTable tests table copying functionality.
func TestDriver_CopyTable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	const handle = "@clickhouse_test"
	th, src, drvr, _, db := testh.NewWith(t, handle)

	srcTblName := stringz.UniqTableName(t.Name() + "_src")
	destTblName := stringz.UniqTableName(t.Name() + "_dest")

	// Create source table
	colNames := []string{"id", "name"}
	colKinds := []kind.Kind{kind.Int, kind.Text}
	tblDef := schema.NewTable(srcTblName, colNames, colKinds)

	err := drvr.CreateTable(th.Context, db, tblDef)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(srcTblName)) })

	// Insert test data
	insertStmt := "INSERT INTO " + stringz.BacktickQuote(srcTblName) + " (id, name) VALUES (?, ?)"
	_ = th.ExecSQL(src, insertStmt, 1, "Alice")
	// Note: ClickHouse driver may return 0 for affected rows on INSERT

	// Copy table with data
	_, err = drvr.CopyTable(th.Context, db, tablefq.From(srcTblName), tablefq.From(destTblName), true)
	require.NoError(t, err)
	// Note: ClickHouse driver may return 0 for copied rows count
	t.Cleanup(func() { th.DropTable(src, tablefq.From(destTblName)) })

	// Verify destination table has data
	sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+stringz.BacktickQuote(destTblName))
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
}
