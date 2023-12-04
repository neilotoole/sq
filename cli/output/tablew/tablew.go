// Package tablew implements text table output writers.
//
// The actual rendering of the text table is handled by a heavily modified
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
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/shopspring/decimal"

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

// renderResultCell renders a record value to a string.
// Arg val is guaranteed to be one of the types
// constrained by record.Valid.
func (t *table) renderResultCell(knd kind.Kind, val any) string {
	switch val := val.(type) {
	case nil:
		return t.sprintNull()
	case string:

		// Although val is string, we allow for the case where
		// the kind is not kind.Text: for example, sqlite returns
		// values of kind.Time as a string.

		switch knd { //nolint:exhaustive // ignore kind.Unknown and kind.Null
		case kind.Datetime, kind.Date, kind.Time:
			return t.pr.Datetime.Sprint(val)
		case kind.Float, kind.Int:
			return t.pr.Number.Sprint(val)
		case kind.Decimal:
			d, err := decimal.NewFromString(val)
			if err != nil {
				// Shouldn't happen
				return t.pr.Number.Sprint(val)
			}
			return t.pr.Number.Sprint(stringz.FormatDecimal(d))
		case kind.Bool:
			return t.pr.Bool.Sprint(val)
		case kind.Bytes:
			return t.sprintBytes([]byte(val))
		case kind.Text:
			return t.pr.String.Sprint(val)
		default:
			// Shouldn't happen
			return val
		}
	case float64:
		return t.sprintFloat64(val)
	case int64:
		return t.sprintInt64(val)
	case bool:
		return t.sprintBool(val)
	case time.Time:
		if val.IsZero() {
			return t.sprintNull()
		}

		var s string
		switch knd { //nolint:exhaustive
		default:
			s = t.pr.FormatDatetime(val)
		case kind.Time:
			s = t.pr.FormatTime(val)
		case kind.Date:
			s = t.pr.FormatDate(val)
		}

		return t.pr.Datetime.Sprint(s)
	case []byte:
		return t.sprintBytes(val)
	case decimal.Decimal:
		return t.pr.Number.Sprint(stringz.FormatDecimal(val))
	}

	// TODO: this should really return an error, or at least log it?
	return t.pr.Error.Sprintf("%v", val)
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

func (t *table) sprintBool(b bool) string {
	return t.pr.Bool.Sprint(strconv.FormatBool(b))
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

func (t *table) appendRowsAndRenderAll(ctx context.Context, rows [][]string) error {
	for _, v := range rows {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		t.tblImpl.Append(v)
	}
	return t.tblImpl.RenderAll(ctx)
}

func (t *table) appendRows(ctx context.Context, rows [][]string) error {
	for _, v := range rows {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		t.tblImpl.Append(v)
	}
	return nil
}

func (t *table) renderAll(ctx context.Context) error {
	return t.tblImpl.RenderAll(ctx)
}

func (t *table) renderRow(ctx context.Context, row []string) error {
	t.tblImpl.Append(row)
	return t.tblImpl.RenderAll(ctx) // Send output
}

func getColorForVal(pr *output.Printing, v any) *color.Color {
	switch v.(type) {
	case nil:
		return pr.Null
	case int, int64, float32, float64, uint, uint64, decimal.Decimal:
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
