package excelw

import (
	"math"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/xuri/excelize/v2"
)

var (
	// OptDatetimeFormat is Excel's custom datetime format string.
	OptDatetimeFormat = options.NewString(
		"format.excel.datetime",
		"",
		0,
		"yyyy-mm-dd hh:mm",
		func(s string) error {
			err := validateDatetimeFormatString(s)
			return errz.Wrap(err, "config: format.excel.datetime: invalid format string")
		},
		"Timestamp format string for Excel datetime values",
		`Timestamp format for datetime values: that is, for values that have
both a date and time component. The exact format is specific to
Microsoft Excel, but is broadly similar to strftime.

Examples:

  "yyyy-mm-dd hh:mm"           1989-11-09 16:07
  "dd/mm/yy h:mm am/pm"        09/11/89 4:07 pm
  "dd-mmm-yy h:mm:ss AM/PM"    09-Nov-89 4:07:01 PM
`,
		options.TagOutput,
	)

	// OptDateFormat is Excel's custom date-only format string.
	OptDateFormat = options.NewString(
		"format.excel.date",
		"",
		0,
		"yyyy-mm-dd",
		func(s string) error {
			err := validateDatetimeFormatString(s)
			return errz.Wrap(err, "config: format.excel.date: invalid format string")
		},
		"Date format string for Excel date-only values",
		`Date format string for Microsoft Excel date-only values. The exact format
is specific to Excel, but is broadly similar to strftime.

Examples:

  "yyyy-mm-dd"	  1989-11-09
  "dd/mm/yy"      09/11/89
  "dd-mmm-yy"     09-Nov-89`,
		options.TagOutput,
	)

	// OptTimeFormat is Excel's custom time format string.
	OptTimeFormat = options.NewString(
		"format.excel.time",
		"",
		0,
		"hh:mm:ss",
		func(s string) error {
			err := validateDatetimeFormatString(s)
			return errz.Wrap(err, "config: format.excel.time: invalid format string")
		},
		"Time format string for Excel time-only values",
		`Time format string for Microsoft Excel time-only values. The exact format is
specific to Excel, but is broadly similar to strftime.

Examples:

  "hh:mm:ss"         16:07:10
  "h:mm am/pm"	     4:07 pm
  "h:mm:ss AM/PM"    4:07:01 PM

Note that time-only values are sometimes programmatically indistinguishable
from datetime values. In that situation, use format.excel.datetime instead.`,
		options.TagOutput,
	)
)

func validateDatetimeFormatString(s string) error {
	xl := excelize.NewFile()
	defer func() { _ = xl.Close() }()

	_, err := xl.NewStyle(&excelize.Style{CustomNumFmt: &s})
	return errz.Err(err)
}

// timeOnlyToExcelFloat returns a float value for the time-only part
// of t. This is needed because Excel really prefers if time values
// are represented as a float.
//
// See: https://xuri.me/excelize/en/cell.html#SetCellStyle
func timeOnlyToExcelFloat(t time.Time) (float64, error) {
	now := time.Now().UTC()
	t2 := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		t.Hour(),
		t.Minute(),
		t.Second(),
		t.Nanosecond(),
		now.Location(),
	)

	f, err := timeToExcelTime(t2, false)
	if err != nil {
		return 0, errz.Err(err)
	}

	// We're only interested in the fractional part
	_, f = math.Modf(f)

	return f, nil
}

// timeOnlyStringToExcelFloat is a convenience wrapper
// around timeOnlyToExcelFloat, that handles a time-only string
// such as "16:07:04".
func timeOnlyStringToExcelFloat(s string) (float64, error) {
	t, err := time.Parse(time.TimeOnly, s)
	if err != nil {
		return 0, errz.Err(err)
	}

	return timeOnlyToExcelFloat(t)
}
