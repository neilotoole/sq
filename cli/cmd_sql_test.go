package cli_test

import (
	"context"
	"fmt"
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

						for i, wantVal := range tc.wantQueryVals {
							gotVal, ok := results[i]["name"].(string)
							require.True(t, ok, "expected 'name' column to be string")
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

			// Cleanup: Drop test table
			tr.Reset()
			err = tr.Exec("sql", "DROP TABLE test_edge_cases")
			require.NoError(t, err, "failed to drop test table")
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
