package timez_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/timez"
	"github.com/neilotoole/sq/testh/tu"
)

var (
	mar1UTC, _  = timez.ParseDateUTC("2023-03-01")
	denver      *time.Location
	nov12denver time.Time
)

func init() { //nolint:gochecknoinits
	var err error
	if denver, err = time.LoadLocation("America/Denver"); err != nil {
		panic(err)
	}
	nov12denver = time.Date(2020, 11, 12, 13, 14, 15, 12345678, denver)
}

func TestTimestampUTC(t *testing.T) {
	tm := time.Date(2021, 0o1, 0o1, 7, 7, 7, 0, time.UTC)
	s := timez.TimestampUTC(tm)
	require.Equal(t, "2021-01-01T07:07:07.000Z", s)

	// A non-UTC instant must be converted to UTC before formatting.
	// nov12denver is 2020-11-12 13:14:15 MST (-0700) == 20:14:15 UTC.
	require.Equal(t, "2020-11-12T20:14:15.012Z", timez.TimestampUTC(nov12denver))

	s = timez.TimestampUTC(time.Now().UTC())
	t.Log(s)
}

func TestDateUTC(t *testing.T) {
	// A Denver-evening instant that has already rolled over to the next
	// day in UTC, proving DateUTC converts to UTC before formatting.
	// 2020-11-12 20:00 MST == 2020-11-13 03:00 UTC.
	tm := time.Date(2020, 11, 12, 20, 0, 0, 0, denver)
	require.Equal(t, "2020-11-13", timez.DateUTC(tm))

	require.Equal(t, "2023-03-01", timez.DateUTC(mar1UTC))
}

func TestTimestampToRFC3339(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{in: "", want: ""},
		{in: "not_a_time", want: ""},
		{in: "2022-12-30T09:36:31-0700", want: "2022-12-30T16:36:31Z"},      // RFC3339Variant
		{in: "2021-01-01T07:07:07.000Z", want: "2021-01-01T07:07:07Z"},      // ISO8601 (millis dropped)
		{in: "2021-01-16T18:26:39-07:00", want: "2021-01-17T01:26:39Z"},     // RFC3339
		{in: "2021-01-16T18:26:39.216-07:00", want: "2021-01-17T01:26:39Z"}, // RFC3339 w/ fractional
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			require.Equal(t, tc.want, timez.TimestampToRFC3339(tc.in))
		})
	}
}

func TestTimestampToDate(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{in: "", want: ""},
		{in: "not_a_time", want: ""},
		{in: "2021-01-01T07:07:07.000Z", want: "2021-01-01"},
		// 2021-01-16 18:26:39 -0700 == 2021-01-17 01:26:39 UTC, so the
		// UTC date is the following day.
		{in: "2021-01-16T18:26:39-07:00", want: "2021-01-17"},
		{in: "2022-12-30T09:36:31-0700", want: "2022-12-30"},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			require.Equal(t, tc.want, timez.TimestampToDate(tc.in))
		})
	}
}

func TestParseTimestampUTC(t *testing.T) {
	testCases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{in: "", wantErr: true},
		{in: "not_a_time", wantErr: true},
		{in: "2021-01-16", wantErr: true},                       // date only, not a timestamp
		{in: "2021-01-16T18:18:49.348-0700", want: 1610846329},  // RFC3339Variant
		{in: "2021-01-16T18:26:39.216-07:00", want: 1610846799}, // RFC3339 w/ fractional
		{in: "2021-01-16T18:26:39-07:00", want: 1610846799},     // RFC3339
		{in: "2021-01-17T01:26:39.216Z", want: 1610846799},      // ISO8601
		{in: "2021-01-17T01:26:39Z", want: 1610846799},          // RFC3339 zulu
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			tm, err := timez.ParseTimestampUTC(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, time.UTC, tm.Location())
			require.Equal(t, tc.want, tm.Unix())
		})
	}
}

func TestParseLocalDate(t *testing.T) {
	tm, err := timez.ParseLocalDate("2023-03-01")
	require.NoError(t, err)
	require.Equal(t, time.Local, tm.Location())
	y, m, d := tm.Date()
	require.Equal(t, 2023, y)
	require.Equal(t, time.March, m)
	require.Equal(t, 1, d)
	require.Equal(t, 0, tm.Hour())
	require.Equal(t, 0, tm.Minute())
	require.Equal(t, 0, tm.Second())

	// A 'T' marks it as a timestamp, which is rejected.
	_, err = timez.ParseLocalDate("2023-03-01T00:00:00Z")
	require.Error(t, err)

	// A malformed date is rejected by the underlying parse.
	_, err = timez.ParseLocalDate("not_a_date")
	require.Error(t, err)
}

func TestParseDateUTC(t *testing.T) {
	tm, err := timez.ParseDateUTC("2023-03-01")
	require.NoError(t, err)
	require.Equal(t, time.UTC, tm.Location())
	require.Equal(t, "2023-03-01", tm.Format(time.DateOnly))
	require.Equal(t, 0, tm.Hour())

	// A 'T' marks it as a timestamp, which is rejected.
	_, err = timez.ParseDateUTC("2023-03-01T00:00:00Z")
	require.Error(t, err)

	// A malformed date is rejected by the underlying parse.
	_, err = timez.ParseDateUTC("not_a_date")
	require.Error(t, err)
}

func TestParseDateOrTimestampUTC(t *testing.T) {
	testCases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{in: "", wantErr: true},
		{in: "not_a_time", wantErr: true},
		{in: "2021-01-16T18:18:49.348-0700", want: 1610846329},
		{in: "2021-01-16T18:26:39.216-07:00", want: 1610846799},
		{in: "2021-01-16T18:26:39-07:00", want: 1610846799},
		{in: "2021-01-17T01:26:39.216Z", want: 1610846799},
		{in: "2021-01-17", want: 1610841600},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			tm, err := timez.ParseDateOrTimestampUTC(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, time.UTC, tm.Location())
			require.Equal(t, tc.want, tm.Unix())
		})
	}
}

func TestMustParse(t *testing.T) {
	tm := timez.MustParse(time.RFC3339, "2021-01-16T18:26:39-07:00")
	require.Equal(t, int64(1610846799), tm.Unix())

	require.Panics(t, func() {
		timez.MustParse(time.RFC3339, "not_a_time")
	})
}

func TestZuluTimestamp(t *testing.T) {
	const (
		input = `2022-12-30T09:36:31-0700`
		want  = `2022-12-30T16:36:31Z`
	)
	got := timez.TimestampToRFC3339(input)
	require.Equal(t, want, got)
}

func TestNamedLayouts(t *testing.T) {
	got := timez.NamedLayouts()
	require.NotEmpty(t, got)

	// The returned slice must be a copy: mutating it must not affect
	// a subsequent call.
	got[0] = "MUTATED"
	got2 := timez.NamedLayouts()
	require.NotEqual(t, "MUTATED", got2[0])
}

func TestTimestampLayouts(t *testing.T) {
	require.NotEmpty(t, timez.TimestampLayouts)

	// Every registered layout must be a real time layout, i.e. formatting
	// a known instant must substitute reference-time tokens. A corrupted
	// layout with no recognizable tokens (e.g. "garbage") formats to
	// itself verbatim, which this catches.
	tm := time.Date(2021, 1, 16, 18, 26, 39, 0, time.UTC)
	for name, layout := range timez.TimestampLayouts {
		require.NotEmpty(t, layout, "layout %q must be non-empty", name)
		got := tm.Format(layout)
		require.NotEqual(t, layout, got,
			"layout %q produced no substitution, likely not a valid layout", name)
	}
}

func TestFormatFunc(t *testing.T) {
	// Fixed instant: 2020-11-12 13:14:15.012345678 MST (-0700).
	tm := nov12denver

	t.Run("unix", func(t *testing.T) {
		require.Equal(t, strconv.FormatInt(tm.Unix(), 10), timez.FormatFunc("Unix")(tm))
		require.Equal(t, strconv.FormatInt(tm.UnixMilli(), 10), timez.FormatFunc("UnixMilli")(tm))
		require.Equal(t, strconv.FormatInt(tm.UnixMicro(), 10), timez.FormatFunc("UnixMicro")(tm))
		require.Equal(t, strconv.FormatInt(tm.UnixNano(), 10), timez.FormatFunc("UnixNano")(tm))
	})

	t.Run("case_insensitive", func(t *testing.T) {
		// Named layouts are matched case-insensitively.
		require.Equal(t, timez.FormatFunc("Unix")(tm), timez.FormatFunc("unix")(tm))
		require.Equal(t, timez.FormatFunc("Unix")(tm), timez.FormatFunc("UNIX")(tm))
		require.Equal(t, timez.FormatFunc("RFC3339")(tm), timez.FormatFunc("rfc3339")(tm))
	})

	t.Run("named_stdlib", func(t *testing.T) {
		// Each non-unix named layout must equal direct stdlib formatting.
		cases := map[string]string{
			"ANSIC":       time.ANSIC,
			"DateOnly":    time.DateOnly,
			"DateTime":    time.DateTime,
			"ISO8601":     timez.ISO8601,
			"ISO8601Z":    timez.ISO8601Z,
			"RFC1123":     time.RFC1123,
			"RFC1123Z":    time.RFC1123Z,
			"RFC3339":     time.RFC3339,
			"RFC3339Nano": time.RFC3339Nano,
			"RFC3339Z":    timez.RFC3339Z,
			"RFC822":      time.RFC822,
			"RFC822Z":     time.RFC822Z,
			"RFC850":      time.RFC850,
			"TimeOnly":    time.TimeOnly,
			"UnixDate":    time.UnixDate,
		}
		for name, layout := range cases {
			require.Equal(t, tm.Format(layout), timez.FormatFunc(name)(tm),
				"named layout %q", name)
		}
	})

	t.Run("strftime", func(t *testing.T) {
		// An unrecognized layout is treated as strftime, not stdlib.
		require.Equal(t, "2020/11/12", timez.FormatFunc("%Y/%m/%d")(tm))
		require.Equal(t, strconv.FormatInt(tm.Unix(), 10), timez.FormatFunc("%s")(tm))
	})

	t.Run("all_named", func(t *testing.T) {
		// Every advertised named layout must produce non-empty output.
		for _, layout := range timez.NamedLayouts() {
			require.NotEmpty(t, timez.FormatFunc(layout)(tm), "layout %q", layout)
		}
	})
}

func TestExcelLongDate(t *testing.T) {
	s := mar1UTC.Format(timez.ExcelLongDate)
	require.Equal(t, "Wednesday, March 1, 2023", s)
}
