// Package excelw implements output writers for Microsoft Excel.
// It uses the https://github.com/qax-os/excelize library.
// See docs: https://xuri.me/excelize
package excelw

import (
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/samber/lo"

	"github.com/neilotoole/sq/libsq/core/kind"

	"github.com/xuri/excelize/v2"

	"github.com/neilotoole/sq/libsq/core/record"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
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
func (w *recordWriter) initStyles() error {
	var err error

	w.headerStyle, err = w.xfile.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return errw(err)
	}

	w.datetimeStyle, err = w.xfile.NewStyle(&excelize.Style{
		// NumFmt:       22, // 22: "m/d/yy hh:mm",
		CustomNumFmt: lo.ToPtr("yyyy-mm-dd hh:mm"),
	})
	if err != nil {
		return errw(err)
	}

	w.dateStyle, err = w.xfile.NewStyle(&excelize.Style{
		// NumFmt:       14, // 	14: "mm-dd-yy",
		CustomNumFmt: lo.ToPtr("yyyy-mm-dd"),
	})
	if err != nil {
		return errw(err)
	}

	w.timeStyle, err = w.xfile.NewStyle(&excelize.Style{
		NumFmt:       21, // 	21: "hh:mm:ss",
		CustomNumFmt: nil,
	})

	return errw(err)
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
		switch field.Kind() {
		case kind.Datetime:
			// TODO: These widths could be configurable?
			wantWidth = 20
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
func (w *recordWriter) WriteRecords(recs []record.Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	//
	for i, rec := range recs {
		rowi := i + w.nextRow

		for j, val := range rec {
			cellIndex := cellName(j, rowi)

			switch val := val.(type) {
			case nil:
				// Do nothing for nil
			case []byte:
				if err := w.xfile.SetCellValue(SheetName, cellIndex, val); err != nil {
					return errw(err)
				}
			case string:
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
