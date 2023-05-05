// Package tablew implements text table output writers.
//
// The actual rendering of the text table is handled by a modified
// version ofolekukonko/tablewriter which can be found in the internal
// sub-package. At the time, tablewriter didn't provide all the
// functionality that sq required. However, that package has been
// significantly developed since then fork, and it may be possible
// that we could dispense with the forked version entirely and directly
// use a newer version of tablewriter.
//
// This entire package could use a rewrite, a lot has changed with sq
// since this package was first created. So, if you see code in here
// that doesn't make sense to you, you're probably judging it correctly.
package tablew

import (
	"database/sql"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/neilotoole/sq/libsq/core/timez"

	"github.com/fatih/color"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/tablew/internal"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// table encapsulates our table implementation.
type table struct {
	pr     *output.Printing
	out    io.Writer
	header bool

	tblImpl *internal.Table
}

func (t *table) renderResultCell(knd kind.Kind, val any) string { //nolint:funlen,cyclop,gocognit,gocyclo
	switch val := val.(type) {
	case string:
		return val
	case *sql.NullString:
		if !val.Valid {
			return t.sprintNull()
		}
		return t.pr.String.Sprint(val.String)
	case *string:
		if val == nil {
			return t.sprintNull()
		}

		// Although val is string, we allow for the case where
		// the kind is not kind.Text: for example, sqlite returns
		// values of kind.Time as a string.

		switch knd { //nolint:exhaustive // ignore kind.Unknown and kind.Null
		case kind.Datetime, kind.Date, kind.Time:
			return t.pr.Datetime.Sprint(*val)
		case kind.Decimal, kind.Float, kind.Int:
			return t.pr.Number.Sprint(*val)
		case kind.Bool:
			return t.pr.Bool.Sprint(*val)
		case kind.Bytes:
			return t.sprintBytes([]byte(*val))
		case kind.Text:
			return t.pr.String.Sprint(*val)
		default:
			// Shouldn't happen
			return *val
		}

	case float64:
		return t.sprintFloat64(val)
	case *float64:
		if val == nil {
			return t.sprintNull()
		}
		return t.sprintFloat64(*val)
	case *sql.NullFloat64:
		if !val.Valid {
			return t.sprintNull()
		}
		return t.sprintFloat64(val.Float64)
	case float32:
		return t.sprintFloat64(float64(val))
	case *float32:
		if val == nil {
			return t.sprintNull()
		}
		return t.sprintFloat64(float64(*val))
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		s := fmt.Sprintf("%d", val)
		return t.pr.Number.Sprint(s)
	case *int:
		if val == nil {
			return t.sprintNull()
		}
		return t.sprintInt64(int64(*val))
	case *int8:
		if val == nil {
			return t.sprintNull()
		}
		return t.sprintInt64(int64(*val))
	case *int16:
		if val == nil {
			return t.sprintNull()
		}
		return t.sprintInt64(int64(*val))
	case *int32:
		if val == nil {
			return t.sprintNull()
		}
		return t.sprintInt64(int64(*val))
	case *int64:
		if val == nil {
			return t.sprintNull()
		}
		return t.sprintInt64(*val)
	case *uint:
		if val == nil {
			return t.sprintNull()
		}
		return t.sprintInt64(int64(*val))
	case *uint8:
		if val == nil {
			return t.sprintNull()
		}
		return t.sprintInt64(int64(*val))
	case *uint16:
		if val == nil {
			return t.sprintNull()
		}
		return t.sprintInt64(int64(*val))
	case *uint32:
		if val == nil {
			return t.sprintNull()
		}
		return t.sprintInt64(int64(*val))
	case *uint64:
		if val == nil {
			return t.sprintNull()
		}
		return t.sprintInt64(int64(*val))
	case *sql.NullInt64:
		if !val.Valid {
			return t.sprintNull()
		}
		return t.sprintInt64(val.Int64)
	case bool:
		return t.pr.Bool.Sprint(strconv.FormatBool(val))
	case *bool:
		if val == nil {
			return t.sprintNull()
		}
		return t.pr.Bool.Sprint(strconv.FormatBool(*val))
	case *time.Time:
		if val == nil {
			return t.sprintNull()
		}

		var s string
		switch knd { //nolint:exhaustive
		default:
			s = val.Format(timez.DatetimeFormat)
		case kind.Time:
			s = val.Format(timez.TimeFormat)
		case kind.Date:
			s = val.Format(timez.DateFormat)
		}

		return t.pr.Datetime.Sprint(s)

	case *sql.NullBool:
		if !val.Valid {
			return t.sprintNull()
		}
		return t.pr.Bool.Sprint(strconv.FormatBool(val.Bool))
	case nil:
		return t.sprintNull()
	case []byte:
		return t.sprintBytes(val)
	case *[]byte:
		if val == nil || *val == nil {
			return t.sprintNull()
		}
		return t.sprintBytes(*val)
	case *sql.RawBytes:
		if val == nil || *val == nil {
			return t.sprintNull()
		}
		if knd == kind.Text {
			return string(*val)
		}
		return t.sprintBytes(*val)
	}
	return ""
}

func (t *table) sprintBytes(b []byte) string {
	s := fmt.Sprintf("[%d bytes]", len(b))
	return t.pr.Bytes.Sprint(s)
}

func (t *table) sprintNull() string {
	return t.pr.Null.Sprint("NULL")
}

func (t *table) sprintInt64(num int64) string {
	return t.pr.Number.Sprint(strconv.FormatInt(num, 10))
}

func (t *table) sprintFloat64(num float64) string {
	return t.pr.Number.Sprint(stringz.FormatFloat(num))
}

// reset resets the table internals.
func (t *table) reset() {
	t.tblImpl = internal.NewTable(t.out)
	t.setTableWriterOptions()
	t.tblImpl.SetAutoFormatHeaders(false)
	t.tblImpl.SetAutoWrapText(false)
}

func (t *table) setTableWriterOptions() {
	t.tblImpl.SetAlignment(internal.AlignLeft)
	t.tblImpl.SetAutoWrapText(false)
	t.tblImpl.SetBorder(false)
	t.tblImpl.SetHeaderAlignment(internal.AlignLeft)
	t.tblImpl.SetCenterSeparator("")
	t.tblImpl.SetColumnSeparator("")
	t.tblImpl.SetRowSeparator("")
	t.tblImpl.SetBorders(internal.Border{Left: false, Top: false, Right: false, Bottom: false})
	t.tblImpl.SetAutoFormatHeaders(false)
	t.tblImpl.SetHeaderDisable(!t.header)
	t.tblImpl.SetHeaderTrans(t.pr.Header.SprintFunc())
}

func (t *table) appendRowsAndRenderAll(rows [][]string) {
	for _, v := range rows {
		t.tblImpl.Append(v)
	}
	t.tblImpl.RenderAll()
}

func (t *table) appendRows(rows [][]string) {
	for _, v := range rows {
		t.tblImpl.Append(v)
	}
}

func (t *table) renderAll() {
	t.tblImpl.RenderAll()
}

func (t *table) renderRow(row []string) {
	t.tblImpl.Append(row)
	t.tblImpl.RenderAll() // Send output
}

func getColorForVal(pr *output.Printing, v any) *color.Color {
	switch v.(type) {
	case nil:
		return pr.Null
	case int, int64, float32, float64, uint, uint64:
		return pr.Number
	case bool:
		return pr.Bool
	case string:
		return pr.String
	case time.Time:
		return pr.Datetime
	case time.Duration:
		return pr.Duration
	default:
		return pr.Normal
	}
}
