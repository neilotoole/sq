package xlsx_test

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/langz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/timez"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

var sakilaSheets = []string{
	"actor",
	"address",
	"category",
	"city",
	"country",
	"customer",
	"film",
	"film_actor",
	"film_category",
	"film_text",
	"inventory",
	"language",
	"payment",
	"rental",
	"staff",
	"store",
}

func TestSakilaInspectSource(t *testing.T) {
	t.Parallel()
	tu.SkipIssueWindows(t, tu.GH371ExcelSlowWin)
	tu.SkipShort(t, true)

	th := testh.New(t)
	src := th.Source(sakila.XLSX)

	tr := testrun.New(th.Context, t, nil).Hush().Add(*src)

	err := tr.Exec("inspect", "--json", src.Handle)
	require.NoError(t, err)
}

func TestSakilaInspectSheets(t *testing.T) {
	t.Parallel()
	tu.SkipIssueWindows(t, tu.GH371ExcelSlowWin)
	tu.SkipShort(t, true)

	for _, sheet := range sakilaSheets {
		sheet := sheet

		t.Run(sheet, func(t *testing.T) {
			t.Parallel()
			th := testh.New(t)
			src := th.Source(sakila.XLSX)

			tr := testrun.New(th.Context, t, nil).Hush().Add(*src)

			err := tr.Exec("inspect", "--json", src.Handle+"."+sheet)
			require.NoError(t, err)
		})
	}
}

func BenchmarkInspectSheets(b *testing.B) {
	tu.SkipIssueWindows(b, tu.GH371ExcelSlowWin)
	tu.SkipShort(b, true)

	for _, sheet := range sakilaSheets {
		sheet := sheet

		b.Run(sheet, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				th := testh.New(b)
				src := th.Source(sakila.XLSX)

				tr := testrun.New(th.Context, b, nil).Hush().Add(*src)

				err := tr.Exec("inspect", "--json", src.Handle+"."+sheet)
				if err != nil {
					b.Error(err)
				}
			}
		})
	}
}

func TestSakila_query_cmd(t *testing.T) {
	t.Parallel()
	tu.SkipIssueWindows(t, tu.GH371ExcelSlowWin)
	tu.SkipShort(t, true)

	for _, sheet := range sakilaSheets {
		sheet := sheet

		t.Run(sheet, func(t *testing.T) {
			t.Parallel()
			th := testh.New(t)
			src := th.Source(sakila.XLSX)

			tr := testrun.New(th.Context, t, nil).Hush().Add(*src)

			err := tr.Exec("--jsonl", "."+sheet)
			require.NoError(t, err)
			t.Log("\n", tr.Out.String())
		})
	}
}

func TestOpenFileFormats(t *testing.T) {
	t.Parallel()
	tu.SkipIssueWindows(t, tu.GH371ExcelSlowWin)
	tu.SkipShort(t, true)

	testCases := []struct {
		filename string
		wantErr  bool
	}{
		{"sakila.xlsx", false},
		{"sakila.xlam", false},
		{"sakila.xlsm", false},
		{"sakila.xltm", false},
		{"sakila.xltx", false},
		{"sakila.strict_openxml.xlsx", false},

		// .xls and .xlsb aren't supported. Perhaps one day we'll incorporate
		// support via a library such as https://github.com/extrame/xls.
		{"sakila.xls", true},
		{"sakila.xlsb", true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.filename, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Add(&source.Source{
				Handle:   "@excel",
				Type:     drivertype.XLSX,
				Location: filepath.Join("testdata", "file_formats", tc.filename),
			})

			grip, err := th.Grips().Open(th.Context, src)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			db, err := grip.DB(th.Context)
			require.NoError(t, err)
			err = db.PingContext(th.Context)
			require.NoError(t, err)

			sink, err := th.QuerySQL(src, nil, "SELECT * FROM actor")

			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCols(), sink.RecMeta.MungedNames())
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
			require.Equal(t, sakila.TblActorColKinds(), sink.RecMeta.Kinds())
			wantRec0 := record.Record{
				int64(1), "PENELOPE", "GUINESS",
				time.Date(2020, time.February, 15, 6, 59, 28, 0, time.UTC),
			}
			require.Equal(t, wantRec0, sink.Recs[0])
		})
	}
}

func TestSakila_query(t *testing.T) {
	t.Parallel()
	tu.SkipIssueWindows(t, tu.GH371ExcelSlowWin)
	tu.SkipShort(t, true)

	testCases := []struct {
		sheet     string
		wantCols  []string
		wantCount int
		wantKinds []kind.Kind
		wantRec0  record.Record
	}{
		{
			sheet:     sakila.TblActor,
			wantCols:  sakila.TblActorCols(),
			wantCount: sakila.TblActorCount,
			wantKinds: sakila.TblActorColKinds(),
			wantRec0: record.Record{
				int64(1), "PENELOPE", "GUINESS",
				time.Date(2020, time.February, 15, 6, 59, 28, 0, time.UTC),
			},
		},
		{
			sheet:     sakila.TblFilmActor,
			wantCols:  sakila.TblFilmActorCols(),
			wantCount: sakila.TblFilmActorCount,
			wantKinds: sakila.TblFilmActorColKinds(),
			wantRec0: record.Record{
				int64(1), int64(1),
				time.Date(2020, time.February, 15, 6, 59, 32, 0, time.UTC),
			},
		},
		{
			sheet:     sakila.TblPayment,
			wantCols:  sakila.TblPaymentCols(),
			wantCount: sakila.TblPaymentCount,
			wantKinds: sakila.TblPaymentColKinds(),
			wantRec0: record.Record{
				int64(1), int64(1), int64(1), int64(76), decimal.New(299, -2),
				time.Date(2005, time.May, 25, 11, 30, 37, 0, time.UTC),
				time.Date(2020, time.February, 15, 6, 59, 47, 0, time.UTC),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.sheet, func(t *testing.T) {
			t.Parallel()
			th := testh.New(t)
			src := th.Source(sakila.XLSX)

			sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+tc.sheet)
			require.NoError(t, err)
			require.Equal(t, tc.wantCols, sink.RecMeta.MungedNames())
			require.Equal(t, tc.wantCount, len(sink.Recs))
			require.Equal(t, tc.wantKinds, sink.RecMeta.Kinds())
			require.Equal(t, tc.wantRec0, sink.Recs[0])
		})
	}
}

func Test_XLSX_BadDateRecognition(t *testing.T) {
	t.Parallel()

	th := testh.New(t)

	src := &source.Source{
		Handle:   "@xlsx_bad_date",
		Type:     drivertype.XLSX,
		Location: proj.Abs("drivers/xlsx/testdata/problem_with_recognizing_date_colA.xlsx"),
		Options:  options.Options{driver.OptIngestHeader.Key(): true},
	}

	require.True(t, src.Options.IsSet(driver.OptIngestHeader))

	hasHeader := driver.OptIngestHeader.Get(src.Options)
	require.True(t, hasHeader)

	sink, err := th.QuerySQL(src, nil, "SELECT * FROM Summary")
	require.NoError(t, err)
	require.Equal(t, 21, len(sink.Recs))
}

// TestHandleSomeSheetsEmpty verifies that sq can import XLSX
// when there are some empty sheets.
func TestHandleSomeSheetsEmpty(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	src := th.Add(&source.Source{
		Handle:   "@xlsx_empty_sheets",
		Type:     drivertype.XLSX,
		Location: "testdata/some_sheets_empty.xlsx",
	})

	srcMeta, err := th.SourceMetadata(src)
	require.NoError(t, err)
	tblNames := srcMeta.TableNames()
	require.Len(t, tblNames, 1)
	require.Equal(t, []string{"Sheet1"}, tblNames)

	for _, tblName := range []string{"Sheet2Empty, Sheet3Empty"} {
		_, err = th.TableMetadata(src, tblName)
		require.Error(t, err)
		require.True(t, errz.Has[*driver.NotExistError](err))
	}
}

func TestIngestDuplicateColumns(t *testing.T) {
	actorDataRow0 := []string{"1", "PENELOPE", "GUINESS", "2020-02-15T06:59:28Z", "1"}

	ctx := context.Background()
	tr := testrun.New(ctx, t, nil).Hush()

	err := tr.Exec("add",
		"--handle", "@actor_dup",
		"--ingest.header=true",
		filepath.Join("testdata", "actor_duplicate_cols.xlsx"),
	)
	require.NoError(t, err)

	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("--csv", ".actor"))
	wantHeaders := []string{"actor_id", "first_name", "last_name", "last_update", "actor_id_1"}
	data := tr.BindCSV()
	require.Equal(t, wantHeaders, data[0])

	// Make sure the data is correct
	require.Len(t, data, sakila.TblActorCount+1) // +1 for header row
	require.Equal(t, actorDataRow0, data[1])

	// Verify that changing the template works
	const tpl2 = "x_{{.Name}}{{with .Recurrence}}_{{.}}{{end}}"

	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec(
		"config",
		"set",
		driver.OptIngestColRename.Key(),
		tpl2,
	))
	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("--csv", ".actor"))
	wantHeaders = []string{"x_actor_id", "x_first_name", "x_last_name", "x_last_update", "x_actor_id_1"}
	data = tr.BindCSV()
	require.Equal(t, wantHeaders, data[0])
}

func TestDetectHeaderRow(t *testing.T) {
	actorRows := [][]string{
		{"1", "PENELOPE", "GUINESS", "2020-02-15T06:59:28Z"},
		{"2", "NICK", "WAHLBERG", "2020-02-15T06:59:28Z"},
		{"3", "ED", "CHASE", "2020-02-15T06:59:28Z"},
	}
	abcd := []string{"A", "B", "C", "D"}

	testCases := []struct {
		filename        string
		wantRecordCount int
		matchRecords    [][]string
	}{
		{
			filename:        "actor_header.xlsx",
			wantRecordCount: sakila.TblActorCount + 1,
			matchRecords:    [][]string{sakila.TblActorCols(), actorRows[0], actorRows[1], actorRows[2]},
		},
		{
			filename:        "actor_no_header.xlsx",
			wantRecordCount: sakila.TblActorCount + 1,
			matchRecords:    [][]string{abcd, actorRows[0], actorRows[1], actorRows[2]},
		},
		{
			filename:        "actor_double_header.xlsx",
			wantRecordCount: sakila.TblActorCount + 3,
			matchRecords:    [][]string{abcd, sakila.TblActorCols(), sakila.TblActorCols(), actorRows[0]},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.filename, func(t *testing.T) {
			ctx := context.Background()
			fp := filepath.Join("testdata", tc.filename)

			tr := testrun.New(ctx, t, nil).Hush()
			err := tr.Exec("add", fp)
			require.NoError(t, err)

			tr = testrun.New(ctx, t, tr)
			require.NoError(t, tr.Exec("--csv", "--header", ".actor"))

			data := tr.BindCSV()

			for _, rec := range data {
				t.Log(rec)
			}

			require.Equal(t, tc.wantRecordCount, len(data))

			require.True(t, len(data) >= len(tc.matchRecords))
			for i, wantRec := range tc.matchRecords {
				gotRec := data[i]
				require.Equal(t, wantRec, gotRec, "record %d", i)
			}
		})
	}
}

var datetimeFormats = map[string]string{
	"RFC3339":              time.RFC3339,
	"RFC3339Z":             timez.RFC3339Z,
	"ISO8601":              timez.ISO8601,
	"ISO8601Z":             timez.ISO8601Z,
	"RFC3339Nano":          time.RFC3339Nano,
	"RFC3339NanoZ":         timez.RFC3339NanoZ,
	"ANSIC":                time.ANSIC,
	"UnixDate":             time.UnixDate,
	"RubyDate":             time.RubyDate,
	"RFC8222":              time.RFC822,
	"RFC8222Z":             time.RFC822Z,
	"RFC850":               time.RFC850,
	"RFC1123":              time.RFC1123,
	"RFC1123Z":             time.RFC1123Z,
	"Stamp":                time.Stamp,
	"StampMilli":           time.StampMilli,
	"StampMicro":           time.StampMicro,
	"StampNano":            time.StampNano,
	"DateHourMinuteSecond": timez.DateHourMinuteSecond,
	"DateHourMinute":       timez.DateHourMinute,
}

func TestDates(t *testing.T) {
	denver, err := time.LoadLocation("America/Denver")
	require.NoError(t, err)
	tm := time.Date(1989, 11, 9, 15, 17, 59, 123456700, denver)

	keys := maps.Keys(datetimeFormats)
	slices.Sort(keys)

	for _, k := range keys {
		format := datetimeFormats[k]
		s := tm.Format(format)
		t.Logf("%25s    %s", k, s)
	}

	t.Logf("%#v", keys)
}

func TestDatetime(t *testing.T) {
	t.Parallel()

	src := &source.Source{
		Handle:   "@excel/datetime",
		Type:     drivertype.XLSX,
		Location: "testdata/datetime.xlsx",
	}

	wantNanoUTC := time.Unix(0, fixt.TimestampUnixNano1989).UTC()
	wantMilliUTC := wantNanoUTC.Truncate(time.Millisecond)
	wantSecUTC := wantNanoUTC.Truncate(time.Second)
	wantMinUTC := wantNanoUTC.Truncate(time.Minute)

	testCases := []struct {
		sheet       string
		wantHeaders []string
		wantKinds   []kind.Kind
		wantVals    []any
	}{
		{
			sheet:       "date",
			wantHeaders: []string{"Long", "Short", "d-mmm-yy", "mm-dd-yy", "mmmm d, yyyy"},
			wantKinds:   langz.Make(5, kind.Date),
			wantVals: lo.ToAnySlice(langz.Make(5,
				time.Date(1989, time.November, 9, 0, 0, 0, 0, time.UTC))),
		},
		{
			sheet:       "time",
			wantHeaders: []string{"time1", "time2", "time3", "time4", "time5", "time6"},
			wantKinds:   langz.Make(6, kind.Time),
			wantVals:    []any{"15:17:00", "15:17:00", "15:17:00", "15:17:00", "15:17:00", "15:17:59"},
		},
		{
			sheet: "datetime",
			wantHeaders: []string{
				"ANSIC",
				"DateHourMinute",
				"DateHourMinuteSecond",
				"ISO8601",
				"ISO8601Z",
				"RFC1123",
				"RFC1123Z",
				"RFC3339",
				"RFC3339Nano",
				"RFC3339NanoZ",
				"RFC3339Z",
				"RFC8222",
				"RFC8222Z",
				"RFC850",
				"RubyDate",
				"Stamp",
				"StampMicro",
				"StampMilli",
				"StampNano",
				"UnixDate",
			},
			wantKinds: langz.Make(20, kind.Datetime),
			wantVals: lo.ToAnySlice([]time.Time{
				wantSecUTC,   // ANSIC
				wantMinUTC,   // DateHourMinute
				wantMinUTC,   // DateHourMinuteSecond
				wantMilliUTC, // ISO8601
				wantMilliUTC, // ISO8601Z
				wantSecUTC,   // RFC1123
				wantSecUTC,   // RFC1123Z
				wantSecUTC,   // RFC3339
				wantNanoUTC,  // RFC3339Nano
				wantNanoUTC,  // RFC3339NanoZ
				wantSecUTC,   // RFC3339Z
				wantMinUTC,   // RFC8222
				wantMinUTC,   // RFC8222Z
				wantSecUTC,   // RFC850
				wantSecUTC,   // RubyDate
				wantMinUTC,   // Stamp
				wantMinUTC,   // StampMicro
				wantMinUTC,   // StampMilli
				wantMinUTC,   // StampNano
				wantSecUTC,   // UnixDate
			}),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.sheet, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src = th.Add(src)

			sink, err := th.QuerySLQ(src.Handle+"."+tc.sheet, nil)
			require.NoError(t, err)

			assert.Equal(t, tc.wantHeaders, sink.RecMeta.MungedNames())
			require.Len(t, sink.Recs, 1)
			t.Log(sink.Recs[0])

			for i, col := range sink.RecMeta.MungedNames() {
				i, col := i, col
				t.Run(col, func(t *testing.T) {
					assert.Equal(t, tc.wantKinds[i].String(), sink.RecMeta.Kinds()[i].String())
					if gotTime, ok := sink.Recs[0][i].(time.Time); ok {
						// REVISIT: If it's a time value, we want to compare UTC times.
						// This may actually be a bug.
						wantTime, ok := tc.wantVals[i].(time.Time)
						require.True(t, ok)
						require.Equal(t, wantTime.Unix(), gotTime.Unix())
						require.Equal(t, wantTime.UTC(), gotTime.UTC())
					} else {
						assert.EqualValues(t, tc.wantVals[i], sink.Recs[0][i])
					}
				})
			}
		})
	}
}
