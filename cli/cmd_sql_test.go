package cli_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/userdriver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/testsrc"
)

func TestCmdSQL_Insert_LONG(t *testing.T) {
	testh.SkipShort(t, true)
	testCmdSQL_Insert(t, sakila.All, sakila.All)
}
func TestCmdSQL_Insert(t *testing.T) {
	testCmdSQL_Insert(t, sakila.SQLLatest, sakila.SQLLatest)
}

// testCmdSQL_Insert tests "sq sql QUERY --insert=dest.tbl".
func testCmdSQL_Insert(t *testing.T, origins, dests []string) {
	for _, origin := range origins {
		origin := origin

		t.Run("origin_"+origin, func(t *testing.T) {
			testh.SkipShort(t, origin == sakila.XLSX)

			for _, dest := range dests {
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
					actualDestTbl := th.CopyTable(false, destSrc, sakila.TblActor, "", false)
					t.Cleanup(func() { th.DropTable(destSrc, actualDestTbl) })

					ru := newRun(t).add(*originSrc)
					if destSrc.Handle != originSrc.Handle {
						ru.add(*destSrc)
					}

					insertTo := fmt.Sprintf("%s.%s", destSrc.Handle, actualDestTbl)
					query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(sakila.TblActorCols, ", "), originTbl)

					err := ru.exec("sql", "--insert="+insertTo, query)
					require.NoError(t, err)

					sink, err := th.QuerySQL(destSrc, "select * from "+actualDestTbl)
					require.NoError(t, err)
					require.Equal(t, sakila.TblActorCount, len(sink.Recs))
				})
			}
		})
	}
}

func TestCmdSQL_SelectFromUserDriver(t *testing.T) {
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

				ru := newRun(t).add(*src)

				udDefs := testh.DriverDefsFrom(t, testsrc.PathDriverDefPpl, testsrc.PathDriverDefRSS)
				for _, udDef := range udDefs {
					require.Empty(t, userdriver.ValidateDriverDef(udDef))
				}
				ru.rc.Config.Ext.UserDrivers = append(ru.rc.Config.Ext.UserDrivers, udDefs...)

				err := ru.exec("sql", "--csv", "--no-header", "SELECT * FROM "+wantTbl.tblName)
				require.NoError(t, err)
				recs := ru.mustReadCSV()
				require.Equal(t, wantTbl.wantRows, len(recs), "expected %d rows in tbl %q but got %s", wantTbl.wantRows, wantTbl, len(recs))
				require.Equal(t, wantTbl.wantCols, len(recs[0]), "expected %d cols in tbl %q but got %s", wantTbl.wantCols, wantTbl, len(recs[0]))
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
		wantCount int
		wantErr   bool
	}{
		{fpath: proj.Abs(sakila.PathCSVActorNoHeader), tbl: source.MonotableName, wantCount: sakila.TblActorCount},
		{fpath: proj.Abs(sakila.PathXLSXSubset), tbl: sakila.TblActor, wantCount: sakila.TblActorCount + 1}, // +1 is for the header row in the XLSX file
		{fpath: proj.Abs("README.md"), wantErr: true},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(testh.TName(tc.fpath), func(t *testing.T) {
			t.Parallel()

			f, err := os.Open(tc.fpath)
			require.NoError(t, err)

			ru := newRun(t).hush()
			//ru := newRun(t)
			ru.rc.Stdin = f

			err = ru.exec("sql", "--no-header", "SELECT * FROM "+tc.tbl)
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
