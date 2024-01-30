package csv_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/loz"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/timez"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

func TestSmoke(t *testing.T) {
	t.Parallel()

	testCases := []string{sakila.CSVActor, sakila.TSVActor, sakila.CSVActorHTTP}

	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(sakila.CSVActor)

			sink, err := th.QuerySQL(src, nil, "SELECT * FROM data")
			require.NoError(t, err)
			require.Equal(t, len(sakila.TblActorCols()), len(sink.RecMeta))
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
		})
	}
}

func TestSakila_query(t *testing.T) {
	t.Parallel()
	tu.SkipIssueWindows(t, tu.GH355SQLiteDecimalWin)

	testCases := []struct {
		file      string
		wantCols  []string
		wantCount int
		wantKinds []kind.Kind
		wantRec0  record.Record
	}{
		{
			file:      sakila.TblActor,
			wantCols:  sakila.TblActorCols(),
			wantCount: sakila.TblActorCount,
			wantKinds: sakila.TblActorColKinds(),
			wantRec0: record.Record{
				int64(1), "PENELOPE", "GUINESS",
				time.Date(2020, time.February, 15, 6, 59, 28, 0, time.UTC),
			},
		},
		{
			file:      sakila.TblFilmActor,
			wantCols:  sakila.TblFilmActorCols(),
			wantCount: sakila.TblFilmActorCount,
			wantKinds: sakila.TblFilmActorColKinds(),
			wantRec0: record.Record{
				int64(1), int64(1),
				time.Date(2020, time.February, 15, 6, 59, 32, 0, time.UTC),
			},
		},
		{
			file:      sakila.TblPayment,
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

	for _, drvr := range []drivertype.Type{drivertype.CSV, drivertype.TSV} {
		drvr := drvr

		t.Run(drvr.String(), func(t *testing.T) {
			t.Parallel()

			for _, tc := range testCases {
				tc := tc

				t.Run(tc.file, func(t *testing.T) {
					t.Parallel()

					th := testh.New(t)
					src := th.Add(&source.Source{
						Handle:   "@" + tc.file,
						Type:     drvr,
						Location: filepath.Join("testdata", "sakila-"+drvr.String(), tc.file+"."+drvr.String()),
					})

					sink, err := th.QuerySLQ(src.Handle+".data", nil)
					require.NoError(t, err)
					gotCols, gotKinds := sink.RecMeta.MungedNames(), sink.RecMeta.Kinds()
					require.Equal(t, tc.wantCols, gotCols)
					assert.Equal(t, tc.wantKinds, gotKinds)
					assert.Equal(t, tc.wantCount, len(sink.Recs))
					if tc.wantRec0 != nil {
						require.EqualValues(t, tc.wantRec0, sink.Recs[0])
					}
				})
			}
		})
	}
}

func TestQuerySQL_Count(t *testing.T) {
	t.Parallel()

	testCases := []string{sakila.CSVActor, sakila.TSVActor}
	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)

			sink, err := th.QuerySQL(src, nil, "SELECT * FROM data")
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))

			sink, err = th.QuerySQL(src, nil, "SELECT COUNT(*) FROM data")
			require.NoError(t, err)
			require.EqualValues(t, int64(sakila.TblActorCount), sink.Result())
		})
	}
}

func TestEmptyAsNull(t *testing.T) {
	t.Parallel()

	// We've added a bunch of tu.OpenFileCount calls to debug
	// windows file close ordering issues. These should be
	// removed when done.

	t.Cleanup(func() {
		tu.OpenFileCount(t, true, "earliest cleanup (last exec)") // FIXME: delete this line
	})

	th := testh.New(t)
	tu.OpenFileCount(t, true, "top") // FIXME: delete this line

	sink, err := th.QuerySLQ(sakila.CSVAddress+`| .data | .[0:1]`, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
	tu.OpenFileCount(t, true, "after sink") // FIXME: delete this line

	require.Equal(t, stringz.Strings(sakila.TblAddressColKinds()), stringz.Strings(sink.RecMeta.Kinds()))

	ts, err := timez.ParseTimestampUTC("2020-02-15T06:59:28Z")
	require.NoError(t, err)

	rec0 := sink.Recs[0]
	want := []any{
		int64(1),
		"47 MySakila Drive",
		nil,
		nil,
		int64(300),
		nil,
		nil,
		ts,
	}

	for i := range want {
		require.EqualValues(t, want[i], rec0[i], "field [%d]", i)
	}
	tu.OpenFileCount(t, true, "bottom") // FIXME: delete this line
}

func TestIngest_DuplicateColumns(t *testing.T) {
	ctx := context.Background()
	tr := testrun.New(ctx, t, nil)

	err := tr.Exec(
		"add", filepath.Join("testdata", "actor_duplicate_cols.csv"),
		"--handle", "@actor_dup",
	)
	require.NoError(t, err)

	tr = testrun.New(ctx, t, tr).Hush()
	require.NoError(t, tr.Exec("--csv", ".data"))
	wantHeaders := []string{"actor_id", "first_name", "last_name", "last_update", "actor_id_1"}
	data := tr.BindCSV()
	require.Equal(t, wantHeaders, data[0])

	// Make sure the data is correct
	require.Len(t, data, sakila.TblActorCount+1) // +1 for header row
	wantFirstDataRecord := []string{"1", "PENELOPE", "GUINESS", "2020-02-15T06:59:28Z", "1"}
	require.Equal(t, wantFirstDataRecord, data[1])

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
	require.NoError(t, tr.Exec("--csv", ".data"))
	wantHeaders = []string{"x_actor_id", "x_first_name", "x_last_name", "x_last_update", "x_actor_id_1"}
	data = tr.BindCSV()
	require.Equal(t, wantHeaders, data[0])
}

func TestIngest_Kind_Timestamp(t *testing.T) {
	t.Parallel()

	wantNanoUTC := time.Unix(0, fixt.TimestampUnixNano1989).UTC()
	wantMilliUTC := wantNanoUTC.Truncate(time.Millisecond)
	wantSecUTC := wantNanoUTC.Truncate(time.Second)
	wantMinUTC := wantNanoUTC.Truncate(time.Minute)

	testCases := []struct {
		file        string
		wantHeaders []string
		wantVals    []time.Time
	}{
		{
			file: "test_timestamp",
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
				"UnixDate",
			},
			wantVals: []time.Time{
				wantSecUTC,   // ANSIC
				wantMinUTC,   // DateHourMinute
				wantSecUTC,   // DateHourMinuteSecond
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
				wantSecUTC,   // UnixDate
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := &source.Source{
				Handle:   "@tsv/" + tc.file,
				Type:     drivertype.TSV,
				Location: filepath.Join("testdata", tc.file+".tsv"),
			}
			src = th.Add(src)

			sink, err := th.QuerySLQ(src.Handle+".data", nil)
			require.NoError(t, err)

			assert.Equal(t, tc.wantHeaders, sink.RecMeta.MungedNames())
			require.Len(t, sink.Recs, 1)
			t.Log(sink.Recs[0])

			for i, col := range sink.RecMeta.MungedNames() {
				i, col := i, col
				t.Run(col, func(t *testing.T) {
					t.Logf("[%d] %s", i, col)
					assert.Equal(t, kind.Datetime.String(), sink.RecMeta.Kinds()[i].String())
					wantTime := tc.wantVals[i]
					gotTime, ok := sink.Recs[0][i].(time.Time)
					require.True(t, ok)
					t.Logf(
						"wantTime: %s  |  %s  |  %d  ",
						wantTime.Format(time.RFC3339Nano),
						wantTime.Location(),
						wantTime.Unix(),
					)
					t.Logf(
						" gotTime: %s  |  %s  |  %d  ",
						gotTime.Format(time.RFC3339Nano),
						gotTime.Location(),
						gotTime.Unix(),
					)
					assert.Equal(t, wantTime.Unix(), gotTime.Unix())
					assert.Equal(t, wantTime.UTC(), gotTime.UTC())
				})
			}
		})
	}
}

func TestIngest_Kind_Date(t *testing.T) {
	t.Parallel()

	wantDate := time.Date(1989, time.November, 9, 0, 0, 0, 0, time.UTC)

	testCases := []struct {
		file        string
		wantHeaders []string
		wantVals    []time.Time
	}{
		{
			file:        "test_date",
			wantHeaders: []string{"Long", "Short", "d-mmm-yy", "mm-dd-yy", "mmmm d, yyyy"},
			wantVals:    loz.Make(5, wantDate),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := &source.Source{
				Handle:   "@tsv/" + tc.file,
				Type:     drivertype.TSV,
				Location: filepath.Join("testdata", tc.file+".tsv"),
			}
			src = th.Add(src)

			sink, err := th.QuerySLQ(src.Handle+".data", nil)
			require.NoError(t, err)

			assert.Equal(t, tc.wantHeaders, sink.RecMeta.MungedNames())
			require.Len(t, sink.Recs, 1)
			t.Log(sink.Recs[0])

			for i, col := range sink.RecMeta.MungedNames() {
				i, col := i, col
				t.Run(col, func(t *testing.T) {
					t.Logf("[%d] %s", i, col)
					assert.Equal(t, kind.Date.String(), sink.RecMeta.Kinds()[i].String())
					gotTime, ok := sink.Recs[0][i].(time.Time)
					require.True(t, ok)
					wantTime := tc.wantVals[i]
					t.Logf("wantTime: %s  |  %s  |  %d  ", wantTime.Format(time.RFC3339Nano), wantTime.Location(), wantTime.Unix())
					t.Logf(" gotTime: %s  |  %s  |  %d  ", gotTime.Format(time.RFC3339Nano), gotTime.Location(), gotTime.Unix())
					assert.Equal(t, wantTime.Unix(), gotTime.Unix())
					assert.Equal(t, wantTime.UTC(), gotTime.UTC())
				})
			}
		})
	}
}

func TestIngest_Kind_Time(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		file        string
		wantHeaders []string
		wantVals    []string
	}{
		{
			file:        "test_time",
			wantHeaders: []string{"time1", "time2", "time3", "time4", "time5", "time6"},
			wantVals:    []string{"15:17:00", "15:17:00", "15:17:00", "15:17:00", "15:17:00", "15:17:59"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := &source.Source{
				Handle:   "@tsv/" + tc.file,
				Type:     drivertype.TSV,
				Location: filepath.Join("testdata", tc.file+".tsv"),
			}
			src = th.Add(src)

			sink, err := th.QuerySLQ(src.Handle+".data", nil)
			require.NoError(t, err)

			assert.Equal(t, tc.wantHeaders, sink.RecMeta.MungedNames())
			require.Len(t, sink.Recs, 1)
			t.Log(sink.Recs[0])

			for i, col := range sink.RecMeta.MungedNames() {
				i, col := i, col
				t.Run(col, func(t *testing.T) {
					t.Logf("[%d] %s", i, col)
					assert.Equal(t, kind.Time.String(), sink.RecMeta.Kinds()[i].String())
					gotTime, ok := sink.Recs[0][i].(string)
					require.True(t, ok)
					wantTime := tc.wantVals[i]
					t.Logf("wantTime: %s", wantTime)
					t.Logf(" gotTime: %s", gotTime)
					assert.Equal(t, wantTime, gotTime)
				})
			}
		})
	}
}

// TestGenerateTimestampVals is a utility test that prints out a bunch
// of timestamp values in various time formats and locations.
// It was used to generate values for use in testdata/test_timestamp.tsv.
// It can probably be deleted when we're satisfied with date/time testing.
func TestGenerateTimestampVals(t *testing.T) {
	canonicalTimeUTC := time.Unix(0, fixt.TimestampUnixNano1989).UTC()
	_ = canonicalTimeUTC

	names := maps.Keys(timez.TimestampLayouts)
	slices.Sort(names)

	for _, loc := range []*time.Location{time.UTC, fixt.LosAngeles, fixt.Denver} {
		fmt.Fprintf(os.Stdout, "\n\n%s\n\n", loc.String())
		tm := canonicalTimeUTC.In(loc)

		for _, name := range names {
			layout := timez.TimestampLayouts[name]
			fmt.Fprintf(os.Stdout, "%32s: %s\n", name, tm.Format(layout))
		}
	}

	t.Logf("\n\n")
}
