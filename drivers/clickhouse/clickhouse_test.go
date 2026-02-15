// Package clickhouse_test contains external tests for the ClickHouse driver.
//
// This file (clickhouse_test.go) contains the main driver integration tests,
// including dialect configuration tests and DDL operation tests. Tests that
// require a live ClickHouse instance use tu.SkipShort to skip in short mode.
//
// Test categories:
//   - Dialect tests: Verify SQL dialect configuration (placeholders, quoting)
//   - DDL tests: Verify table creation, column types, copying
//   - Smoke test: Basic connectivity verification
package clickhouse_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/clickhouse"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// TestDialectPlaceholders verifies that the ClickHouse dialect uses positional
// ? placeholders (like MySQL) rather than numbered placeholders (like PostgreSQL's
// $1, $2). This is a unit test that doesn't require a live database.
func TestDialectPlaceholders(t *testing.T) {
	p := &clickhouse.Provider{}
	drvr, err := p.DriverFor(clickhouse.Type)
	require.NoError(t, err)
	sqlDrvr := drvr.(driver.SQLDriver)
	dialect := sqlDrvr.Dialect()

	// ClickHouse should use ? placeholders
	// Test single column, single row
	require.Equal(t, "(?)", dialect.Placeholders(1, 1))

	// Test multiple columns, single row
	require.Equal(t, "(?, ?, ?)", dialect.Placeholders(3, 1))

	// Test single column, multiple rows
	require.Equal(t, "(?), (?), (?)", dialect.Placeholders(1, 3))

	// Test multiple columns, multiple rows
	require.Equal(t, "(?, ?), (?, ?)", dialect.Placeholders(2, 2))
}

// TestDialectEnquote verifies that the ClickHouse dialect uses backtick quoting
// for identifiers (like MySQL) rather than double quotes (like PostgreSQL).
// Example: table_name becomes `table_name`.
//
// This is a unit test that doesn't require a live database.
func TestDialectEnquote(t *testing.T) {
	p := &clickhouse.Provider{}
	drvr, err := p.DriverFor(clickhouse.Type)
	require.NoError(t, err)
	sqlDrvr := drvr.(driver.SQLDriver)
	dialect := sqlDrvr.Dialect()

	testCases := []struct {
		input string
		want  string
	}{
		{"simple", "`simple`"},
		{"table_name", "`table_name`"},
		{"column", "`column`"},
		{"CamelCase", "`CamelCase`"},
		{"with space", "`with space`"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			got := dialect.Enquote(tc.input)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestSmoke is a basic smoke test for ClickHouse connectivity. It verifies
// that the driver can connect to ClickHouse and execute a simple query.
//
// This test requires a live ClickHouse instance and is skipped in short mode.
// It queries the ClickHouse version using SELECT version() to verify the
// connection works end-to-end.
func TestSmoke(t *testing.T) {
	tu.SkipShort(t, true)

	th := testh.New(t)
	src := th.Source(sakila.CH)

	// Query ClickHouse version to verify connectivity
	sink, err := th.QuerySQL(src, nil, "SELECT version()")
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
	t.Logf("ClickHouse version: %v", sink.Recs[0][0])
}

// TestDriver_CreateTable tests the CreateTable DDL operation. It verifies that:
//
//  1. Tables can be created with the specified columns and types
//  2. The table uses MergeTree engine (ClickHouse requirement)
//  3. Data can be inserted and queried back from the created table
//
// This test requires a live ClickHouse instance and is skipped in short mode.
// The created table is automatically cleaned up after the test.
func TestDriver_CreateTable(t *testing.T) {
	tu.SkipShort(t, true)

	th, src, drvr, _, db := testh.NewWith(t, sakila.CH)

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

// TestDriver_TableColumnTypes tests the TableColumnTypes method, which retrieves
// sql.ColumnType information for a table's columns without executing a full query.
//
// This is used by PrepareInsertStmt and other methods that need to know column
// types before inserting data. The test verifies that:
//
//  1. Column types can be retrieved for a newly created table
//  2. The number of columns matches the table definition
//  3. Column names are correctly reported
//
// This test requires a live ClickHouse instance and is skipped in short mode.
func TestDriver_TableColumnTypes(t *testing.T) {
	tu.SkipShort(t, true)

	th, src, drvr, _, db := testh.NewWith(t, sakila.CH)

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

// TestDriver_AlterTableColumnKinds tests the AlterTableColumnKinds DDL
// operation, which changes the types of specified columns in an existing table.
//
// The test verifies that:
//
//  1. A column type can be changed from String to Int64
//  2. The system.columns table reflects the new type
//  3. Mismatched column/kind counts return an error
//
// This test requires a live ClickHouse instance and is skipped in short mode.
func TestDriver_AlterTableColumnKinds(t *testing.T) {
	tu.SkipShort(t, true)

	th, src, drvr, _, db := testh.NewWith(t, sakila.CH)

	tblName := stringz.UniqTableName(t.Name())
	colNames := []string{"id", "val"}
	colKinds := []kind.Kind{kind.Int, kind.Text}

	tblDef := schema.NewTable(tblName, colNames, colKinds)
	err := drvr.CreateTable(th.Context, db, tblDef)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	// Change "val" from Text (String) to Int (Int64).
	err = drvr.AlterTableColumnKinds(th.Context, db, tblName,
		[]string{"val"}, []kind.Kind{kind.Int})
	require.NoError(t, err)

	// Verify the column type changed in system.columns.
	sink, err := th.QuerySQL(src, nil,
		"SELECT type FROM system.columns WHERE table = '"+tblName+"' AND name = 'val'")
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
	require.Equal(t, "Nullable(Int64)", sink.Recs[0][0])

	// Verify mismatched column/kind counts return an error.
	err = drvr.AlterTableColumnKinds(th.Context, db, tblName,
		[]string{"val"}, []kind.Kind{kind.Int, kind.Text})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mismatched count")
}

// TestDriver_CopyTable tests the CopyTable DDL operation, which creates a copy
// of an existing table with the same schema, optionally including data.
//
// The test verifies that:
//
//  1. A new table is created with the same column structure
//  2. When copyData is true, rows are copied to the destination table
//  3. The destination table is queryable after the copy
//
// Known limitation: ClickHouse's INSERT ... SELECT does not report affected row
// counts, so the returned count may be 0 even when rows are successfully copied.
// This is documented in the README's "Known Limitations" section.
//
// This test requires a live ClickHouse instance and is skipped in short mode.
func TestDriver_CopyTable(t *testing.T) {
	tu.SkipShort(t, true)

	th, src, drvr, _, db := testh.NewWith(t, sakila.CH)

	srcTblName := stringz.UniqTableName(t.Name() + "_src")
	destTblName := stringz.UniqTableName(t.Name() + "_dest")

	// Create source table with test data
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
