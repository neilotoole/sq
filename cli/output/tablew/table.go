package tablew

import (
	"database/sql"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/tablew/internal"
	"github.com/neilotoole/sq/libsq/sqlz"
	"github.com/neilotoole/sq/libsq/stringz"
)

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
		return *val
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
		return t.sprintTime(val)
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
	s := fmt.Sprintf("[%d]", len(b))
	return t.fm.Bytes.Sprint(s)
}

func (t *table) sprintNull() string {
	return t.fm.Null.Sprint("NULL")
}
func (t *table) sprintTime(tm *time.Time) string {
	s := fmt.Sprintf("%v", *tm)
	return t.fm.Datetime.Sprint(s)
}

func (t *table) sprintInt64(num int64) string {
	s := fmt.Sprintf("%d", num)
	return t.fm.Number.Sprint(s)
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
