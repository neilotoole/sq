package cli_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/drivers/userdriver"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/driver/dialect"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/testsrc"
	"github.com/neilotoole/sq/testh/tu"
)

// TestCmdSQL_ExecMode runs a sequence of SQL CRUD commands (CREATE, SELECT,
// INSERT, UPDATE, DELETE, DROP) against "sq sql" to verify that sq correctly
// distinguishes between queries (which return result sets) and statements
// (which return rows-affected counts).
//
// The test performs a complete CRUD lifecycle:
//  1. CREATE TABLE - creates a test table (statement, rows_affected=0)
//  2. SELECT - verifies table is empty (query, returns empty result set)
//  3. INSERT x2 - inserts two rows (statements, rows_affected=1 each)
//  4. SELECT - verifies both rows exist (query, returns 2 rows)
//  5. UPDATE - modifies one row (statement, rows_affected=1)
//  6. SELECT - verifies the update (query, returns modified value)
//  7. DELETE - removes one row (statement, rows_affected=1)
//  8. SELECT - verifies deletion (query, returns 1 row)
//  9. DROP TABLE - cleans up (statement, rows_affected=0)
//
// This exercises:
//   - SQL statement type detection (query vs statement)
//   - Correct output format for each type (result set vs rows_affected)
//   - Full CRUD operation support across all database drivers
//   - Sequential statement execution maintaining table state
//
// Note: ClickHouse requires special CREATE TABLE DDL (ENGINE, ORDER BY,
// lightweight mutation settings) and always returns 0 for rows affected
// on DML operations. Per-handle driver configs accommodate these differences.
func TestCmdSQL_ExecMode(t *testing.T) {
	t.Parallel()

	// driverCfg holds per-handle configuration for driver-specific behavior.
	type driverCfg struct {
		// createSQL is the CREATE TABLE DDL for this driver.
		createSQL string
		// wantDMLRowsAffected is the expected rows_affected for INSERT/UPDATE/DELETE.
		// Most drivers return 1; ClickHouse returns -1 (unsupported).
		wantDMLRowsAffected int64
		// wantDDLRowsAffected is the expected rows_affected for DDL statements
		// (CREATE TABLE, DROP TABLE). Most drivers return 0; ClickHouse returns
		// -1 because the CLI converts 0 to RowsAffectedUnsupported when the
		// dialect has IsRowsAffectedUnsupported set.
		wantDDLRowsAffected int64
	}

	defaultCfg := driverCfg{
		createSQL:           "CREATE TABLE test_exec_type (id INTEGER, name VARCHAR(100))",
		wantDMLRowsAffected: 1,
		wantDDLRowsAffected: 0,
	}

	// driverCfgs maps handles to their driver-specific config. Handles not
	// present in this map use defaultCfg.
	driverCfgs := map[string]driverCfg{
		sakila.CH: {
			// ClickHouse requires MergeTree engine and lightweight mutation settings
			// for standard UPDATE/DELETE support. See libsq.TestExecSQL_DDL_DML.
			createSQL: "CREATE TABLE test_exec_type (id Int32, name String)" +
				" ENGINE = MergeTree() ORDER BY id" +
				" SETTINGS enable_block_number_column = 1, enable_block_offset_column = 1",
			// ClickHouse does not report rows affected for any exec statement.
			// The CLI intercepts the raw 0 from the protocol and converts it
			// to -1 (RowsAffectedUnsupported) for both DML and DDL.
			wantDMLRowsAffected: dialect.RowsAffectedUnsupported,
			wantDDLRowsAffected: dialect.RowsAffectedUnsupported,
		},
	}

	testCases := []struct {
		// name is the name of the test.
		name string
		// sql is the SQL command text we are interpreting; it could be a query OR a
		// statement.
		sql string
		// isQuery is true if this is a SQL query; if false, it's a SQL statement.
		isQuery bool
		// wantQueryVals is the set of "name" column values expected to be returned
		// if isQuery is true. We use a single column to simplify verification.
		wantQueryVals []string
		// isDML is true for INSERT/UPDATE/DELETE statements where wantDMLRowsAffected
		// from driverCfg should be used. When false (DDL like CREATE/DROP),
		// wantDDLRowsAffected from driverCfg is used instead.
		isDML bool
	}{
		{
			name: "create_table",
		},
		{
			name:          "select_empty",
			sql:           "SELECT name FROM test_exec_type",
			isQuery:       true,
			wantQueryVals: []string{},
		},
		{
			name:  "insert_alice",
			sql:   "INSERT INTO test_exec_type (id, name) VALUES (1, 'Alice')",
			isDML: true,
		},
		{
			name:  "insert_bob",
			sql:   "INSERT INTO test_exec_type (id, name) VALUES (2, 'Bob')",
			isDML: true,
		},
		{
			name:          "select_two_rows",
			sql:           "SELECT name FROM test_exec_type ORDER BY id",
			isQuery:       true,
			wantQueryVals: []string{"Alice", "Bob"},
		},
		{
			name:  "update_alice",
			sql:   "UPDATE test_exec_type SET name = 'Charlie' WHERE id = 1",
			isDML: true,
		},
		{
			name:          "select_one_row",
			sql:           "SELECT name FROM test_exec_type WHERE id = 1",
			isQuery:       true,
			wantQueryVals: []string{"Charlie"},
		},
		{
			name:  "delete_bob",
			sql:   "DELETE FROM test_exec_type WHERE id = 2",
			isDML: true,
		},
		{
			name:          "select_after_delete",
			sql:           "SELECT name FROM test_exec_type",
			isQuery:       true,
			wantQueryVals: []string{"Charlie"},
		},
		{
			name: "drop_table",
			sql:  "DROP TABLE test_exec_type",
		},
	}

	// Execute the test cases for each of the available SQL sources.
	for _, handle := range sakila.SQLLatest() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle) // Will skip test if source not available

			// Belt-and-suspenders cleanup: registered before the create_table
			// test case runs, so the table is dropped even if a subsequent
			// test case fails before the drop_table case can execute.
			// SQLDriver.DropTable uses ifExists=true, so this is a no-op
			// when drop_table runs successfully.
			t.Cleanup(func() { th.DropTable(src, tablefq.From("test_exec_type")) })

			cfg, ok := driverCfgs[handle]
			if !ok {
				cfg = defaultCfg
			}

			tr := testrun.New(th.Context, t, nil)

			// Set format to JSON so that subsequent runs are using JSON.
			require.NoError(t, tr.Exec("config", "set", "format", "json"))

			tr.Reset().Add(*src)
			require.NoError(t, tr.Exec("ping"), "source %s should be available", handle)

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					tr.Reset()

					sql := tc.sql
					if tc.name == "create_table" {
						sql = cfg.createSQL
					}

					err := tr.Exec("sql", sql)
					require.NoError(t, err, "failed to execute: %s", sql)

					if tc.isQuery {
						// For queries, verify we get the expected values.
						var results []map[string]any
						tr.Bind(&results)

						require.Len(t, results, len(tc.wantQueryVals),
							"expected %d rows, got %d", len(tc.wantQueryVals), len(results))

						// Oracle stores unquoted identifiers in upper case, so
						// the JSON key for the projected column is "NAME".
						nameCol := "name"
						if src.Type == drivertype.Oracle {
							nameCol = "NAME"
						}

						for i, wantVal := range tc.wantQueryVals {
							gotVal, ok := results[i][nameCol].(string)
							require.True(t, ok, "expected %q column to be string", nameCol)
							require.Equal(t, wantVal, gotVal,
								"row %d: expected name=%q, got %q", i, wantVal, gotVal)
						}
					} else {
						// For statements, verify rows_affected.
						var wantAffected int64
						if tc.isDML {
							wantAffected = cfg.wantDMLRowsAffected
						} else {
							wantAffected = cfg.wantDDLRowsAffected
						}

						result := tr.BindMap()
						gotAffected, ok := result["rows_affected"].(float64)
						require.True(t, ok, "expected 'rows_affected' in output")
						require.Equal(t, wantAffected, int64(gotAffected),
							"expected rows_affected=%d, got %d", wantAffected, int64(gotAffected))
					}
				})
			}
		})
	}
}

// TestCmdSQL_MultipleStatements tests the behavior of "sq sql" when the SQL
// string contains multiple statements.
//
// This test is skipped because the behavior varies significantly between
// database drivers, and sq doesn't currently have a strategy for handling
// these situations consistently.
//
// # Observed Driver Behavior (as of 2026-01)
//
// When executing "SELECT * FROM t; SELECT * FROM t":
//
//   - PostgreSQL: ERROR - "cannot insert multiple commands into a prepared statement"
//   - MySQL:      ERROR - driver rejects multiple statements
//   - SQLite:     NO ERROR - silently executes only the first statement
//   - SQL Server: NO ERROR - silently executes only the first statement
//
// When executing "SELECT * FROM t; INSERT INTO t ...":
//
//   - PostgreSQL: ERROR
//   - MySQL:      ERROR
//   - SQLite:     NO ERROR - silently executes only the first statement (SELECT)
//   - SQL Server: ERROR
//
// When executing "INSERT ...; INSERT ...":
//
//   - All drivers: ERROR
//
// # Why This Matters
//
// The inconsistent behavior is problematic because:
//  1. Users may not realize only the first statement executed
//  2. Silent partial execution can lead to data inconsistencies
//  3. The behavior depends on which database you're connected to
//
// # Future Considerations
//
// In a future revision, sq could parse out each statement and execute them
// sequentially. However, this raises questions about output format - how would
// sq return multiple result sets (e.g., multiple JSON arrays), or a combination
// of result sets and "rows affected" output? This is not yet clear.
//
// At the very least, this test documents the current inconsistent behavior.
func TestCmdSQL_MultipleStatements(t *testing.T) {
	t.Skipf("Skipping: multiple-statement behavior varies by driver (see doc comment)")

	t.Parallel()

	// Example error from PostgreSQL:
	//
	//   $ sq sql "select * from actor; select * from actor"
	//   sq: SQL query against @sakila/local/pg failed: select * from actor; select * from actor:
	//       ERROR: cannot insert multiple commands into a prepared statement (SQLSTATE 42601)

	testCases := []struct {
		name string
		sql  string
	}{
		{
			name: "two_selects",
			sql:  "SELECT * FROM actor; SELECT * FROM actor",
		},
		{
			name: "select_and_insert",
			sql:  "SELECT * FROM actor; INSERT INTO actor (actor_id, first_name, last_name) VALUES (9999, 'Test', 'User')",
		},
		{
			name: "two_inserts",
			//nolint:lll
			sql: "INSERT INTO actor (actor_id, first_name, last_name) VALUES (9998, 'A', 'B'); INSERT INTO actor (actor_id, first_name, last_name) VALUES (9999, 'C', 'D')",
		},
	}

	for _, handle := range sakila.SQLLatest() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			tr := testrun.New(th.Context, t, nil).Hush()

			tr.Add(*src)
			require.NoError(t, tr.Exec("ping"), "source %s should be available", handle)

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					tr.Reset()

					err := tr.Exec("sql", tc.sql)
					require.Error(t, err, "expected error for multiple statements: %s", tc.sql)
				})
			}
		})
	}
}

// TestCmdSQL_ExecTypeEdgeCases tests edge cases for SQL statement type detection
// to ensure sq correctly identifies queries vs statements regardless of formatting.
//
// The test verifies that sq handles:
//   - Case variations: SELECT, select, SeLeCt all detected as queries
//   - Lowercase statements: insert, update, delete detected as statements
//   - Block comments: /* comment */ SELECT still detected as query
//   - Common Table Expressions: WITH cte AS (...) SELECT detected as query
//
// For each case, the test verifies:
//   - Queries return a result set (JSON array)
//   - Statements return rows_affected count
//
// This is important because sq needs to determine the output format based on
// whether the SQL is a query (returns data) or statement (returns affected count),
// and this detection must work regardless of SQL formatting conventions.
//
// Note: ClickHouse requires special CREATE TABLE DDL (ENGINE, ORDER BY,
// lightweight mutation settings) and always returns 0 for rows affected
// on DML operations. Per-handle driver configs accommodate these differences.
func TestCmdSQL_ExecTypeEdgeCases(t *testing.T) {
	t.Parallel()

	// driverCfg holds per-handle configuration for driver-specific behavior.
	type driverCfg struct {
		// createSQL is the CREATE TABLE DDL for this driver.
		createSQL string
		// wantDMLRowsAffected is the expected rows_affected for INSERT/UPDATE/DELETE.
		// Most drivers return 1; ClickHouse returns 0.
		wantDMLRowsAffected int64
	}

	defaultCfg := driverCfg{
		createSQL:           "CREATE TABLE test_edge_cases (id INTEGER, name VARCHAR(100))",
		wantDMLRowsAffected: 1,
	}

	// driverCfgs maps handles to their driver-specific config. Handles not
	// present in this map use defaultCfg.
	driverCfgs := map[string]driverCfg{
		sakila.CH: {
			// ClickHouse requires MergeTree engine and lightweight mutation settings
			// for standard UPDATE/DELETE support. See libsq.TestExecSQL_DDL_DML.
			createSQL: "CREATE TABLE test_edge_cases (id Int32, name String)" +
				" ENGINE = MergeTree() ORDER BY id" +
				" SETTINGS enable_block_number_column = 1, enable_block_offset_column = 1",
			// ClickHouse does not report rows affected for DML. The CLI
			// intercepts the raw 0 from the protocol and converts it to -1.
			wantDMLRowsAffected: dialect.RowsAffectedUnsupported,
		},
	}

	testCases := []struct {
		name    string
		sql     string
		isQuery bool
		// isDML is true for INSERT/UPDATE/DELETE statements where wantDMLRowsAffected
		// from driverCfg should be used instead of a fixed value.
		isDML bool
	}{
		// Lowercase variations
		{
			name:    "select_lowercase",
			sql:     "select name from test_edge_cases where id = 1",
			isQuery: true,
		},
		{
			name:  "insert_lowercase",
			sql:   "insert into test_edge_cases (id, name) values (10, 'Lowercase')",
			isDML: true,
		},
		{
			name:  "update_lowercase",
			sql:   "update test_edge_cases set name = 'Updated' where id = 10",
			isDML: true,
		},
		{
			name:  "delete_lowercase",
			sql:   "delete from test_edge_cases where id = 10",
			isDML: true,
		},

		// Mixed case
		{
			name:    "select_mixed_case",
			sql:     "SeLeCt name FROM test_edge_cases WHERE id = 1",
			isQuery: true,
		},

		// NOTE: SQL with leading single-line comments (--) cannot be tested via CLI
		// arguments because the shell interprets -- as a flag prefix. These cases
		// are tested in the unit test TestIsQueryStatement instead.

		// SQL with leading block comments
		{
			name:    "select_with_block_comment",
			sql:     "/* Block comment */ SELECT name FROM test_edge_cases WHERE id = 1",
			isQuery: true,
		},
		{
			name:  "insert_with_block_comment",
			sql:   "/* Insert comment */ INSERT INTO test_edge_cases (id, name) VALUES (30, 'BlockComment')",
			isDML: true,
		},
		{
			name:  "delete_block_cleanup",
			sql:   "DELETE FROM test_edge_cases WHERE id = 30",
			isDML: true,
		},

		// WITH (Common Table Expressions)
		{
			name:    "with_cte",
			sql:     "WITH cte AS (SELECT id, name FROM test_edge_cases) SELECT name FROM cte WHERE id = 1",
			isQuery: true,
		},
		{
			name:    "with_cte_lowercase",
			sql:     "with cte as (select id, name from test_edge_cases) select name from cte where id = 1",
			isQuery: true,
		},
	}

	for _, handle := range sakila.SQLLatest() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)

			// Register cleanup before any DDL runs, so the table is dropped
			// even if an early require.NoError aborts the test. DropTable
			// uses ifExists=true under the hood.
			t.Cleanup(func() { th.DropTable(src, tablefq.From("test_edge_cases")) })

			cfg, ok := driverCfgs[handle]
			if !ok {
				cfg = defaultCfg
			}

			tr := testrun.New(th.Context, t, nil)

			// Set format to JSON
			require.NoError(t, tr.Exec("config", "set", "format", "json"))

			tr.Reset().Add(*src)
			require.NoError(t, tr.Exec("ping"), "source %s should be available", handle)

			// Setup: Create test table and insert initial data
			tr.Reset()
			err := tr.Exec("sql", cfg.createSQL)
			require.NoError(t, err, "failed to create test table")

			tr.Reset()
			err = tr.Exec("sql", "INSERT INTO test_edge_cases (id, name) VALUES (1, 'Alice')")
			require.NoError(t, err, "failed to insert test data")

			// Run edge case tests
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					tr.Reset()

					err := tr.Exec("sql", tc.sql)
					require.NoError(t, err, "failed to execute: %s", tc.sql)

					if tc.isQuery {
						var results []map[string]any
						tr.Bind(&results)
						// Just verify it's treated as a query (returns array)
						require.NotNil(t, results, "expected query results for: %s", tc.sql)
					} else {
						result := tr.BindMap()
						gotAffected, ok := result["rows_affected"].(float64)
						require.True(t, ok, "expected 'rows_affected' in output for: %s", tc.sql)
						require.Equal(t, cfg.wantDMLRowsAffected, int64(gotAffected),
							"expected rows_affected=%d, got %d for: %s",
							cfg.wantDMLRowsAffected, int64(gotAffected), tc.sql)
					}
				})
			}
		})
	}
}

// TestCmdSQL_Insert tests the "sq sql QUERY --insert=@dest.tbl" functionality,
// which executes a SQL query against one source and inserts the results into
// a table in another (or the same) source. This is a key feature for cross-database
// data transfer without requiring intermediate files.
//
// The test performs the following for each origin/destination database combination:
//  1. Creates an empty copy of the actor table in the destination database
//  2. Executes a SELECT query against the origin database's actor table
//  3. Uses --insert to pipe results directly into the destination table
//  4. Verifies all 200 actor rows were successfully transferred
//
// This exercises:
//   - Cross-database querying and insertion (e.g., SQLite → PostgreSQL)
//   - Same-database query-to-insert (e.g., PostgreSQL → PostgreSQL)
//   - The batch insert mechanism for efficiently transferring multiple rows
//   - Schema compatibility between different database engines
//   - The CLI's ability to manage multiple source connections simultaneously
//
// The test matrix covers all combinations of supported SQL databases as both
// origin (data source) and destination (insert target), ensuring the feature
// works regardless of which databases are involved.
func TestCmdSQL_Insert(t *testing.T) {
	for _, origin := range sakila.SQLLatest() {
		t.Run("origin_"+origin, func(t *testing.T) {
			tu.SkipShort(t, origin == sakila.XLSX)

			for _, dest := range sakila.SQLLatest() {
				t.Run("dest_"+dest, func(t *testing.T) {
					t.Parallel()

					th := testh.New(t)
					originSrc, destSrc := th.Source(origin), th.Source(dest)
					originTbl := sakila.TblActor

					if th.IsMonotable(originSrc) {
						originTbl = source.MonotableName
					}

					// To avoid dirtying the destination table, we make a copy
					// of it (without data).
					destTbl := th.CopyTable(true, destSrc, tablefq.From(sakila.TblActor), tablefq.T{}, false)

					tr := testrun.New(th.Context, t, nil).Add(*originSrc)
					if destSrc.Handle != originSrc.Handle {
						tr.Add(*destSrc)
					}

					insertTo := fmt.Sprintf("%s.%s", destSrc.Handle, destTbl)
					query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(sakila.TblActorCols(), ", "), originTbl)

					err := tr.Exec("sql", "--insert="+insertTo, query)
					require.NoError(t, err)

					sink, err := th.QuerySQL(destSrc, nil, "select * from "+destTbl)
					require.NoError(t, err)
					require.Equal(t, sakila.TblActorCount, len(sink.Recs))
				})
			}
		})
	}
}

func TestCmdSQL_SelectFromUserDriver(t *testing.T) {
	t.Parallel()

	testCases := map[string][]struct {
		tblName  string
		wantRows int
		wantCols int
	}{
		testsrc.PplUD: {
			{tblName: "person", wantRows: 3, wantCols: 7},
			{tblName: "skill", wantRows: 6, wantCols: 3},
		},
		testsrc.RSSNYTLocalUD: {
			{tblName: "category", wantRows: 251, wantCols: 4},
			{tblName: "channel", wantRows: 1, wantCols: 7},
			{tblName: "item", wantRows: 45, wantCols: 9},
		},
	}

	for handle, wantTbls := range testCases {
		for _, wantTbl := range wantTbls {
			handle, wantTbl := handle, wantTbl
			t.Run(handle+"__"+wantTbl.tblName, func(t *testing.T) {
				t.Parallel()

				th := testh.New(t)
				src := th.Source(handle)

				tr := testrun.New(th.Context, t, nil).Add(*src)
				udDefs := testh.DriverDefsFrom(t, testsrc.PathDriverDefPpl, testsrc.PathDriverDefRSS)
				require.Len(t, udDefs, 2)
				for _, udDef := range udDefs {
					require.Empty(t, userdriver.ValidateDriverDef(udDef))
				}
				tr.Run.Config.Ext.UserDrivers = append(tr.Run.Config.Ext.UserDrivers, udDefs...)

				err := tr.Exec("sql", "--csv", "--header=false", "SELECT * FROM "+wantTbl.tblName)
				require.NoError(t, err)
				recs := tr.BindCSV()
				require.Equal(t, wantTbl.wantRows, len(recs),
					"expected %d rows in tbl {%s} but got %d", wantTbl.wantRows,
					wantTbl, len(recs))
				require.Equal(t, wantTbl.wantCols, len(recs[0]),
					"expected %d cols in tbl {%s} but got %d",
					wantTbl.wantCols, wantTbl, len(recs[0]))
			})
		}
	}
}

// TestCmdSQL_StdinQuery verifies that cmd sql can read from stdin.
func TestCmdSQL_StdinQuery(t *testing.T) {
	t.Parallel()

	flagIngestHeader := driver.OptIngestHeader.Flag().Name

	testCases := []struct {
		fpath     string
		tbl       string
		flags     map[string]string
		wantCount int
		wantErr   bool
	}{
		{
			fpath:     proj.Abs(sakila.PathCSVActorNoHeader),
			flags:     map[string]string{flagIngestHeader: "false"},
			tbl:       source.MonotableName,
			wantCount: sakila.TblActorCount,
		},
		{
			fpath:     proj.Abs(sakila.PathCSVActor),
			flags:     map[string]string{flagIngestHeader: "true"},
			tbl:       source.MonotableName,
			wantCount: sakila.TblActorCount,
		},
		{
			fpath:     proj.Abs(sakila.PathXLSXActorHeader),
			flags:     map[string]string{flagIngestHeader: "true"},
			tbl:       sakila.TblActor,
			wantCount: sakila.TblActorCount,
		},
		{
			fpath:     proj.Abs(sakila.PathXLSXSubset),
			flags:     map[string]string{flagIngestHeader: "true"},
			tbl:       sakila.TblActor,
			wantCount: sakila.TblActorCount,
		},
		{
			// Parquet can't be read as a stream: the source is materialized
			// to a temp file (like sqlite/duckdb on stdin), then read by
			// DuckDB's read_parquet.
			fpath:     proj.Abs("drivers/parquet/testdata/actor.parquet"),
			tbl:       source.MonotableName,
			wantCount: 3,
		},
		{
			fpath:   proj.Abs("README.md"),
			wantErr: true,
		},
	}

	for i, tc := range testCases {
		name := tu.Name(i, filepath.Base(filepath.Dir(tc.fpath)), filepath.Base(tc.fpath))
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f, err := os.Open(tc.fpath)
			require.NoError(t, err)

			tr := testrun.New(context.Background(), t, nil).Hush()
			tr.Run.Stdin = f

			args := []string{"sql", "--header=false"} // Don't print the header in output
			for k, v := range tc.flags {
				args = append(args, fmt.Sprintf("--%s=%s", k, v))
			}
			args = append(args, "SELECT * FROM "+tc.tbl)

			err = tr.Exec(args...)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			results := tr.BindCSV()
			require.Equal(t, tc.wantCount, len(results))
		})
	}
}

func TestFlagActiveSource_sql(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tr := testrun.New(ctx, t, nil)

	// @sqlite will be the active source
	require.NoError(t, tr.Exec("add", proj.Abs(sakila.PathSL3), "--handle", "@sqlite"))

	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("add", proj.Abs(sakila.PathCSVActor), "--handle", "@csv"))

	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec(
		"sql",
		"--csv",
		"--no-header",
		`select * from actor`,
	))
	require.Len(t, tr.BindCSV(), sakila.TblActorCount)

	// Now, use flag.ActiveSrc to switch the source.
	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec(
		"sql",
		"--csv",
		"--no-header",
		"--src", "@csv",
		"select * from data",
	))
	require.Len(t, tr.BindCSV(), sakila.TblActorCount)

	// Double check that we didn't change the persisted active source
	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("src", "--json"))
	require.Equal(t, "@sqlite", tr.BindMap()["handle"])
}

// TestSQL_ReadOnlyFlag_HappyPath verifies that --readonly succeeds against
// a DuckDB source for a SELECT statement.
func TestSQL_ReadOnlyFlag_HappyPath(t *testing.T) {
	t.Parallel()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil).Add(*th.Source(sakila.Duck))
	err := tr.Exec("sql", "--readonly", "SELECT count(*) AS n FROM actor")
	require.NoError(t, err)
	require.Contains(t, tr.Out.String(), "200")
}

// TestSQL_ROAlias_HappyPath verifies the --ro alias behaves identically.
func TestSQL_ROAlias_HappyPath(t *testing.T) {
	t.Parallel()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil).Add(*th.Source(sakila.Duck))
	err := tr.Exec("sql", "--ro", "SELECT count(*) AS n FROM actor")
	require.NoError(t, err)
	require.Contains(t, tr.Out.String(), "200")
}

// TestSQL_ReadOnlyAndAlias_BothSet verifies that specifying both flags
// is idempotent (no error).
func TestSQL_ReadOnlyAndAlias_BothSet(t *testing.T) {
	t.Parallel()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil).Add(*th.Source(sakila.Duck))
	err := tr.Exec("sql", "--readonly", "--ro", "SELECT count(*) AS n FROM actor")
	require.NoError(t, err)
}

// TestSQL_ReadOnly_ConflictWithURL verifies that --readonly + a URL
// that explicitly says access_mode=READ_WRITE returns a conflict error
// (not a silent override).
func TestSQL_ReadOnly_ConflictWithURL(t *testing.T) {
	t.Parallel()
	th := testh.New(t)
	src := th.Source(sakila.Duck).Clone()
	src.Handle = "@sakila_duck_rw"
	src.Location += "?access_mode=READ_WRITE"

	tr := testrun.New(th.Context, t, nil).Add(*src)
	err := tr.Exec("sql", "--src", src.Handle, "--readonly", "SELECT 1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--readonly")
	require.Contains(t, err.Error(), "READ_WRITE")
	require.Contains(t, err.Error(), src.Handle)
}

// TestSQL_ROAlias_ConflictWithURL verifies the alias produces the same
// conflict error (error message names the canonical flag).
func TestSQL_ROAlias_ConflictWithURL(t *testing.T) {
	t.Parallel()
	th := testh.New(t)
	src := th.Source(sakila.Duck).Clone()
	src.Handle = "@sakila_duck_rw2"
	src.Location += "?access_mode=READ_WRITE"

	tr := testrun.New(th.Context, t, nil).Add(*src)
	err := tr.Exec("sql", "--src", src.Handle, "--ro", "SELECT 1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--readonly",
		"error message should name the canonical flag")
}

// TestSQL_ReadOnlyWithInsert_DestStaysRW verifies that --readonly applies
// only to the source side of an --insert operation: the destination must
// stay READ_WRITE so the INSERT itself succeeds. Mirrors the slq --insert
// pattern fixed by gh610.
func TestSQL_ReadOnlyWithInsert_DestStaysRW(t *testing.T) {
	t.Parallel()
	th := testh.New(t)

	// Source: shared duckdb fixture (RO is fine).
	src := th.Source(sakila.Duck)

	// Destination: temp-copy duckdb so we can write into it without
	// touching the shared fixture.
	srcPath := proj.Abs("drivers/duckdb/testdata/sakila.duckdb")
	dstPath := filepath.Join(t.TempDir(), "dest.duckdb")
	in, err := os.Open(srcPath)
	require.NoError(t, err)
	defer in.Close()
	out, err := os.Create(dstPath)
	require.NoError(t, err)
	_, err = io.Copy(out, in)
	require.NoError(t, err)
	require.NoError(t, out.Close())

	dest := &source.Source{
		Handle:   "@sakila_duck_dest",
		Type:     drivertype.DuckDB,
		Location: "duckdb://" + dstPath,
	}

	destTbl := "ro_insert_dest_" + t.Name()
	// DuckDB-safe table name (no slashes, etc.).
	destTbl = strings.ReplaceAll(destTbl, "/", "_")

	tr := testrun.New(th.Context, t, nil).Add(*src).Add(*dest)
	err = tr.Exec("sql", "--src", src.Handle, "--readonly",
		"--insert", dest.Handle+"."+destTbl,
		"SELECT first_name, last_name FROM actor LIMIT 3")
	require.NoError(t, err,
		"--readonly should apply to the source only; INSERT into dest must succeed")
}

// TestSQL_ReadOnly_RejectsWrites verifies a write statement under --readonly
// surfaces DuckDB's native read-only error cleanly. Uses a temp copy of the
// shared fixture so the write attempt can't accidentally mutate it.
func TestSQL_ReadOnly_RejectsWrites(t *testing.T) {
	t.Parallel()
	th := testh.New(t)

	srcPath := proj.Abs("drivers/duckdb/testdata/sakila.duckdb")
	dstPath := filepath.Join(t.TempDir(), "sakila.duckdb")
	in, err := os.Open(srcPath)
	require.NoError(t, err)
	defer in.Close()
	out, err := os.Create(dstPath)
	require.NoError(t, err)
	_, err = io.Copy(out, in)
	require.NoError(t, err)
	require.NoError(t, out.Close())

	src := &source.Source{
		Handle:   "@sakila_duck_ro_reject",
		Type:     drivertype.DuckDB,
		Location: "duckdb://" + dstPath,
	}

	tr := testrun.New(th.Context, t, nil).Add(*src)
	err = tr.Exec("sql", "--src", src.Handle, "--readonly",
		"INSERT INTO actor (first_name, last_name) VALUES ('X', 'Y')")
	require.Error(t, err)
	msg := err.Error()
	require.True(t,
		strings.Contains(msg, "read-only") || strings.Contains(msg, "Cannot execute"),
		"unexpected error: %s", msg)
}

// TestSQL_ReadOnly_SrcSchema_DoesNotModifyMtime mirrors the SLQ regression
// test: --src.schema triggers verifySourceCatalogSchema, which pre-opens
// the source via Grips.Open. Without hoisting the RO ctx flip ahead of
// determineSources, that pre-open caches a RW grip that subsequent RO
// opens silently reuse, defeating the --readonly intent.
func TestSQL_ReadOnly_SrcSchema_DoesNotModifyMtime(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Duck)
	path := strings.TrimPrefix(src.Location, "duckdb://")

	statBefore, err := os.Stat(path)
	require.NoError(t, err)

	tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
	require.NoError(t, tr.Exec("sql", "--src", src.Handle, "--src.schema=main",
		"--readonly", "SELECT count(*) AS n FROM actor"))

	statAfter, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, statBefore.ModTime(), statAfter.ModTime(),
		"DuckDB file mtime must not change after sq sql --readonly --src.schema")
}

// TestSQL_ReadOnly_Insert_SrcSchema_SourceUntouched verifies the bug
// Copilot caught in round 2: --insert + --src.schema + --readonly was
// pre-opening the source RW (via verifySourceCatalogSchema's grips
// cache), defeating the source-side RO intent. After the fix
// (ephemeral RO open in verifySourceCatalogSchema), the source file
// mtime stays put while the destination is opened RW for the INSERT.
func TestSQL_ReadOnly_Insert_SrcSchema_SourceUntouched(t *testing.T) {
	t.Parallel()
	th := testh.New(t)

	src := th.Source(sakila.Duck)
	srcPath := strings.TrimPrefix(src.Location, "duckdb://")

	// Separate temp-copy DuckDB destination so the INSERT has somewhere
	// to land without touching the shared fixture.
	fixturePath := proj.Abs("drivers/duckdb/testdata/sakila.duckdb")
	dstPath := filepath.Join(t.TempDir(), "dest.duckdb")
	in, err := os.Open(fixturePath)
	require.NoError(t, err)
	defer in.Close()
	out, err := os.Create(dstPath)
	require.NoError(t, err)
	_, err = io.Copy(out, in)
	require.NoError(t, err)
	require.NoError(t, out.Close())
	dest := &source.Source{
		Handle:   "@duck_dest_ro_insert_schema",
		Type:     drivertype.DuckDB,
		Location: "duckdb://" + dstPath,
	}
	destTbl := "ro_insert_schema_" + strings.ReplaceAll(t.Name(), "/", "_")

	srcStatBefore, err := os.Stat(srcPath)
	require.NoError(t, err)

	tr := testrun.New(th.Context, t, nil).Add(*src).Add(*dest)
	require.NoError(t, tr.Exec("sql",
		"--src", src.Handle, "--src.schema=main", "--readonly",
		"--insert", dest.Handle+"."+destTbl,
		"SELECT first_name, last_name FROM actor LIMIT 3"))

	srcStatAfter, err := os.Stat(srcPath)
	require.NoError(t, err)
	require.Equal(t, srcStatBefore.ModTime(), srcStatAfter.ModTime(),
		"DuckDB source file mtime must not change when --readonly + --insert + --src.schema")
}

// TestSQL_ReadOnly_Conflict_Preempted verifies the round-3 Copilot
// finding: the --readonly + URL access_mode=READ_WRITE conflict is
// surfaced BEFORE any file open, including the schema-validation
// pre-open triggered by --src.schema. Without the preflight, the
// file would briefly open RW (URL wins over the RO ctx) before the
// error fired, mutating the source mtime.
func TestSQL_ReadOnly_Conflict_Preempted(t *testing.T) {
	t.Parallel()
	th := testh.New(t)

	// Copy the shared fixture so we can stat its mtime without races
	// against parallel tests using the same shared file.
	fixturePath := proj.Abs("drivers/duckdb/testdata/sakila.duckdb")
	tmpPath := filepath.Join(t.TempDir(), "sakila.duckdb")
	in, err := os.Open(fixturePath)
	require.NoError(t, err)
	defer in.Close()
	out, err := os.Create(tmpPath)
	require.NoError(t, err)
	_, err = io.Copy(out, in)
	require.NoError(t, err)
	require.NoError(t, out.Close())

	src := &source.Source{
		Handle:   "@sakila_duck_conflict_preempt",
		Type:     drivertype.DuckDB,
		Location: "duckdb://" + tmpPath + "?access_mode=READ_WRITE",
	}

	statBefore, err := os.Stat(tmpPath)
	require.NoError(t, err)

	tr := testrun.New(th.Context, t, nil).Add(*src)
	err = tr.Exec("sql", "--src", src.Handle, "--src.schema=main",
		"--readonly", "SELECT 1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--readonly")
	require.Contains(t, err.Error(), "READ_WRITE")

	statAfter, err := os.Stat(tmpPath)
	require.NoError(t, err)
	require.Equal(t, statBefore.ModTime(), statAfter.ModTime(),
		"mtime must not change: conflict must be preempted before any open")
}
