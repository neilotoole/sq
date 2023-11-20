// Package xlsxw implements output writers for Microsoft Excel.
// It uses the https://github.com/qax-os/excelize library.
// See docs: https://xuri.me/excelize
package xlsxw

import (
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	excelize "github.com/xuri/excelize/v2"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
)

type recordWriter struct {
	recMeta       record.Meta
	mu            sync.Mutex
	pr            *output.Printing
	out           io.Writer
	header        bool
	xfile         *excelize.File
	nextRow       int
	timeStyle     int
	dateStyle     int
	datetimeStyle int
	headerStyle   int
}

var _ output.NewRecordWriterFunc = NewRecordWriter

// NewRecordWriter returns an output.RecordWriter instance for XLSX.
func NewRecordWriter(out io.Writer, pr *output.Printing) output.RecordWriter {
	return &recordWriter{out: out, pr: pr, header: pr.ShowHeader}
}

// initStyles sets up the datetime styles. See:
//
// - https://xuri.me/excelize/en/cell.html#SetCellStyle
// - https://exceljet.net/articles/custom-number-formats
// - https://support.microsoft.com/en-gb/office/format-numbers-as-dates-or-times-418bd3fe-0577-47c8-8caa-b4d30c528309#bm2
//
//nolint:lll
func (w *recordWriter) initStyles() error {
	var err error

	if w.headerStyle, err = w.xfile.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	}); err != nil {
		return errw(err)
	}

	if w.pr.ExcelDatetimeFormat != "" {
		if w.datetimeStyle, err = w.xfile.NewStyle(&excelize.Style{
			CustomNumFmt: &w.pr.ExcelDatetimeFormat,
		}); err != nil {
			return errz.Wrap(err, "excel: failed to set excel datetime style")
		}
	}

	if w.pr.ExcelDateFormat != "" {
		if w.dateStyle, err = w.xfile.NewStyle(&excelize.Style{
			CustomNumFmt: &w.pr.ExcelDateFormat,
		}); err != nil {
			return errz.Wrap(err, "excel: failed to set excel date style")
		}
	}

	if w.pr.ExcelTimeFormat != "" {
		if w.timeStyle, err = w.xfile.NewStyle(&excelize.Style{
			CustomNumFmt: &w.pr.ExcelTimeFormat,
		}); err != nil {
			return errz.Wrap(err, "excel: failed to set excel time style")
		}
	}

	return nil
}

// Open implements output.RecordWriter.
func (w *recordWriter) Open(recMeta record.Meta) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.recMeta = recMeta
	var err error
	if w.xfile, err = NewFile(); err != nil {
		return err
	}

	if err = w.initStyles(); err != nil {
		return err
	}

	if w.header {
		w.nextRow++
		for i, colName := range w.recMeta.MungedNames() {
			cell := cellName(i, 0)
			if err := w.xfile.SetCellStr(SheetName, cell, colName); err != nil {
				return errw(err)
			}

			if err := w.xfile.SetCellStyle(SheetName, cell, cell, w.headerStyle); err != nil {
				return errw(err)
			}
		}
	}

	for i, field := range recMeta {
		wantWidth := -1
		switch field.Kind() { //nolint:exhaustive
		case kind.Datetime:
			// TODO: These widths could be configurable?
			wantWidth = 20
		case kind.Date:
			wantWidth = 12
		case kind.Time:
			wantWidth = 16
		case kind.Text:
			wantWidth = 32
		default:
		}

		if wantWidth != -1 {
			if err := w.setColWidth(i, wantWidth); err != nil {
				return err
			}
		}
	}

	return nil
}

// setColWidth takes the zero-indexed col, and sets its width.
func (w *recordWriter) setColWidth(col, width int) error {
	colName := string(rune('A' + col))
	err := w.xfile.SetColWidth(SheetName, colName, colName, float64(width))
	return errw(err)
}

// Flush implements output.RecordWriter.
func (w *recordWriter) Flush() error {
	return nil
}

// Close implements output.RecordWriter.
func (w *recordWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	err := w.xfile.Write(w.out)
	if err != nil {
		return errz.Wrap(err, "excel: unable to write XLSX")
	}
	return nil
}

// WriteRecords implements output.RecordWriter.
func (w *recordWriter) WriteRecords(recs []record.Record) error { //nolint:gocognit
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, rec := range recs {
		rowi := w.nextRow

		for j, val := range rec {
			cellIndex := cellName(j, rowi)

			switch val := val.(type) {
			case nil:
				// Do nothing for nil
			case []byte:
				if len(val) != 0 {
					b64 := base64.StdEncoding.EncodeToString(val)
					if err := w.xfile.SetCellValue(SheetName, cellIndex, b64); err != nil {
						return errw(err)
					}
				}

			case string:
				// It seems that kind.Time values are supplied as string (at least
				// by some backend database drivers). However, Excel won't honor the
				// time format style unless the cell value is set as a float.
				if w.recMeta[j].Kind() == kind.Time {
					if timeFloat, err := timeOnlyStringToExcelFloat(val); err == nil {
						if err = w.xfile.SetCellStyle(SheetName, cellIndex, cellIndex, w.timeStyle); err != nil {
							return errw(err)
						}

						if err = w.xfile.SetCellValue(SheetName, cellIndex, timeFloat); err != nil {
							return errw(err)
						}

						break
					}

					// If there's an error, just continue below, using a plain ol' string.
				}

				if err := w.xfile.SetCellStr(SheetName, cellIndex, val); err != nil {
					return errw(err)
				}
			case bool:
				if err := w.xfile.SetCellBool(SheetName, cellIndex, val); err != nil {
					return errw(err)
				}
			case int64:
				if err := w.xfile.SetCellInt(SheetName, cellIndex, int(val)); err != nil {
					return errw(err)
				}
			case float64:
				if err := w.xfile.SetCellFloat(SheetName, cellIndex, val, -1, 64); err != nil {
					return errw(err)
				}
			case time.Time:
				switch w.recMeta[j].Kind() { //nolint:exhaustive
				default:
					// Shouldn't happen
					if err := w.xfile.SetCellValue(SheetName, cellIndex, val); err != nil {
						return errw(err)
					}

				case kind.Datetime:
					if err := w.xfile.SetCellStyle(SheetName, cellIndex, cellIndex, w.datetimeStyle); err != nil {
						return errw(err)
					}

					if err := w.xfile.SetCellValue(SheetName, cellIndex, val); err != nil {
						return errw(err)
					}
				case kind.Date:
					if err := w.xfile.SetCellStyle(SheetName, cellIndex, cellIndex, w.dateStyle); err != nil {
						return errw(err)
					}

					if err := w.xfile.SetCellValue(SheetName, cellIndex, val); err != nil {
						return errw(err)
					}

				case kind.Time:
					if err := w.xfile.SetCellStyle(SheetName, cellIndex, cellIndex, w.timeStyle); err != nil {
						return errw(err)
					}

					// Excel prefers that time-only values be represented as float, so
					// we try that first.
					if timeFloat, err := timeOnlyToExcelFloat(val); err == nil {
						if err = w.xfile.SetCellValue(SheetName, cellIndex, timeFloat); err != nil {
							return errw(err)
						}

						// Success, we can break out of the switch.
						break
					}

					// No success with the float approach. Just default to setting
					// the time.Time value, and let Excel figure it out.
					if err := w.xfile.SetCellValue(SheetName, cellIndex, val); err != nil {
						return errw(err)
					}
				}
			default:
				// should never happen
				s := fmt.Sprintf("%v", val)
				if err := w.xfile.SetCellStr(SheetName, cellIndex, s); err != nil {
					return errw(err)
				}
			}
		}

		w.nextRow++
	}

	return nil
}

const SheetName = "data"

// NewFile returns a new file with a single, empty sheet named "data".
func NewFile() (*excelize.File, error) {
	f := excelize.NewFile()
	if err := f.SetSheetName("Sheet1", SheetName); err != nil {
		_ = f.Close()
		return nil, errw(err)
	}

	return f, nil
}

func errw(err error) error {
	return errz.Wrap(err, "excel")
}

// cellName accepts zero-index cell coordinates, and returns the call name.
// For example, {0,0} returns "A1".
func cellName(col, row int) string {
	s, _ := excelize.ColumnNumberToName(col + 1)
	s += strconv.Itoa(row + 1)
	return s
}
