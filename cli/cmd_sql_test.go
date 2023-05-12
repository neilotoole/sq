package cli_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/testh/tutil"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/userdriver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/testsrc"
)

// TestCmdSQL_Insert tests "sq sql QUERY --insert=dest.tbl".
func TestCmdSQL_Insert(t *testing.T) {
	for _, origin := range sakila.SQLLatest() {
		origin := origin

		t.Run("origin_"+origin, func(t *testing.T) {
			tutil.SkipShort(t, origin == sakila.XLSX)

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
					tblName := th.CopyTable(true, destSrc, sakila.TblActor, "", false)

					ru := NewTestRun(th.Context, t, nil).add(*originSrc)
					if destSrc.Handle != originSrc.Handle {
						ru.add(*destSrc)
					}

					insertTo := fmt.Sprintf("%s.%s", destSrc.Handle, tblName)
					query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(sakila.TblActorCols(), ", "), originTbl)

					err := ru.Exec("sql", "--insert="+insertTo, query)
					require.NoError(t, err)

					sink, err := th.QuerySQL(destSrc, "select * from "+tblName)
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

				ru := NewTestRun(th.Context, t, nil).add(*src)
				udDefs := testh.DriverDefsFrom(t, testsrc.PathDriverDefPpl, testsrc.PathDriverDefRSS)
				require.Len(t, udDefs, 2)
				for _, udDef := range udDefs {
					require.Empty(t, userdriver.ValidateDriverDef(udDef))
				}
				ru.rc.Config.Ext.UserDrivers = append(ru.rc.Config.Ext.UserDrivers, udDefs...)

				err := ru.Exec("sql", "--csv", "--header=false", "SELECT * FROM "+wantTbl.tblName)
				require.NoError(t, err)
				recs := ru.mustReadCSV()
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

	testCases := []struct {
		fpath     string
		tbl       string
		flags     map[string]string
		wantCount int
		wantErr   bool
	}{
		{
			fpath:     proj.Abs(sakila.PathCSVActorNoHeader),
			flags:     map[string]string{flag.IngestHeader: "false"},
			tbl:       source.MonotableName,
			wantCount: sakila.TblActorCount,
		},
		{
			fpath:     proj.Abs(sakila.PathCSVActor),
			flags:     map[string]string{flag.IngestHeader: "true"},
			tbl:       source.MonotableName,
			wantCount: sakila.TblActorCount,
		},
		{
			fpath:     proj.Abs(sakila.PathXLSXActorHeader),
			flags:     map[string]string{flag.IngestHeader: "true"},
			tbl:       sakila.TblActor,
			wantCount: sakila.TblActorCount,
		},
		{
			fpath:     proj.Abs(sakila.PathXLSXSubset),
			flags:     map[string]string{flag.IngestHeader: "true"},
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
		name := tutil.Name(i, filepath.Base(filepath.Dir(tc.fpath)), filepath.Base(tc.fpath))
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f, err := os.Open(tc.fpath)
			require.NoError(t, err)

			ru := NewTestRun(context.Background(), t, nil).hush()
			ru.rc.Stdin = f

			args := []string{"sql", "--header=false"} // Don't print the header in output
			for k, v := range tc.flags {
				args = append(args, fmt.Sprintf("--%s=%s", k, v))
			}
			args = append(args, "SELECT * FROM "+tc.tbl)

			err = ru.Exec(args...)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			results := ru.mustReadCSV()
			require.Equal(t, tc.wantCount, len(results))
		})
	}
}
