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
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/testsrc"
	"github.com/neilotoole/sq/testh/tu"
)

func TestCmdSQL_ExecType(t *testing.T) {
	tblName

	tSQLCmds := []struct {
		// name is the name of the test.
		name string
		// sql is the SQL command text we are interpreting; it could be a query OR a
		// statement.
		sql string
		// isQuery is true if this is a SQL query; if false, it's a SQL statement.
		isQuery bool
		// wantQueryVals is the set of values expected to be returned if isQuery is
		// true.
		wantQueryVals []any
		// wantRowsAffected is the rows-affected count expected to be returned if
		// isQuery is false.
		wantRowsAffected int64
	}{
		{
			name: "create_table",
			sql:  "CREATE TABLE test_exec_type (id INTEGER, name VARCHAR(100))",
		},
		{
			name:    "select_empty",
			sql:     "SELECT * FROM test_exec_type",
			isQuery: true,
		},
		{
			name: "insert_alice",
			sql:  "INSERT INTO test_exec_type (id, name) VALUES (1, 'Alice')",
		},
		{
			name: "insert_bob",
			sql:  "INSERT INTO test_exec_type (id, name) VALUES (2, 'Bob')",
		},
		{
			name:    "select_two_rows",
			sql:     "SELECT * FROM test_exec_type",
			isQuery: true,
		},
		{
			name: "update_alice",
			sql:  "UPDATE test_exec_type SET name = 'Charlie' WHERE id = 1",
		},
		{
			name:    "select_one_row",
			sql:     "SELECT * FROM test_exec_type WHERE id = 1",
			isQuery: true,
		},
		{
			name: "delete_bob",
			sql:  "DELETE FROM test_exec_type WHERE id = 2",
		},
		{
			name:    "select_after_delete",
			sql:     "SELECT * FROM test_exec_type",
			isQuery: true,
		},
		{
			name: "drop_table",
			sql:  "DROP TABLE test_exec_type",
		},
	}

	for _, handle := range sakila.SQLLatest() {
		t.Run(handle, func(t *testing.T) {
			ctx := t.Context()

			th := testh.New(t)
			src := th.Source(handle)
			tr := testrun.New(ctx, t, nil)
			tr.Add(*src)

			// After this, we'll execute "sq sql X", where X is the SQL cmd string,.
		})
	}

}

// TestCmdSQL_Insert tests "sq sql QUERY --insert=dest.tbl".
func TestCmdSQL_Insert(t *testing.T) {
	for _, origin := range sakila.SQLLatest() {
		origin := origin

		t.Run("origin_"+origin, func(t *testing.T) {
			tu.SkipShort(t, origin == sakila.XLSX)

			for _, dest := range sakila.SQLLatest() {
				dest := dest

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
		tc := tc
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
