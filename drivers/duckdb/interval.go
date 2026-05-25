package duckdb

import (
	"strconv"
	"strings"

	duckdbdriver "github.com/duckdb/duckdb-go/v2"
)

// FormatInterval renders a DuckDB INTERVAL value in DuckDB's native
// canonical text form, e.g. "1 year 2 months 3 days 04:05:06.789",
// "-03:00:00", or "00:00:00". The output is ASCII-only and round-trippable:
// DuckDB re-parses this form back to the identical interval value.
//
// DuckDB stores an interval as three independent signed fields — months,
// days, and microseconds — that never normalize into one another (e.g.
// 24 hours stays in micros and renders as "24:00:00", not "1 day"). Years
// are a display-only split of the months field, and each section carries
// its own sign.
func FormatInterval(iv duckdbdriver.Interval) string {
	var parts []string

	years := iv.Months / 12
	months := iv.Months % 12
	if years != 0 {
		parts = append(parts, pluralUnit(int64(years), "year"))
	}
	if months != 0 {
		parts = append(parts, pluralUnit(int64(months), "month"))
	}
	if iv.Days != 0 {
		parts = append(parts, pluralUnit(int64(iv.Days), "day"))
	}

	// The time section is shown when there are micros, or when the whole
	// interval is zero (canonical zero renders as "00:00:00").
	if iv.Micros != 0 || len(parts) == 0 {
		parts = append(parts, formatIntervalTime(iv.Micros))
	}

	return strings.Join(parts, " ")
}

// pluralUnit formats a signed count with its unit, pluralizing when the
// magnitude is not 1, e.g. pluralUnit(1, "year") == "1 year",
// pluralUnit(-2, "day") == "-2 days".
func pluralUnit(n int64, unit string) string {
	s := strconv.FormatInt(n, 10) + " " + unit
	if n != 1 && n != -1 {
		s += "s"
	}
	return s
}

// formatIntervalTime renders a microsecond count as "[-]HH:MM:SS[.ffffff]".
// HH is the total number of hours and may exceed 23 (e.g. "277:46:40"); it
// is zero-padded to a minimum of two digits. The fractional part is the
// microsecond remainder written with up to six digits and trailing zeros
// trimmed, and is omitted entirely when zero.
func formatIntervalTime(micros int64) string {
	sign := ""
	if micros < 0 {
		sign = "-"
		micros = -micros
	}

	const usPerSec = 1_000_000
	secs := micros / usPerSec
	frac := micros % usPerSec

	hours := secs / 3600
	mins := (secs % 3600) / 60
	s := secs % 60

	var b strings.Builder
	b.WriteString(sign)

	// Hours: minimum two digits, unbounded above.
	hs := strconv.FormatInt(hours, 10)
	if len(hs) < 2 {
		b.WriteByte('0')
	}
	b.WriteString(hs)
	b.WriteByte(':')
	writePad2(&b, mins)
	b.WriteByte(':')
	writePad2(&b, s)

	if frac != 0 {
		fs := strconv.FormatInt(frac, 10)
		fs = strings.Repeat("0", 6-len(fs)) + fs // left-pad to 6 digits
		fs = strings.TrimRight(fs, "0")          // trim trailing zeros
		b.WriteByte('.')
		b.WriteString(fs)
	}

	return b.String()
}

// writePad2 writes n to b, zero-padded to a minimum of two digits.
func writePad2(b *strings.Builder, n int64) {
	if n < 10 {
		b.WriteByte('0')
	}
	b.WriteString(strconv.FormatInt(n, 10))
}
