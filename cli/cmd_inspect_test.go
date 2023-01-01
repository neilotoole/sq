package cli_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/csv"
	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/drivers/xlsx"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestCmdInspect(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		handle   string
		wantErr  bool
		wantType source.Type
		wantTbls []string
	}{
		{
			handle:   sakila.CSVActor,
			wantType: csv.TypeCSV,
			wantTbls: []string{source.MonotableName},
		},
		{
			handle:   sakila.TSVActor,
			wantType: csv.TypeTSV,
			wantTbls: []string{source.MonotableName},
		},
		{
			handle:   sakila.XLSX,
			wantType: xlsx.Type,
			wantTbls: sakila.AllTbls(),
		},
		{
			handle:   sakila.SL3,
			wantType: sqlite3.Type,
			wantTbls: sakila.AllTblsViews(),
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(tc.handle)

			ru := newRun(t).add(*src)

			err := ru.Exec("inspect", "--json")
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			md := &source.Metadata{}
			require.NoError(t, json.Unmarshal(ru.out.Bytes(), md))
			require.Equal(t, tc.wantType, md.SourceType)
			require.Equal(t, src.Handle, md.Handle)
			require.Equal(t, src.Location, md.Location)
			require.Equal(t, tc.wantTbls, md.TableNames())
		})
	}
}

func TestCmdInspectSmoke(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	ru := newRun(t)
	err := ru.Exec("inspect")
	require.Error(t, err, "should fail because no active src")

	ru = newRun(t)
	ru.add(*src) // now have an active src

	err = ru.Exec("inspect", "--json")
	require.NoError(t, err, "should pass because there is an active src")

	md := &source.Metadata{}
	require.NoError(t, json.Unmarshal(ru.out.Bytes(), md))
	require.Equal(t, sqlite3.Type, md.SourceType)
	require.Equal(t, sakila.SL3, md.Handle)
	require.Equal(t, src.RedactedLocation(), md.Location)
	require.Equal(t, sakila.AllTblsViews(), md.TableNames())

	// Try one more source for good measure
	ru = newRun(t)
	src = th.Source(sakila.CSVActor)
	ru.add(*src)

	err = ru.Exec("inspect", "--json", src.Handle)
	require.NoError(t, err)

	md = &source.Metadata{}
	require.NoError(t, json.Unmarshal(ru.out.Bytes(), md))
	require.Equal(t, csv.TypeCSV, md.SourceType)
	require.Equal(t, sakila.CSVActor, md.Handle)
	require.Equal(t, src.Location, md.Location)
	require.Equal(t, []string{source.MonotableName}, md.TableNames())
}

func TestCmdInspect_Stdin(t *testing.T) {
	testCases := []struct {
		fpath    string
		wantErr  bool
		wantType source.Type
		wantTbls []string
	}{
		{fpath: proj.Abs(sakila.PathCSVActor), wantType: csv.TypeCSV, wantTbls: []string{source.MonotableName}},
		{fpath: proj.Abs(sakila.PathTSVActor), wantType: csv.TypeTSV, wantTbls: []string{source.MonotableName}},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tutil.Name(tc.fpath), func(t *testing.T) {
			f, err := os.Open(tc.fpath) // No need to close f
			require.NoError(t, err)

			ru := newRun(t)
			ru.rc.Stdin = f

			err = ru.Exec("inspect", "--json")
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err, "should read from stdin")

			md := &source.Metadata{}
			require.NoError(t, json.Unmarshal(ru.out.Bytes(), md))
			require.Equal(t, tc.wantType, md.SourceType)
			require.Equal(t, source.StdinHandle, md.Handle)
			require.Equal(t, source.StdinHandle, md.Location)
			require.Equal(t, tc.wantTbls, md.TableNames())
		})
	}
}
