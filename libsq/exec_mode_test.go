package libsq_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// TestDBExecContext_DDL_CREATE verifies that DB.ExecContext correctly handles
// CREATE TABLE statements. This test exists because QueryContext was
// previously (incorrectly) used for all SQL statements. While lenient
// drivers accept QueryContext for DDL, it's semantically wrong per
// database/sql documentation and fails on stricter drivers like ClickHouse.
func TestDBExecContext_DDL_CREATE(t *testing.T) {
	tableName := stringz.UniqTableName(t.Name())

	testCases := []struct {
		handle    string
		createSQL string
	}{
		{
			handle:    sakila.Pg,
			createSQL: `CREATE TABLE ` + tableName + ` (id INTEGER, name TEXT)`,
		},
		{
			handle:    sakila.SL3,
			createSQL: `CREATE TABLE ` + tableName + ` (id INTEGER, name TEXT)`,
		},
		{
			handle:    sakila.My,
			createSQL: `CREATE TABLE ` + tableName + ` (id INTEGER, name TEXT)`,
		},
		{
			handle:    sakila.MS,
			createSQL: `CREATE TABLE ` + tableName + ` (id INTEGER, name NVARCHAR(MAX))`,
		},
		{
			handle: sakila.CH,
			createSQL: `CREATE TABLE ` + tableName +
				` (id Int32, name String) ENGINE = MergeTree() ORDER BY id`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.handle, func(t *testing.T) {
			th := testh.New(t)
			src := th.Source(tc.handle)
			grip := th.Open(src)

			db, err := grip.DB(th.Context)
			require.NoError(t, err)

			result, err := db.ExecContext(th.Context, tc.createSQL)
			require.NoError(t, err)
			require.NotNil(t, result)
			t.Cleanup(func() {
				_, _ = db.ExecContext(th.Context, `DROP TABLE IF EXISTS `+tableName)
			})

			// Verify table was created by querying it.
			var count int
			err = db.QueryRowContext(th.Context, `SELECT COUNT(*) FROM `+tableName).Scan(&count)
			require.NoError(t, err)
			require.Equal(t, 0, count)
		})
	}
}

// TestDBExecContext_DML_INSERT verifies that DB.ExecContext correctly handles
// INSERT statements and returns accurate affected row counts.
func TestDBExecContext_DML_INSERT(t *testing.T) {
	tableName := stringz.UniqTableName(t.Name())

	testCases := []struct {
		handle           string
		createSQL        string
		wantAffectedRows int64 // Expected affected rows; ClickHouse returns 0 (see below)
	}{
		{
			handle:           sakila.Pg,
			createSQL:        `CREATE TABLE ` + tableName + ` (id INTEGER, name TEXT)`,
			wantAffectedRows: 2,
		},
		{
			handle:           sakila.SL3,
			createSQL:        `CREATE TABLE ` + tableName + ` (id INTEGER, name TEXT)`,
			wantAffectedRows: 2,
		},
		{
			handle:           sakila.My,
			createSQL:        `CREATE TABLE ` + tableName + ` (id INTEGER, name TEXT)`,
			wantAffectedRows: 2,
		},
		{
			handle:           sakila.MS,
			createSQL:        `CREATE TABLE ` + tableName + ` (id INTEGER, name NVARCHAR(MAX))`,
			wantAffectedRows: 2,
		},
		{
			handle: sakila.CH,
			createSQL: `CREATE TABLE ` + tableName +
				` (id Int32, name String) ENGINE = MergeTree() ORDER BY id`,
			// ClickHouse's protocol returns 0 from sql.Result.RowsAffected().
			// See TestExecContext_DML_UPDATE for detailed explanation.
			wantAffectedRows: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.handle, func(t *testing.T) {
			th := testh.New(t)
			src := th.Source(tc.handle)
			grip := th.Open(src)

			db, err := grip.DB(th.Context)
			require.NoError(t, err)

			_, err = db.ExecContext(th.Context, tc.createSQL)
			require.NoError(t, err)
			t.Cleanup(func() {
				_, _ = db.ExecContext(th.Context, `DROP TABLE `+tableName)
			})

			insertSQL := `INSERT INTO ` + tableName + ` (id, name) VALUES (1, 'Alice'), (2, 'Bob')`
			result, err := db.ExecContext(th.Context, insertSQL)
			require.NoError(t, err)

			affected, err := result.RowsAffected()
			require.NoError(t, err)
			require.Equal(t, tc.wantAffectedRows, affected)

			// Verify data was inserted.
			var count int
			err = db.QueryRowContext(th.Context, `SELECT COUNT(*) FROM `+tableName).Scan(&count)
			require.NoError(t, err)
			require.Equal(t, 2, count)
		})
	}
}

// TestDBExecContext_DML_UPDATE verifies that DB.ExecContext correctly handles
// UPDATE statements and returns accurate affected row counts.
func TestDBExecContext_DML_UPDATE(t *testing.T) {
	tableName := stringz.UniqTableName(t.Name())

	testCases := []struct {
		handle           string
		createSQL        string
		wantAffectedRows int64 // Expected affected rows; ClickHouse returns 0 (see below)
	}{
		{
			handle:           sakila.Pg,
			createSQL:        `CREATE TABLE ` + tableName + ` (id INTEGER, name TEXT)`,
			wantAffectedRows: 2,
		},
		{
			handle:           sakila.SL3,
			createSQL:        `CREATE TABLE ` + tableName + ` (id INTEGER, name TEXT)`,
			wantAffectedRows: 2,
		},
		{
			handle:           sakila.My,
			createSQL:        `CREATE TABLE ` + tableName + ` (id INTEGER, name TEXT)`,
			wantAffectedRows: 2,
		},
		{
			handle:           sakila.MS,
			createSQL:        `CREATE TABLE ` + tableName + ` (id INTEGER, name NVARCHAR(MAX))`,
			wantAffectedRows: 2,
		},
		{
			handle: sakila.CH,
			// ClickHouse requires lightweight update settings. See TestExecSQL_DDL_DML
			// for detailed explanation of these settings.
			createSQL: `CREATE TABLE ` + tableName +
				` (id Int32, name String) ENGINE = MergeTree() ORDER BY id` +
				` SETTINGS enable_block_number_column = 1, enable_block_offset_column = 1`,
			// ClickHouse's protocol returns 0 for affected rows from sql.Result.RowsAffected().
			// This is the raw driver behavior. Note that sq's higher-level driver methods
			// (e.g., CopyTable) return dialect.RowsAffectedUnsupported (-1) to distinguish
			// "unavailable" from "zero rows affected".
			wantAffectedRows: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.handle, func(t *testing.T) {
			th := testh.New(t)
			src := th.Source(tc.handle)
			grip := th.Open(src)

			db, err := grip.DB(th.Context)
			require.NoError(t, err)

			_, err = db.ExecContext(th.Context, tc.createSQL)
			require.NoError(t, err)
			t.Cleanup(func() {
				_, _ = db.ExecContext(th.Context, `DROP TABLE `+tableName)
			})

			insertSQL := `INSERT INTO ` + tableName + ` (id, name) VALUES (1, 'Alice'), (2, 'Bob'), (3, 'Charlie')`
			_, err = db.ExecContext(th.Context, insertSQL)
			require.NoError(t, err)

			updateSQL := `UPDATE ` + tableName + ` SET name = 'Updated' WHERE id <= 2`
			result, err := db.ExecContext(th.Context, updateSQL)
			require.NoError(t, err)

			affected, err := result.RowsAffected()
			require.NoError(t, err)
			require.Equal(t, tc.wantAffectedRows, affected)
		})
	}
}

// TestExecSQL_DDL_DML tests the libsq.ExecSQL function with DDL (CREATE, DROP)
// and DML (INSERT, UPDATE, DELETE) statements across multiple databases.
func TestExecSQL_DDL_DML(t *testing.T) {
	tableName := stringz.UniqTableName(t.Name())

	testCases := []struct {
		handle    string
		createSQL string
	}{
		{
			handle:    sakila.Pg,
			createSQL: `CREATE TABLE ` + tableName + ` (id INTEGER, name TEXT)`,
		},
		{
			handle:    sakila.SL3,
			createSQL: `CREATE TABLE ` + tableName + ` (id INTEGER, name TEXT)`,
		},
		{
			handle:    sakila.My,
			createSQL: `CREATE TABLE ` + tableName + ` (id INTEGER, name TEXT)`,
		},
		{
			handle:    sakila.MS,
			createSQL: `CREATE TABLE ` + tableName + ` (id INTEGER, name NVARCHAR(MAX))`,
		},
		{
			handle: sakila.CH,
			// ClickHouse requires special configuration for UPDATE/DELETE operations.
			//
			// Unlike traditional RDBMS, ClickHouse was designed primarily for append-only
			// analytics workloads. Historically, UPDATE and DELETE were only available as
			// asynchronous "mutations" that rewrote entire data parts in the background.
			//
			// ClickHouse 22.8+ introduced "lightweight deletes" and later "lightweight
			// updates" which execute synchronously like standard SQL, but these require
			// special table settings to materialize hidden columns that track row positions:
			//
			//   - enable_block_number_column: Materializes the _block_number column,
			//     which identifies which data block contains each row.
			//
			//   - enable_block_offset_column: Materializes the _block_offset column,
			//     which identifies the row's position within its block.
			//
			// Together, these columns allow ClickHouse to locate and modify specific rows
			// without rewriting entire data parts. Without these settings, UPDATE/DELETE
			// statements fail with: "Lightweight updates are not supported. Lightweight
			// updates are supported only for tables with materialized _block_number column."
			//
			// Note: ClickHouse UPDATE/DELETE return 0 for affected rows (unlike Postgres/
			// SQLite), but the operations do execute successfully.
			//
			// See: https://clickhouse.com/docs/en/guides/developer/lightweight-update
			// See: https://clickhouse.com/docs/en/guides/developer/lightweight-delete
			createSQL: `CREATE TABLE ` + tableName +
				` (id Int32, name String) ENGINE = MergeTree() ORDER BY id` +
				` SETTINGS enable_block_number_column = 1, enable_block_offset_column = 1`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.handle, func(t *testing.T) {
			th := testh.New(t)
			src := th.Source(tc.handle)

			// Test CREATE TABLE
			affected := th.ExecSQL(src, tc.createSQL)
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
