package cli_test

import (
	"encoding/json"
	"os"
	"testing"

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
	th := testh.New(t)
	src := th.Source(sakila.SL3)
	ru := newRun(t)

	err := ru.exec("inspect")
	require.Error(t, err, "should fail because no active src")

	ru = newRun(t)
	ru.add(*src) // now have an active src

	err = ru.exec("inspect", "--json")
	require.NoError(t, err, "should pass because there is an active src")

	md := &source.Metadata{}
	require.NoError(t, json.Unmarshal(ru.out.Bytes(), md))
	require.Equal(t, sqlite3.Type, md.SourceType)
	require.Equal(t, sakila.SL3, md.Handle)
	require.Equal(t, src.Location, md.Location)
	require.Equal(t, sakila.AllTbls, md.TableNames())

	// Try one more source for good measure
	ru = newRun(t)
	src = th.Source(sakila.CSVActor)
	ru.add(*src)

	err = ru.exec("inspect", "--json", src.Handle)
	require.NoError(t, err)

	md = &source.Metadata{}
	require.NoError(t, json.Unmarshal(ru.out.Bytes(), md))
	require.Equal(t, csv.TypeCSV, md.SourceType)
	require.Equal(t, sakila.CSVActor, md.Handle)
	require.Equal(t, src.Location, md.Location)
	require.Equal(t, []string{source.MonotableName}, md.TableNames())
}

func TestCmdInspect_Stdin(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		fpath    string
		wantErr  bool
		wantType source.Type
		wantTbls []string
	}{
		{fpath: proj.Abs(sakila.PathCSVActor), wantType: csv.TypeCSV, wantTbls: []string{source.MonotableName}},
		{fpath: proj.Abs(sakila.PathTSVActor), wantType: csv.TypeTSV, wantTbls: []string{source.MonotableName}},
		{fpath: proj.Abs(sakila.PathXLSX), wantType: xlsx.Type, wantTbls: sakila.AllTbls},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(testh.TName(tc.fpath), func(t *testing.T) {
			testh.SkipShort(t, tc.wantType == xlsx.Type)
			t.Parallel()

			f, err := os.Open(tc.fpath)
			require.NoError(t, err)
			ru := newRun(t)
			ru.rc.Stdin = f

			err = ru.exec("inspect", "--json")
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
