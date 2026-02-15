package libsq_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// TestQuerySQL_Smoke is a smoke test of the testh.QuerySQL helper function,
// which executes raw SQL queries against data sources and returns results in
// a testh.RecordSink.
//
// This test verifies that:
//   - Raw SQL SELECT queries execute successfully across multiple source types
//     (SQLite, MySQL, PostgreSQL, SQL Server, CSV, XLSX).
//   - The returned records have the expected field types as defined by sq's
//     type system (sqlz.RTypeInt64, sqlz.RTypeString, sqlz.RTypeTime).
//   - The actor table returns the expected number of rows (sakila.TblActorCount).
//
// The test uses the Sakila sample database's actor table, which has a consistent
// schema across all SQL sources: (actor_id INT, first_name TEXT, last_name TEXT,
// last_update TIMESTAMP). For document sources (CSV, XLSX), the schema is inferred
// but should produce equivalent Go types.
//
// This is a foundational test that validates the query execution path works
// correctly before more specific tests exercise edge cases.
func TestQuerySQL_Smoke(t *testing.T) {
	t.Parallel()

	wantActorFieldTypes := []reflect.Type{
		sqlz.RTypeInt64,
		sqlz.RTypeString,
		sqlz.RTypeString,
		sqlz.RTypeTime,
	}

	testCases := []struct {
		handle     string
		fieldTypes []reflect.Type
	}{
		{
			handle:     sakila.SL3,
			fieldTypes: wantActorFieldTypes,
		},
		{
			handle:     sakila.My,
			fieldTypes: wantActorFieldTypes,
		},
		{
			handle:     sakila.Pg,
			fieldTypes: wantActorFieldTypes,
		},
		{
			handle:     sakila.MS,
			fieldTypes: wantActorFieldTypes,
		},
		{
			handle:     sakila.CSVActor,
			fieldTypes: wantActorFieldTypes,
		},
		{
			handle:     sakila.XLSX,
			fieldTypes: wantActorFieldTypes,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.handle, func(t *testing.T) {
			tu.SkipShort(t, tc.handle == sakila.XLSX)
			t.Parallel()

			th := testh.New(t)
			src := th.Source(tc.handle)

			tblName := sakila.TblActor
			if th.IsMonotable(src) {
				tblName = source.MonotableName
			}

			sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+tblName)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
			require.Equal(t, len(tc.fieldTypes), len(sink.Recs[0]))
			for i := range sink.Recs[0] {
				require.Equal(t,
					tc.fieldTypes[i].String(),
					reflect.TypeOf(sink.Recs[0][i]).String(),
					"expected field[%d] {%s} to have type %s but got %s",
					i,
					sink.RecMeta[i].Name(),
					tc.fieldTypes[i].String(),
					reflect.TypeOf(sink.Recs[0][i]).String(),
				)
			}
		})
	}
}

// TestQuerySQL_Count verifies that aggregate SQL queries (specifically COUNT(*))
// execute correctly across all SQL database types supported by sq.
//
// This test validates two query patterns for each database:
//  1. SELECT * FROM actor - Returns all rows, verifying the row count matches
//     sakila.TblActorCount (200 rows in the standard Sakila dataset).
//  2. SELECT COUNT(*) FROM actor - Returns a single row with the count as an
//     int64, verifying the aggregate function works and type conversion is correct.
//
// The test iterates over sakila.SQLAll(), which includes PostgreSQL, MySQL,
// SQLite, SQL Server, and ClickHouse. Each database is tested in parallel.
//
// This test is important because:
//   - Aggregate functions may behave differently across databases.
//   - The int64 type assertion verifies sq's type normalization works correctly
//     (all databases should return COUNT(*) as int64, not database-specific types).
//   - It exercises both row-returning and scalar-returning query patterns.
func TestQuerySQL_Count(t *testing.T) { //nolint:tparallel
	testCases := sakila.SQLAll()
	for _, handle := range testCases {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)

			sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+sakila.TblActor)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))

			sink, err = th.QuerySQL(src, nil, "SELECT COUNT(*) FROM "+sakila.TblActor)
			require.NoError(t, err)
			count, ok := sink.Recs[0][0].(int64)
			require.True(t, ok)
			require.Equal(t, int64(sakila.TblActorCount), count)
		})
	}
}

// TestJoinDuplicateColNamesAreRenamed verifies that sq correctly handles JOIN
// queries that produce duplicate column names in the result set.
//
// When joining tables, it's common to have columns with the same name in both
// tables (e.g., actor_id, last_update). Standard SQL allows this, but it creates
// ambiguity when accessing results by column name. This test verifies that sq
// automatically renames duplicate columns to ensure uniqueness.
//
// Test setup:
//   - Executes: SELECT * FROM actor INNER JOIN film_actor ON actor.actor_id = film_actor.actor_id
//   - Without renaming, this would produce duplicate column names:
//     [actor_id, first_name, last_name, last_update, actor_id, film_id, last_update]
//   - Note the duplicate "actor_id" and "last_update" columns.
//
// Expected behavior:
//   - All column names in RecMeta.MungedNames() should be unique.
//   - Duplicate columns are renamed with a suffix (e.g., "actor_id_1", "last_update_1").
//
// This is critical for:
//   - JSON/CSV output where duplicate keys would cause data loss or invalid output.
//   - Programmatic access to results by column name.
//   - Cross-database compatibility (some databases handle this differently).
//
// The test uses SQLite (sakila.SL3) as a representative database, since column
// renaming is handled by sq's result processing layer, not the database driver.
func TestJoinDuplicateColNamesAreRenamed(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	const query = "SELECT * FROM actor INNER JOIN film_actor ON actor.actor_id = film_actor.actor_id LIMIT 1"

	sink, err := th.QuerySQL(src, nil, query)
	require.NoError(t, err)
	colNames := sink.RecMeta.MungedNames()
	// Without intervention, the returned column names would contain duplicates.
	//  [actor_id, first_name, last_name, last_update, actor_id, film_id, last_update]

	t.Logf("Cols: [%s]", strings.Join(colNames, ", "))

	colCounts := lo.CountValues(colNames)
	for col, count := range colCounts {
		assert.True(t, count == 1, "col name {%s} is not unique (occurs %d times)",
			col, count)
	}
}

// TestDBExecContext_DDL_CREATE verifies that the standard library's
// *sql.DB.ExecContext method correctly handles CREATE TABLE (DDL) statements
// across all supported SQL databases.
//
// # Background
//
// Go's database/sql package provides two primary methods for executing SQL:
//   - DB.QueryContext: For statements that return rows (SELECT, SHOW, etc.)
//   - DB.ExecContext: For statements that don't return rows (CREATE, INSERT, etc.)
//
// Previously, sq incorrectly used QueryContext for all SQL statements. While
// lenient drivers (PostgreSQL, MySQL, SQLite) tolerate this misuse, it violates
// the database/sql contract and fails on stricter drivers like ClickHouse, which
// returns an error when QueryContext is used for DDL statements.
//
// # Test Coverage
//
// This test executes CREATE TABLE statements against each SQL database type:
//   - PostgreSQL (sakila.Pg): Standard SQL syntax
//   - SQLite (sakila.SL3): Standard SQL syntax
//   - MySQL (sakila.My): Standard SQL syntax
//   - SQL Server (sakila.MS): Uses NVARCHAR(MAX) instead of TEXT
//   - ClickHouse (sakila.CH): Requires ENGINE and ORDER BY clauses
//
// # Verification
//
// For each database, the test:
//  1. Calls db.ExecContext with a CREATE TABLE statement.
//  2. Verifies no error is returned and result is non-nil.
//  3. Queries the newly created table (SELECT COUNT(*)) to confirm it exists.
//  4. Cleans up by dropping the table in t.Cleanup.
//
// # ClickHouse Specifics
//
// ClickHouse requires additional DDL syntax not needed by other databases:
//   - ENGINE = MergeTree(): Specifies the table engine (MergeTree is standard for OLAP).
//   - ORDER BY id: Required for MergeTree tables to define the primary sort key.
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

// TestDBExecContext_DML_INSERT verifies that the standard library's
// *sql.DB.ExecContext method correctly handles INSERT (DML) statements and
// returns accurate affected row counts via sql.Result.RowsAffected().
//
// # Background
//
// INSERT statements are DML (Data Manipulation Language) operations that modify
// data. Unlike DDL (CREATE, DROP), DML operations typically return a count of
// affected rows through sql.Result.RowsAffected(). This test verifies that:
//   - INSERT statements execute successfully via ExecContext.
//   - The returned sql.Result reports the correct number of inserted rows.
//   - The data is actually persisted (verified by SELECT COUNT(*)).
//
// # Test Coverage
//
// This test executes INSERT statements against each SQL database type:
//   - PostgreSQL, SQLite, MySQL, SQL Server: Expect RowsAffected() = 2
//   - ClickHouse: Expect RowsAffected() = 0 (see below)
//
// # ClickHouse Behavior
//
// ClickHouse's native protocol does not report affected row counts for INSERT
// operations. The sql.Result.RowsAffected() method always returns 0, regardless
// of how many rows were actually inserted. This is a protocol limitation, not
// an error. The test accounts for this by setting wantAffectedRows = 0 for
// ClickHouse while still verifying the INSERT succeeded via SELECT COUNT(*).
//
// Note: sq's higher-level driver methods (e.g., SQLDriver.CopyTable) return
// dialect.RowsAffectedUnsupported (-1) to distinguish "count unavailable" from
// "zero rows affected". This test exercises the raw driver behavior (0), not
// sq's abstraction layer.
//
// # Verification
//
// For each database, the test:
//  1. Creates a test table with driver-specific DDL.
//  2. Executes INSERT INTO ... VALUES (1, 'Alice'), (2, 'Bob').
//  3. Asserts RowsAffected() matches the expected value (2 or 0 for ClickHouse).
//  4. Queries SELECT COUNT(*) to verify both rows were actually inserted.
//  5. Drops the table in t.Cleanup.
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

// TestDBExecContext_DML_UPDATE verifies that the standard library's
// *sql.DB.ExecContext method correctly handles UPDATE (DML) statements and
// returns accurate affected row counts via sql.Result.RowsAffected().
//
// # Background
//
// UPDATE statements modify existing rows and return a count of affected rows
// through sql.Result.RowsAffected(). This test verifies that:
//   - UPDATE statements execute successfully via ExecContext.
//   - The returned sql.Result reports the correct number of updated rows.
//
// # Test Coverage
//
// This test executes UPDATE statements against each SQL database type:
//   - PostgreSQL, SQLite, MySQL, SQL Server: Expect RowsAffected() = 2
//   - ClickHouse: Expect RowsAffected() = 0 (see below)
//
// # ClickHouse Specifics
//
// ClickHouse requires special table settings for UPDATE operations to work:
//
//   - enable_block_number_column = 1: Materializes the _block_number hidden column.
//   - enable_block_offset_column = 1: Materializes the _block_offset hidden column.
//
// These settings enable "lightweight updates" (introduced in ClickHouse 22.8+),
// which execute synchronously like standard SQL. Without these settings, UPDATE
// statements fail with: "Lightweight updates are not supported."
//
// See TestExecSQL_DDL_DML for a detailed explanation of ClickHouse's UPDATE/DELETE
// architecture and why these settings are required.
//
// Like INSERT, ClickHouse's protocol returns 0 for RowsAffected() even when rows
// are successfully updated. This is a protocol limitation, not an error.
//
// # Verification
//
// For each database, the test:
//  1. Creates a test table with driver-specific DDL (including lightweight update
//     settings for ClickHouse).
//  2. Inserts 3 rows: (1, 'Alice'), (2, 'Bob'), (3, 'Charlie').
//  3. Executes UPDATE ... SET name = 'Updated' WHERE id <= 2 (affects 2 rows).
//  4. Asserts RowsAffected() matches the expected value (2 or 0 for ClickHouse).
//  5. Drops the table in t.Cleanup.
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

// TestExecSQL_DDL_DML tests sq's higher-level testh.ExecSQL helper (which wraps
// libsq.ExecSQL) with both DDL (CREATE, DROP) and DML (INSERT, UPDATE, DELETE)
// statements across all supported SQL databases.
//
// # Difference from TestDBExecContext_* Tests
//
// The TestDBExecContext_* tests exercise the raw *sql.DB.ExecContext method from
// Go's standard library to verify driver-level behavior. This test exercises sq's
// abstraction layer (libsq.ExecSQL / testh.ExecSQL), which:
//   - Automatically detects whether SQL is a query (SELECT) or statement (INSERT).
//   - Routes to QueryContext or ExecContext appropriately.
//   - Provides a unified interface for executing arbitrary SQL.
//
// # Test Coverage
//
// For each database (PostgreSQL, SQLite, MySQL, SQL Server, ClickHouse), this
// test executes a complete DDL/DML lifecycle:
//  1. CREATE TABLE - DDL statement, typically returns 0 affected rows.
//  2. INSERT - DML statement, inserts 2 rows.
//  3. UPDATE - DML statement, updates 1 row.
//  4. DELETE - DML statement, deletes 1 row.
//  5. DROP TABLE - DDL statement, typically returns 0 affected rows.
//
// The affected row counts are logged but not strictly asserted (except via the
// helper's internal error checking), since some databases (ClickHouse) return 0
// for all operations.
//
// # ClickHouse Architecture Notes
//
// ClickHouse was designed for append-only analytics workloads, not transactional
// updates. Historically, UPDATE and DELETE were only available as asynchronous
// "mutations" that rewrote entire data parts in the background.
//
// ClickHouse 22.8+ introduced "lightweight deletes" and "lightweight updates"
// which execute synchronously, but require special table settings:
//
//   - enable_block_number_column = 1: Materializes _block_number, identifying
//     which data block contains each row.
//   - enable_block_offset_column = 1: Materializes _block_offset, identifying
//     the row's position within its block.
//
// Together, these columns allow ClickHouse to locate and modify specific rows
// without rewriting entire data parts. Without these settings, UPDATE/DELETE
// fail with: "Lightweight updates are not supported. Lightweight updates are
// supported only for tables with materialized _block_number column."
//
// Even with lightweight updates enabled, ClickHouse returns 0 for affected rows
// (unlike Postgres/SQLite), but the operations execute successfully.
//
// See:
//   - https://clickhouse.com/docs/en/guides/developer/lightweight-update
//   - https://clickhouse.com/docs/en/guides/developer/lightweight-delete
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
