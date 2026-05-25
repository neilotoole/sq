package duckdb_test

import (
	"database/sql"
	"math"
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
		{0, 0, -1500000, "-00:00:01.5"},
		{-1, 0, 3600000000, "-1 month 01:00:00"},
	}

	for _, tc := range testCases {
		t.Run(tc.want, func(t *testing.T) {
			iv := duckdbdriver.Interval{Months: tc.months, Days: tc.days, Micros: tc.micros}
			require.Equal(t, tc.want, duckdb.FormatInterval(iv))
		})
	}
}

// TestFormatInterval_OracleCrossCheck asserts FormatInterval byte-matches
// DuckDB's own ::VARCHAR rendering across a matrix of interval expressions,
// including negative and mixed-sign values. DuckDB is the spec.
func TestFormatInterval_OracleCrossCheck(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()

	exprs := []string{
		"INTERVAL '1 year 2 months 3 days 4 hours 5 minutes 6.789 seconds'",
		"INTERVAL '25 months'",
		"INTERVAL '-25 months'",
		"INTERVAL '12 months'",
		"INTERVAL '-1 month'",
		"INTERVAL '1 month -2 days'",
		"INTERVAL '14 days'",
		"INTERVAL '1 day 12 hours'",
		"INTERVAL '-1 day -12 hours'",
		"INTERVAL '1 month -3 hours 30 minutes'",
		"INTERVAL '-3 hours'",
		"INTERVAL '24 hours'",
		"INTERVAL '1000000 seconds'",
		"INTERVAL '0.000003 seconds'",
		"INTERVAL '0 seconds'",
	}

	for _, expr := range exprs {
		t.Run(expr, func(t *testing.T) {
			var iv duckdbdriver.Interval
			require.NoError(t, db.QueryRow("SELECT "+expr).Scan(&iv))

			var want string
			require.NoError(t, db.QueryRow("SELECT ("+expr+")::VARCHAR").Scan(&want))

			require.Equal(t, want, duckdb.FormatInterval(iv),
				"expr=%s struct=%+v", expr, iv)
		})
	}
}

// TestFormatInterval_MinInt64 guards against int64 negation overflow: the
// magnitude of math.MinInt64 micros must not wrap to a bogus positive value.
// The formatter clamps math.MinInt64 to math.MaxInt64 (1µs short of the true
// magnitude, a value no real DuckDB interval reaches), so the output is exact
// and deterministic.
func TestFormatInterval_MinInt64(t *testing.T) {
	var got string
	require.NotPanics(t, func() {
		got = duckdb.FormatInterval(duckdbdriver.Interval{Micros: math.MinInt64})
	})
	require.Equal(t, "-2562047788:00:54.775807", got)
}

// TestFormatInterval_RoundTrip asserts that a rendered interval string can
// be re-ingested by DuckDB to produce the identical interval value.
func TestFormatInterval_RoundTrip(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()

	exprs := []string{
		"INTERVAL '1 year 2 months 3 days 4 hours 5 minutes 6.789 seconds'",
		"INTERVAL '-25 months'",
		"INTERVAL '1 month -2 days'",
		"INTERVAL '-1 day -12 hours'",
		"INTERVAL '1 month -3 hours 30 minutes'",
		"INTERVAL '1000000 seconds'",
		"INTERVAL '0.000003 seconds'",
		"INTERVAL '0 seconds'",
	}

	for _, expr := range exprs {
		t.Run(expr, func(t *testing.T) {
			var orig duckdbdriver.Interval
			require.NoError(t, db.QueryRow("SELECT "+expr).Scan(&orig))

			rendered := duckdb.FormatInterval(orig)

			var got duckdbdriver.Interval
			require.NoError(t,
				db.QueryRow("SELECT CAST(? AS INTERVAL)", rendered).Scan(&got),
				"rendered=%q did not re-parse", rendered)

			require.Equal(t, orig, got, "round-trip mismatch for rendered=%q", rendered)
		})
	}
}
