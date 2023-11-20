package xlsxw

// The code below is lifted from the excelize package.

import (
	"time"
)

const (
	nanosInADay    = float64((24 * time.Hour) / time.Nanosecond)
	dayNanoseconds = 24 * time.Hour
	maxDuration    = 290 * 364 * dayNanoseconds
	roundEpsilon   = 1e-9
)

var (
	daysInMonth           = []int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}    //nolint:unused
	excel1900Epoc         = time.Date(1899, time.December, 30, 0, 0, 0, 0, time.UTC) //nolint:unused
	excel1904Epoc         = time.Date(1904, time.January, 1, 0, 0, 0, 0, time.UTC)
	excelMinTime1900      = time.Date(1899, time.December, 31, 0, 0, 0, 0, time.UTC)
	excelBuggyPeriodStart = time.Date(1900, time.March, 1, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond)
)

// timeToExcelTime provides a function to convert time to Excel time.
func timeToExcelTime(t time.Time, date1904 bool) (float64, error) { //nolint:unparam
	date := excelMinTime1900
	if date1904 {
		date = excel1904Epoc
	}
	if t.Before(date) {
		return 0, nil
	}
	tt, diff, result := t, t.Sub(date), 0.0
	for diff >= maxDuration {
		result += float64(maxDuration / dayNanoseconds)
		tt = tt.Add(-maxDuration)
		diff = tt.Sub(date)
	}

	rem := diff % dayNanoseconds
	result += float64(diff-rem)/float64(dayNanoseconds) + float64(rem)/float64(dayNanoseconds)

	// Excel dates after 28th February 1900 are actually one day out.
	// Excel behaves as though the date 29th February 1900 existed, which it didn't.
	// Microsoft intentionally included this bug in Excel so that it would remain compatible with the spreadsheet
	// program that had the majority market share at the time; Lotus 1-2-3.
	// https://www.myonlinetraininghub.com/excel-date-and-time
	if !date1904 && t.After(excelBuggyPeriodStart) {
		result++
	}
	return result, nil
}
