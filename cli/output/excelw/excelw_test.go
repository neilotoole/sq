package excelw_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/gif"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/neilotoole/sq/cli/testrun"

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
			pr.ExcelDatetimeFormat = excelw.OptDatetimeFormat.Default()
			pr.ExcelDateFormat = excelw.OptDateFormat.Default()
			pr.ExcelTimeFormat = excelw.OptTimeFormat.Default()
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

func readCellValue(t *testing.T, fpath, sheet, cell string) string {
	xl, err := excelize.OpenFile(fpath)
	require.NoError(t, err)

	val, err := xl.GetCellValue(sheet, cell)
	require.NoError(t, err)

	return val
}

func TestOptDatetimeFormats(t *testing.T) {
	const query = `SELECT
'1989-11-09T16:07:01Z'::timestamp AS date_time,
'1989-11-09T16:07:01Z'::date AS date_only,
('1989-11-09T16:07:01Z'::timestamp)::time AS time_only`

	th := testh.New(t)
	src := th.Source(sakila.Pg)

	tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
	require.NoError(t, tr.Exec("config", "set", excelw.OptDatetimeFormat.Key(), "yy/mm/dd - hh:mm:ss"))
	tr = testrun.New(th.Context, t, tr)
	require.NoError(t, tr.Exec("config", "set", excelw.OptDateFormat.Key(), "yy/mmm/dd"))
	tr = testrun.New(th.Context, t, tr)
	require.NoError(t, tr.Exec("config", "set", excelw.OptTimeFormat.Key(), "h:mm am/pm"))

	tr = testrun.New(th.Context, t, tr)
	require.NoError(t, tr.Exec("sql", "--xlsx", query))

	fpath := tutil.WriteTemp(t, "*.xlsx", tr.Out.Bytes(), true)

	gotDatetime := readCellValue(t, fpath, source.MonotableName, "A2")
	gotDate := readCellValue(t, fpath, source.MonotableName, "B2")
	gotTime := readCellValue(t, fpath, source.MonotableName, "C2")

	assert.Equal(t, "89/11/09 - 16:07:01", gotDatetime)
	assert.Equal(t, "89/Nov/09", gotDate)
	assert.Equal(t, "4:07 pm", gotTime)
}
