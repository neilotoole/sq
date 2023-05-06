package timez_test

import (
	"testing"
	"time"

	"github.com/neilotoole/sq/libsq/core/timez"
	"github.com/neilotoole/sq/testh/tutil"
	"github.com/stretchr/testify/require"
)

var (
	mar1UTC, _  = timez.ParseDateUTC("2023-03-01")
	denver      *time.Location
	nov12denver time.Time
	mar1denver  time.Time
)

func init() { //nolint:gochecknoinits
	var err error
	if denver, err = time.LoadLocation("America/Denver"); err != nil {
		panic(err)
	}
	nov12denver = time.Date(2020, 11, 12, 13, 14, 15, 12345678, denver)
	mar1denver = mar1UTC.In(denver)
}

func TestTimestampUTC(t *testing.T) {
	tm := time.Date(2021, 0o1, 0o1, 7, 7, 7, 0, time.UTC)
	s := timez.TimestampUTC(tm)
	t.Log(s)
	require.Equal(t, "2021-01-01T07:07:07.000Z", s)

	s = timez.TimestampUTC(time.Now().UTC())
	t.Log(s)
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
		tc := tc
		t.Run(tutil.Name(i, tc.in), func(t *testing.T) {
			tm, err := timez.ParseDateOrTimestampUTC(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			t.Logf("Got: %s", timez.TimestampUTC(tm))
			ut := tm.Unix()
			require.Equal(t, tc.want, ut)
		})
	}
}

func TestZuluTimestamp(t *testing.T) {
	const (
		input = `2022-12-30T09:36:31-0700`
		want  = `2022-12-30T16:36:31Z`
	)
	got := timez.TimestampToRFC3339(input)
	require.Equal(t, want, got)
}

func TestFormatFunc(t *testing.T) {
	layouts := timez.NamedLayouts()
	// Add some custom layouts
	layouts = append(layouts, "%Y/%m/%d", "%s")

	for _, layout := range layouts {
		fn := timez.FormatFunc(layout)
		got := fn(nov12denver)
		t.Logf("%16s: %s", layout, got)
	}
}
