package kind

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/stretchr/testify/require"
)

func TestDetectKindDatetime(t *testing.T) {
	denver, err := time.LoadLocation("America/Denver")
	require.NoError(t, err)
	tm := time.Date(1989, 11, 9, 15, 17, 59, 123456700, denver)

	for _, f := range datetimeFormats {
		f := f
		t.Run(tutil.Name(f), func(t *testing.T) {
			s := tm.Format(f)

			ok, gotF := detectKindDatetime(s)
			assert.True(t, ok)

			t.Logf("%25s   %s   %s", f, s, gotF)
			_ = gotF
			assert.Equal(t, f, gotF)
		})
	}
}

func TestDetectKindDate(t *testing.T) {
	valid := []string{
		"2006-01-02",
		"02 Jan 2006",
		"2006-01-02",
		"01-02-06",
		"01-02-2006",
		"02-Jan-2006",
		"2-Jan-2006",
		"2-Jan-06",
		"Jan  2, 2006",
		"Jan 2, 2006",
		"January 2, 2006",
		"02/Jan/06",
		"2/Jan/06",
		"Monday, January 2, 2006",
		"Mon, January 2, 2006",
	}

	for i, input := range valid {
		input := input
		t.Run(tutil.Name("valid", i, input), func(t *testing.T) {
			t.Log(input)
			ok, gotF := detectKindDate(input)
			require.True(t, ok)

			tm, err := time.Parse(gotF, input)
			require.NoError(t, err)
			t.Log(tm)
		})
	}

	invalid := []string{
		"not a date",
		"",
		"2006",
		"January",
		"Monday",
	}

	for i, input := range invalid {
		input := input
		t.Run(tutil.Name("invalid", i, input), func(t *testing.T) {
			t.Log(input)
			ok, gotF := detectKindDate(input)
			require.False(t, ok)
			require.Empty(t, gotF)
		})
	}
}

func TestDetectKindTime(t *testing.T) {
	valid := []string{
		"3:17 PM",
		"3:17PM",
		"3:17 PM",
		"3:17pm",
		"15:17",
		"15:17:59",
		"3:17",
		"03:17",
	}

	for i, input := range valid {
		input := input
		t.Run(tutil.Name("valid", i, input), func(t *testing.T) {
			t.Log(input)
			ok, gotF := detectKindTime(input)
			require.True(t, ok)

			tm, err := time.Parse(gotF, input)
			require.NoError(t, err)
			t.Log(tm)
		})
	}

	invalid := []string{
		"not a time",
		"",
		"3",
		"15",
		"15 17",
		"02-Jan-2006",
	}

	for i, input := range invalid {
		input := input
		t.Run(tutil.Name("invalid", i, input), func(t *testing.T) {
			t.Log(input)
			ok, gotF := detectKindTime(input)
			require.False(t, ok)
			require.Empty(t, gotF)
		})
	}
}
