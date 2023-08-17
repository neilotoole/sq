package csv_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/neilotoole/sq/drivers/csv"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/loz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/neilotoole/sq/libsq/driver"

	"github.com/neilotoole/sq/cli/testrun"

	"github.com/neilotoole/sq/libsq/core/timez"

	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
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

			sink, err := th.QuerySQL(src, "SELECT * FROM data")
			require.NoError(t, err)
			require.Equal(t, len(sakila.TblActorCols()), len(sink.RecMeta))
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
		})
	}
}

func TestSakila_query(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		file      string
		wantCols  []string
		wantCount int
		wantKinds []kind.Kind
	}{
		{
			file:      sakila.TblActor,
			wantCols:  sakila.TblActorCols(),
			wantCount: sakila.TblActorCount,
			wantKinds: sakila.TblActorColKinds(),
		},
		{
			file:      sakila.TblFilmActor,
			wantCols:  sakila.TblFilmActorCols(),
			wantCount: sakila.TblFilmActorCount,
			wantKinds: sakila.TblFilmActorColKinds(),
		},
		{
			file:      sakila.TblPayment,
			wantCols:  sakila.TblPaymentCols(),
			wantCount: sakila.TblPaymentCount,
			wantKinds: sakila.TblPaymentColKinds(),
		},
	}

	for _, driver := range []source.DriverType{csv.TypeCSV, csv.TypeTSV} {
		driver := driver

		t.Run(driver.String(), func(t *testing.T) {
			t.Parallel()

			for _, tc := range testCases {
				tc := tc

				t.Run(tc.file, func(t *testing.T) {
					t.Parallel()

					th := testh.New(t, testh.OptLongOpen())
					src := th.Add(&source.Source{
						Handle:   "@" + tc.file,
						Type:     driver,
						Location: filepath.Join("testdata", "sakila-"+driver.String(), tc.file+"."+driver.String()),
					})

					sink, err := th.QuerySLQ(src.Handle+".data", nil)
					require.NoError(t, err)
					gotCols, gotKinds := sink.RecMeta.MungedNames(), sink.RecMeta.Kinds()
					require.Equal(t, tc.wantCols, gotCols)
					require.Equal(t, tc.wantKinds, gotKinds)
					require.Equal(t, tc.wantCount, len(sink.Recs))
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

			sink, err := th.QuerySQL(src, "SELECT * FROM data")
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))

			sink, err = th.QuerySQL(src, "SELECT COUNT(*) FROM data")
			require.NoError(t, err)
			require.EqualValues(t, int64(sakila.TblActorCount), sink.Result())
		})
	}
}

func TestEmptyAsNull(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	sink, err := th.QuerySLQ(sakila.CSVAddress+`| .data | .[0:1]`, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))

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
}

func TestIngestDuplicateColumns(t *testing.T) {
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

func TestDatetime(t *testing.T) {
	t.Parallel()

	denver, err := time.LoadLocation("America/Denver")
	require.NoError(t, err)

	wantDtNanoUTC := time.Date(1989, 11, 9, 15, 17, 59, 123456700, time.UTC)
	wantDtMilliUTC := wantDtNanoUTC.Truncate(time.Millisecond)
	wantDtSecUTC := wantDtNanoUTC.Truncate(time.Second)
	wantDtMinUTC := wantDtNanoUTC.Truncate(time.Minute)
	wantDtNanoMST := time.Date(1989, 11, 9, 15, 17, 59, 123456700, denver)
	wantDtMilliMST := wantDtNanoMST.Truncate(time.Millisecond)
	wantDtSecMST := wantDtNanoMST.Truncate(time.Second)
	wantDtMinMST := wantDtNanoMST.Truncate(time.Minute)

	testCases := []struct {
		file        string
		wantHeaders []string
		wantKinds   []kind.Kind
		wantVals    []any
	}{
		{
			file:        "test_date",
			wantHeaders: []string{"Long", "Short", "d-mmm-yy", "mm-dd-yy", "mmmm d, yyyy"},
			wantKinds:   loz.Make(5, kind.Date),
			wantVals: lo.ToAnySlice(loz.Make(5,
				time.Date(1989, time.November, 9, 0, 0, 0, 0, time.UTC))),
		},
		{
			file:        "test_time",
			wantHeaders: []string{"time1", "time2", "time3", "time4", "time5", "time6"},
			wantKinds:   loz.Make(6, kind.Time),
			wantVals:    []any{"15:17:00", "15:17:00", "15:17:00", "15:17:00", "15:17:00", "15:17:59"},
		},
		{
			file: "test_datetime",
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
			wantKinds: loz.Make(20, kind.Datetime),
			wantVals: lo.ToAnySlice([]time.Time{
				wantDtSecUTC,   // ANSIC
				wantDtMinUTC,   // DateHourMinute
				wantDtSecUTC,   // DateHourMinuteSecond
				wantDtMilliMST, // ISO8601
				wantDtMilliUTC, // ISO8601Z
				wantDtSecMST,   // RFC1123
				wantDtSecMST,   // RFC1123Z
				wantDtSecMST,   // RFC3339
				wantDtNanoMST,  // RFC3339Nano
				wantDtNanoUTC,  // RFC3339NanoZ
				wantDtSecUTC,   // RFC3339Z
				wantDtMinMST,   // RFC8222
				wantDtMinMST,   // RFC8222Z
				wantDtSecMST,   // RFC850
				wantDtSecMST,   // RubyDate
				wantDtMinUTC,   // Stamp
				wantDtMinUTC,   // StampMicro
				wantDtMinUTC,   // StampMilli
				wantDtMinUTC,   // StampNano
				wantDtSecMST,   // UnixDate
			}),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t, testh.OptLongOpen())
			src := &source.Source{
				Handle:   "@tsv/" + tc.file,
				Type:     csv.TypeTSV,
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
					assert.Equal(t, tc.wantKinds[i].String(), sink.RecMeta.Kinds()[i].String())
					if gotTime, ok := sink.Recs[0][i].(time.Time); ok {
						// REVISIT: If it's a time value, we want to compare UTC times.
						// This may actually be a bug.
						wantTime, ok := tc.wantVals[i].(time.Time)
						require.True(t, ok)
						assert.Equal(t, wantTime.Unix(), gotTime.Unix())
						assert.Equal(t, wantTime.UTC(), gotTime.UTC())
					} else {
						assert.EqualValues(t, tc.wantVals[i], sink.Recs[0][i])
					}
				})
			}
		})
	}
}
