package duckdb_test

import (
	"math"
	"strings"
	"testing"

	duckdbdriver "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/duckdb"
)

func TestFormatInterval(t *testing.T) {
	testCases := []struct {
		months int32
		days   int32
		micros int64
		want   string
	}{
		{14, 3, 14706789000, "1 year 2 months 3 days 04:05:06.789"},
		{25, 0, 0, "2 years 1 month"},
		{-25, 0, 0, "-2 years -1 month"},
		{12, 0, 0, "1 year"},
		{-1, 0, 0, "-1 month"},
		{1, -2, 0, "1 month -2 days"},
		{0, 14, 0, "14 days"},
		{0, 1, 43200000000, "1 day 12:00:00"},
		{0, -1, -43200000000, "-1 day -12:00:00"},
		{1, 0, -9000000000, "1 month -02:30:00"},
		{0, 0, -10800000000, "-03:00:00"},
		{0, 0, 86400000000, "24:00:00"},
		{0, 0, 1000000000000, "277:46:40"},
		{0, 0, 3, "00:00:00.000003"},
		{0, 0, 0, "00:00:00"},
	}

	for _, tc := range testCases {
		t.Run(tc.want, func(t *testing.T) {
			iv := duckdbdriver.Interval{Months: tc.months, Days: tc.days, Micros: tc.micros}
			require.Equal(t, tc.want, duckdb.FormatInterval(iv))
		})
	}
}

// TestFormatInterval_MinInt64 guards against int64 negation overflow: the
// magnitude of math.MinInt64 micros must not wrap to a bogus positive value.
func TestFormatInterval_MinInt64(t *testing.T) {
	var got string
	require.NotPanics(t, func() {
		got = duckdb.FormatInterval(duckdbdriver.Interval{Micros: math.MinInt64})
	})
	require.True(t, strings.HasPrefix(got, "-"), "want negative sign, got %q", got)
	require.Equal(t, 2, strings.Count(got, ":"), "want HH:MM:SS shape, got %q", got)
}
