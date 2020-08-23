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

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/tablew/internal"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// table encapsulates the
type table struct {
	fm     *output.Formatting
	out    io.Writer
	header bool

	tblImpl *internal.Table
}

func (t *table) renderResultCell(kind sqlz.Kind, val interface{}) string {
	switch val := val.(type) {
	case string:
		return val
	case *sql.NullString:
		if !val.Valid {
			return t.sprintNull()
		}
		return t.fm.String.Sprint(val.String)
	case *string:
		if val == nil {
			return t.sprintNull()
		}

		// Although val is string, we allow for the case where
		// the kind is not KindText: for example, sqlite returns
		// values of KindTime as a string.

		switch kind {
		case sqlz.KindDatetime, sqlz.KindDate, sqlz.KindTime:
			return t.fm.Datetime.Sprint(*val)
		case sqlz.KindDecimal, sqlz.KindFloat, sqlz.KindInt:
			return t.fm.Number.Sprint(*val)
		case sqlz.KindBool:
			return t.fm.Bool.Sprint(*val)
		case sqlz.KindBytes:
			return t.sprintBytes([]byte(*val))
		case sqlz.KindText:
			return t.fm.String.Sprint(*val)
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
		return t.fm.Number.Sprint(s)
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
		return t.fm.Bool.Sprint(strconv.FormatBool(val))
	case *bool:
		if val == nil {
			return t.sprintNull()
		}
		return t.fm.Bool.Sprint(strconv.FormatBool(*val))
	case *time.Time:
		if val == nil {
			return t.sprintNull()
		}

		var s string
		switch kind {
		default:
			s = val.Format(stringz.DatetimeFormat)
		case sqlz.KindTime:
			s = val.Format(stringz.TimeFormat)
		case sqlz.KindDate:
			s = val.Format(stringz.DateFormat)
		}

		return t.fm.Datetime.Sprint(s)

	case *sql.NullBool:
		if !val.Valid {
			return t.sprintNull()
		}
		return t.fm.Bool.Sprint(strconv.FormatBool(val.Bool))
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
		if kind == sqlz.KindText {
			return fmt.Sprintf("%s", *val)
		}
		return t.sprintBytes(*val)
	}
	return ""
}

func (t *table) sprintBytes(b []byte) string {
	s := fmt.Sprintf("[%d bytes]", len(b))
	return t.fm.Bytes.Sprint(s)
}

func (t *table) sprintNull() string {
	return t.fm.Null.Sprint("NULL")
}

func (t *table) sprintInt64(num int64) string {
	return t.fm.Number.Sprint(strconv.FormatInt(num, 10))
}
func (t *table) sprintFloat64(num float64) string {
	return t.fm.Number.Sprint(stringz.FormatFloat(num))
}

func (t *table) reset() {
	t.tblImpl = internal.NewTable(t.out)
	t.setTableWriterOptions()
	t.tblImpl.SetAutoFormatHeaders(false)
	t.tblImpl.SetAutoWrapText(false)
}

func (t *table) setTableWriterOptions() {
	t.tblImpl.SetAlignment(internal.AlignLeft)
	t.tblImpl.SetAutoWrapText(true)
	t.tblImpl.SetBorder(false)
	t.tblImpl.SetHeaderAlignment(internal.AlignLeft)
	t.tblImpl.SetCenterSeparator("")
	t.tblImpl.SetColumnSeparator("")
	t.tblImpl.SetRowSeparator("")
	t.tblImpl.SetBorders(internal.Border{Left: false, Top: false, Right: false, Bottom: false})
	t.tblImpl.SetAutoFormatHeaders(false)
	t.tblImpl.SetHeaderDisable(!t.header)
	t.tblImpl.SetHeaderTrans(t.fm.Header.SprintFunc())
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
