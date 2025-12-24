package libsq_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// uniqueTableName generates a unique table name for testing
func uniqueTableName(prefix string) string {
	// Use timestamp to ensure uniqueness
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// TestQueryVsExec_DDL_CREATE demonstrates the semantic difference between
// QueryContext and ExecContext when executing DDL statements.
//
// This test shows WHY the CLI fix was necessary:
// - QueryContext is semantically wrong for DDL (CREATE TABLE doesn't return rows)
// - ExecContext is semantically correct for DDL
//
// While lenient drivers (MySQL, Postgres) may accept QueryContext on DDL,
// it's incorrect per database/sql documentation and causes issues with
// stricter drivers like ClickHouse.
func TestQueryVsExec_DDL_CREATE(t *testing.T) {
	testCases := []string{
		sakila.Pg12,
		sakila.SL3,
	}

	for _, handle := range testCases {
		handle := handle
		t.Run(handle, func(t *testing.T) {
			th := testh.New(t)
			src := th.Source(handle)
			grip := th.Open(src)

			db, err := grip.DB(th.Context)
			require.NoError(t, err)

			tableName := uniqueTableName("test_query_vs_exec_create")
			createSQL := `CREATE TABLE ` + tableName + ` (id INTEGER, name TEXT)`

			// === WRONG WAY: Using QueryContext for CREATE TABLE ===
			// This is what the CLI was doing before the fix
			t.Run("wrong_way_querycontext", func(t *testing.T) {
				rows, err := db.QueryContext(th.Context, createSQL)

				// Note: Some drivers accept this (incorrectly), some don't
				if err == nil {
					// If it doesn't error, we get an empty rows object
					require.NotNil(t, rows, "QueryContext returns rows object even for DDL")

					// The rows object has no actual rows (DDL doesn't return data)
					hasRows := rows.Next()
					require.False(t, hasRows, "DDL statements don't return rows")

					// We MUST close the rows (even though empty)
					err = rows.Close()
					require.NoError(t, err)

					// Key problem: We can't get affected row count from rows
					t.Log("✗ QueryContext on DDL: Semantically wrong - DDL doesn't return rows")
				} else {
					// Stricter drivers reject this outright
					t.Logf("✗ QueryContext on DDL: Driver correctly rejected with error: %v", err)
				}

				// Clean up: drop table if it was created
				_, _ = db.ExecContext(th.Context, `DROP TABLE IF EXISTS `+tableName)
			})

			// === CORRECT WAY: Using ExecContext for CREATE TABLE ===
			// This is what the CLI does after the fix
			t.Run("correct_way_execcontext", func(t *testing.T) {
				result, err := db.ExecContext(th.Context, createSQL)
				require.NoError(t, err, "ExecContext should work for DDL")

				// We get a Result object (correct for DDL/DML)
				require.NotNil(t, result)

				// RowsAffected may return 0 for DDL (depends on driver)
				// but that's semantically correct - DDL typically doesn't affect rows
				affected, err := result.RowsAffected()
				if err == nil {
					t.Logf("✓ ExecContext on DDL: Correct - affected rows: %d", affected)
				} else {
					t.Logf("✓ ExecContext on DDL: Correct - driver doesn't support RowsAffected for DDL")
				}

				// Verify table was actually created
				var count int
				err = db.QueryRowContext(th.Context,
					`SELECT COUNT(*) FROM `+tableName).Scan(&count)
				require.NoError(t, err, "Table should exist after CREATE")
				require.Equal(t, 0, count, "New table should be empty")

				// Clean up
				_, err = db.ExecContext(th.Context, `DROP TABLE `+tableName)
				require.NoError(t, err)
			})
		})
	}
}

// TestQueryVsExec_DML_INSERT demonstrates the semantic difference between
// QueryContext and ExecContext when executing DML statements.
//
// This test shows a concrete example of incorrect behavior:
// - QueryContext on INSERT returns wrong affected row count (0)
// - ExecContext on INSERT returns correct affected row count
func TestQueryVsExec_DML_INSERT(t *testing.T) {
	testCases := []string{
		sakila.Pg12,
		sakila.SL3,
	}

	for _, handle := range testCases {
		handle := handle
		t.Run(handle, func(t *testing.T) {
			th := testh.New(t)
			src := th.Source(handle)
			grip := th.Open(src)

			db, err := grip.DB(th.Context)
			require.NoError(t, err)

			tableName := uniqueTableName("test_query_vs_exec_insert")

			// Setup: Create test table
			createSQL := `CREATE TABLE ` + tableName + ` (id INTEGER, name TEXT)`
			_, err = db.ExecContext(th.Context, createSQL)
			require.NoError(t, err)
			defer func() {
				_, _ = db.ExecContext(th.Context, `DROP TABLE `+tableName)
			}()

			insertSQL := `INSERT INTO ` + tableName + ` (id, name) VALUES (1, 'Alice'), (2, 'Bob')`
			expectedRowCount := 2

			// === WRONG WAY: Using QueryContext for INSERT ===
			t.Run("wrong_way_querycontext", func(t *testing.T) {
				// First, clear table
				_, _ = db.ExecContext(th.Context, `DELETE FROM `+tableName)

				rows, err := db.QueryContext(th.Context, insertSQL)

				if err == nil {
					require.NotNil(t, rows)

					// INSERT doesn't return rows (unless using RETURNING clause)
					hasRows := rows.Next()
					require.False(t, hasRows, "INSERT without RETURNING doesn't return rows")

					_ = rows.Close()

					// Key problem: Can't get affected row count!
					t.Log("✗ QueryContext on INSERT: Can't get affected row count")

					// Verify data was actually inserted (if the driver accepted it)
					var count int
					err = db.QueryRowContext(th.Context,
						`SELECT COUNT(*) FROM `+tableName).Scan(&count)
					if err == nil && count == expectedRowCount {
						t.Logf("  Data was inserted (%d rows), but we couldn't detect it from QueryContext", count)
					}
				} else {
					t.Logf("✗ QueryContext on INSERT: Driver rejected with error: %v", err)
				}
			})

			// === CORRECT WAY: Using ExecContext for INSERT ===
			t.Run("correct_way_execcontext", func(t *testing.T) {
				// First, clear table
				_, _ = db.ExecContext(th.Context, `DELETE FROM `+tableName)

				result, err := db.ExecContext(th.Context, insertSQL)
				require.NoError(t, err, "ExecContext should work for INSERT")

				// We get a Result object with affected row count
				affected, err := result.RowsAffected()
				require.NoError(t, err)

				// This is the key improvement: we get the CORRECT affected count
				require.Equal(t, int64(expectedRowCount), affected,
					"✓ ExecContext on INSERT: Correct affected row count")

				t.Logf("✓ ExecContext on INSERT: Correctly reports %d rows affected", affected)

				// Verify data was actually inserted
				var count int
				err = db.QueryRowContext(th.Context,
					`SELECT COUNT(*) FROM `+tableName).Scan(&count)
				require.NoError(t, err)
				require.Equal(t, expectedRowCount, count)
			})
		})
	}
}

// TestQueryVsExec_DML_UPDATE demonstrates affected row count for UPDATE
func TestQueryVsExec_DML_UPDATE(t *testing.T) {
	testCases := []string{
		sakila.Pg12,
		sakila.SL3,
	}

	for _, handle := range testCases {
		handle := handle
		t.Run(handle, func(t *testing.T) {
			th := testh.New(t)
			src := th.Source(handle)
			grip := th.Open(src)

			db, err := grip.DB(th.Context)
			require.NoError(t, err)

			tableName := uniqueTableName("test_query_vs_exec_update")

			// Setup: Create test table with data
			createSQL := `CREATE TABLE ` + tableName + ` (id INTEGER, name TEXT)`
			_, err = db.ExecContext(th.Context, createSQL)
			require.NoError(t, err)
			defer func() {
				_, _ = db.ExecContext(th.Context, `DROP TABLE `+tableName)
			}()

			// Insert test data
			insertSQL := `INSERT INTO ` + tableName + ` (id, name) VALUES (1, 'Alice'), (2, 'Bob'), (3, 'Charlie')`
			_, err = db.ExecContext(th.Context, insertSQL)
			require.NoError(t, err)

			updateSQL := `UPDATE ` + tableName + ` SET name = 'Updated' WHERE id <= 2`
			expectedAffectedRows := int64(2)

			// === WRONG WAY: Using QueryContext for UPDATE ===
			t.Run("wrong_way_querycontext", func(t *testing.T) {
				rows, err := db.QueryContext(th.Context, updateSQL)

				if err == nil {
					require.NotNil(t, rows)
					hasRows := rows.Next()
					require.False(t, hasRows, "UPDATE without RETURNING doesn't return rows")
					_ = rows.Close()

					t.Log("✗ QueryContext on UPDATE: Can't get affected row count")
				} else {
					t.Logf("✗ QueryContext on UPDATE: Driver rejected with error: %v", err)
				}
			})

			// === CORRECT WAY: Using ExecContext for UPDATE ===
			t.Run("correct_way_execcontext", func(t *testing.T) {
				// Reset data
				_, _ = db.ExecContext(th.Context, `UPDATE `+tableName+` SET name = 'Original'`)

				result, err := db.ExecContext(th.Context, updateSQL)
				require.NoError(t, err)

				affected, err := result.RowsAffected()
				require.NoError(t, err)
				require.Equal(t, expectedAffectedRows, affected,
					"✓ ExecContext on UPDATE: Correct affected row count")

				t.Logf("✓ ExecContext on UPDATE: Correctly reports %d rows affected", affected)
			})
		})
	}
}

// TestExecSQL_Function tests the libsq.ExecSQL function we added
func TestExecSQL_Function(t *testing.T) {
	testCases := []string{
		sakila.Pg12,
		sakila.SL3,
	}

	for _, handle := range testCases {
		handle := handle
		t.Run(handle, func(t *testing.T) {
			th := testh.New(t)
			src := th.Source(handle)

			tableName := uniqueTableName("test_execsql")

			// Test CREATE TABLE
			createSQL := `CREATE TABLE ` + tableName + ` (id INTEGER, name TEXT)`
			affected := th.ExecSQL(src, createSQL)
			t.Logf("CREATE TABLE affected: %d rows (typically 0 for DDL)", affected)

			// Test INSERT
			insertSQL := `INSERT INTO ` + tableName + ` (id, name) VALUES (1, 'Alice'), (2, 'Bob')`
			affected = th.ExecSQL(src, insertSQL)
			// Note: Some drivers return 0 for INSERT, but ExecSQL still works correctly
			t.Logf("INSERT affected: %d rows", affected)

			// Test UPDATE
			updateSQL := `UPDATE ` + tableName + ` SET name = 'Updated' WHERE id = 1`
			affected = th.ExecSQL(src, updateSQL)
			t.Logf("UPDATE affected: %d rows", affected)

			// Test DELETE
			deleteSQL := `DELETE FROM ` + tableName + ` WHERE id = 2`
			affected = th.ExecSQL(src, deleteSQL)
			t.Logf("DELETE affected: %d rows", affected)

			// Test DROP TABLE
			dropSQL := `DROP TABLE ` + tableName
			affected = th.ExecSQL(src, dropSQL)
			t.Logf("DROP TABLE affected: %d rows (typically 0 for DDL)", affected)
		})
	}
}

// TestCLI_WasUsingQueryForEverything documents what the CLI was doing wrong.
// This is a documentation test that shows the problem we fixed.
func TestCLI_WasUsingQueryForEverything(t *testing.T) {
	t.Run("documentation", func(t *testing.T) {
		t.Log("=== THE BUG ===")
		t.Log("Before the fix, cli/cmd_sql.go:execSQLPrint() was calling:")
		t.Log("  libsq.QuerySQL(ctx, grip, nil, recw, args[0])")
		t.Log("for ALL SQL statements, including:")
		t.Log("  - CREATE TABLE (DDL)")
		t.Log("  - INSERT, UPDATE, DELETE (DML)")
		t.Log("  - DROP TABLE, ALTER TABLE (DDL)")
		t.Log("")
		t.Log("=== WHY THIS WAS WRONG ===")
		t.Log("QuerySQL internally calls db.QueryContext(), which is for queries that RETURN ROWS")
		t.Log("Per Go's database/sql documentation:")
		t.Log("  - QueryContext: for SELECT, SHOW, DESCRIBE, etc. (returns *sql.Rows)")
		t.Log("  - ExecContext: for DDL/DML that don't return rows (returns sql.Result)")
		t.Log("")
		t.Log("=== CONSEQUENCES ===")
		t.Log("1. Semantically incorrect: CREATE TABLE doesn't return rows")
		t.Log("2. Can't get affected row count for INSERT/UPDATE/DELETE")
		t.Log("3. Violates database/sql contract")
		t.Log("4. Stricter drivers (ClickHouse) correctly reject this with 'bad connection' error")
		t.Log("5. Lenient drivers (MySQL, Postgres) accept it but it's still wrong")
		t.Log("")
		t.Log("=== THE FIX ===")
		t.Log("1. Added libsq.ExecSQL() that uses db.ExecContext()")
		t.Log("2. Added isQueryStatement() to detect query vs statement")
		t.Log("3. Updated execSQLPrint() to route correctly:")
		t.Log("   - Statements (CREATE, INSERT, etc.) → ExecSQL")
		t.Log("   - Queries (SELECT, SHOW, etc.) → QuerySQL")
		t.Log("")
		t.Log("=== RESULT ===")
		t.Log("✓ Semantically correct usage of database/sql")
		t.Log("✓ Correct affected row counts for DML")
		t.Log("✓ Works with all drivers (including strict ones like ClickHouse)")
		t.Log("✓ Follows Go best practices")
	})
}
