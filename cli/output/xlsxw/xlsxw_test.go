package xlsxw_test

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/neilotoole/sq/testh/testsrc"

	"github.com/neilotoole/sq/cli/output/xlsxw"

	"github.com/tealeg/xlsx/v2"

	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/stretchr/testify/require"
)

func TestRecordWriter(t *testing.T) {
	testCases := []struct {
		name     string
		handle   string
		tbl      string
		numRecs  int
		fixtPath string
	}{
		{name: "actor_0", handle: sakila.SL3, tbl: sakila.TblActor, numRecs: 0, fixtPath: "testdata/actor_0_rows.xlsx"},
		{name: "actor_3", handle: sakila.SL3, tbl: sakila.TblActor, numRecs: 3, fixtPath: "testdata/actor_3_rows.xlsx"},
		{name: "tbl_types_3", handle: testsrc.MiscDB, tbl: testsrc.TblTypes, numRecs: -1, fixtPath: "testdata/miscdb_tbl_types.xlsx"},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			recMeta, recs := testh.RecordsFromTbl(t, tc.handle, tc.tbl)
			if tc.numRecs >= 0 {
				recs = recs[0:tc.numRecs]
			}

			buf := &bytes.Buffer{}
			w := xlsxw.NewRecordWriter(buf, true)
			require.NoError(t, w.Open(recMeta))

			require.NoError(t, w.WriteRecords(recs))
			require.NoError(t, w.Close())

			want, err := ioutil.ReadFile(tc.fixtPath)
			require.NoError(t, err)
			requireEqualXLSX(t, want, buf.Bytes())
		})
	}
}

func requireEqualXLSX(t *testing.T, data1, data2 []byte) {
	xl1, err := xlsx.OpenBinary(data1)
	require.NoError(t, err)
	xl2, err := xlsx.OpenBinary(data2)
	require.NoError(t, err)

	parts1, err := xl1.MarshallParts()
	require.NoError(t, err)
	parts2, err := xl2.MarshallParts()
	require.NoError(t, err)

	for k1, v1 := range parts1 {
		v2, ok := parts2[k1]
		require.True(t, ok)
		require.Equal(t, v1, v2)
	}

	for k2 := range parts2 {
		_, ok := parts1[k2]
		require.True(t, ok)
	}

	vals1, err := xl1.ToSlice()
	require.NoError(t, err)
	vals2, err := xl2.ToSlice()
	require.NoError(t, err)

	require.Equal(t, vals1, vals2)
}
