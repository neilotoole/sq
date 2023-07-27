package excelw_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/gif"
	"os"
	"testing"

	"github.com/neilotoole/sq/testh/fixt"

	"github.com/neilotoole/sq/libsq/source"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/xuri/excelize/v2"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/testsrc"

	"github.com/neilotoole/sq/cli/output/excelw"

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
		{
			name:     "actor_0",
			handle:   sakila.SL3,
			tbl:      sakila.TblActor,
			numRecs:  0,
			fixtPath: "testdata/actor_0_rows.xlsx",
		},
		{
			name:     "actor_3",
			handle:   sakila.SL3,
			tbl:      sakila.TblActor,
			numRecs:  3,
			fixtPath: "testdata/actor_3_rows.xlsx",
		},
		{
			name:     "tbl_types_3",
			handle:   testsrc.MiscDB,
			tbl:      testsrc.TblTypes,
			numRecs:  -1,
			fixtPath: "testdata/miscdb_tbl_types.xlsx",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			recMeta, recs := testh.RecordsFromTbl(t, tc.handle, tc.tbl)
			if tc.numRecs >= 0 {
				recs = recs[0:tc.numRecs]
			}

			buf := &bytes.Buffer{}
			pr := output.NewPrinting()
			w := excelw.NewRecordWriter(buf, pr)
			require.NoError(t, w.Open(recMeta))

			require.NoError(t, w.WriteRecords(recs))
			require.NoError(t, w.Close())

			_ = tutil.WriteTemp(t, fmt.Sprintf("*.%s.test.xlsx", tc.name), buf.Bytes(), false)

			want, err := os.ReadFile(tc.fixtPath)
			require.NoError(t, err)
			requireEqualXLSX(t, want, buf.Bytes())
		})
	}
}

func TestBytesEncodedAsBase64(t *testing.T) {
	recMeta, recs := testh.RecordsFromTbl(t, testsrc.BlobDB, "blobs")

	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	w := excelw.NewRecordWriter(buf, pr)
	require.NoError(t, w.Open(recMeta))

	require.NoError(t, w.WriteRecords(recs))
	require.NoError(t, w.Close())

	xl, err := excelize.OpenReader(buf)
	require.NoError(t, err)
	cellStr, err := xl.GetCellValue(source.MonotableName, "B2")
	require.NoError(t, err)

	b, err := base64.StdEncoding.DecodeString(cellStr)
	require.NoError(t, err)

	require.Equal(t, fixt.GopherSize, len(b))

	gopher, err := gif.Decode(bytes.NewReader(b))
	require.NoError(t, err)
	size := gopher.Bounds().Size()
	require.Equal(t, image.Point{X: 64, Y: 64}, size)
}

func requireEqualXLSX(t *testing.T, data1, data2 []byte) {
	xl1, err := excelize.OpenReader(bytes.NewReader(data1))
	require.NoError(t, err)

	xl2, err := excelize.OpenReader(bytes.NewReader(data2))
	require.NoError(t, err)

	require.Equal(t, xl1.SheetCount, xl2.SheetCount)

	sheetNames1, sheetNames2 := xl1.GetSheetList(), xl2.GetSheetList()
	require.Equal(t, sheetNames1, sheetNames2)

	for _, sheetName := range sheetNames1 {
		rows1, err := xl1.GetRows(sheetName)
		require.NoError(t, err)
		rows2, err := xl2.GetRows(sheetName)
		require.NoError(t, err)

		require.Equal(t, len(rows1), len(rows2),
			"sheet {%s}: number of rows not equal", sheetName)

		for i := range rows1 {
			row1 := rows1[i]
			row2 := rows2[i]
			require.Equal(t, row1, row2, "sheet {%s}: row[%d} not equal", sheetName, i)
		}
	}
}
