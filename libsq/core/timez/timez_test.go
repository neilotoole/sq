package timez_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ncruces/go-strftime"

	"github.com/neilotoole/sq/libsq/core/timez"
	"github.com/neilotoole/sq/testh/tutil"
	"github.com/stretchr/testify/require"
)

var (
	mar1, _    = timez.ParseDateUTC("2023-03-01")
	mar1denver time.Time
	denver     *time.Location
)

func init() { //nolint:gochecknoinits
	var err error
	if denver, err = time.LoadLocation("America/Denver"); err != nil {
		panic(err)
	}
	mar1denver = mar1.In(denver)
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

func TestTimeTest(t *testing.T) {
	// FIXME: delete TestTimeTest
	stdlib := mar1denver.Format(time.RFC3339)
	moi := mar1denver.Format(timez.RFC3339Variant)
	iso8601 := mar1denver.Format(timez.ISO8601)
	_ = moi
	// assert.Equal(t, stdlib, moi)
	assert.Equal(t, stdlib, iso8601)
}

func TestNamedLayout(t *testing.T) {
	epoch := "%s"

	s := strftime.Format(epoch, time.Now())
	t.Log(s)

	unix, err := strftime.Layout("%s")
	require.NoError(t, err)
	t.Log(unix)
}
