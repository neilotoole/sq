package xlsx

import (
	"io"
	"os"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq/lib/common"
	"github.com/neilotoole/sq/lib/drvr"
	"github.com/neilotoole/sq/lib/util"
	"github.com/tealeg/xlsx"
)

var w io.Writer = os.Stdout

type XLSXWriter struct {
	header      bool
	needsHeader bool
	xfile       *xlsx.File
	sheet       *xlsx.Sheet
}

func NewWriter(header bool) *XLSXWriter {

	return &XLSXWriter{header: header, needsHeader: header}
}

func (w *XLSXWriter) Metadata(meta *drvr.SourceMetadata) error {
	return util.Errorf("not implemented")
}

func (w *XLSXWriter) Open() error {
	lg.Debugf("Open()")

	w.xfile = xlsx.NewFile()

	sheet, err := w.xfile.AddSheet("Sheet1")
	if err != nil {
		return util.Errorf("unable to create XLSX sheet: %v", err)
	}

	w.sheet = sheet

	return nil
}
func (w *XLSXWriter) Close() error {

	lg.Debugf("Close()")

	if w.xfile == nil {
		return util.Errorf("unable to write nil XLSX: must be opened first")
	}

	err := w.xfile.Write(os.Stdout)
	if err != nil {
		return util.Errorf("unable to write XLSX: %v", err)
	}
	return nil
}

func (w *XLSXWriter) ResultRows(rows []*common.ResultRow) error {

	lg.Debugf("ResultRows()")

	if w.xfile == nil || w.sheet == nil {
		return util.Errorf("unable to write nil XLSX file: must be opened first")
	}

	if w.header && len(w.sheet.Rows) == 0 {

	}

	for _, row := range rows {

		if w.needsHeader {

			headerRow := w.sheet.AddRow()

			for _, colType := range row.Fields {
				cell := headerRow.AddCell()
				cell.SetString(colType.Name)
			}

			w.needsHeader = false
		}

		xrow := w.sheet.AddRow()

		for _, val := range row.Values {

			cell := xrow.AddCell()

			lg.Debugf("have val with type: %T: %v", val, val)

			switch val := val.(type) {
			case nil:
			case *[]byte:
				cell.SetValue(*val)
			case *sql.NullString:
				if val.Valid {
					cell.SetString(val.String)
				}
			case *sql.NullBool:

				if val.Valid {
					cell.SetBool(val.Bool)
				}

			case *sql.NullInt64:

				if val.Valid {
					cell.SetInt64(val.Int64)

				}
			case *sql.NullFloat64:

				if val.Valid {
					cell.SetFloat(val.Float64)
				}
				// TODO: support datetime

			default:
				cell.SetValue(val)
				lg.Debugf("unexpected column value type, treating as default: %T(%v)", val, val)
			}

		}

	}

	return nil
}

//for _, row := range rows {
//
//	for _, val := range row.Values {
//		switch val := val.(type) {
//		case nil:
//		case *[]byte:
//			w.Write(*val)
//		case *sql.NullString:
//			if val.Valid {
//				fmt.Fprintf(w, val.String)
//			}
//		case *sql.NullBool:
//
//			if val.Valid {
//				fmt.Fprintf(w, "%t", val.Bool)
//			}
//
//		case *sql.NullInt64:
//
//			if val.Valid {
//				fmt.Fprintf(w, "%d", val.Int64)
//			}
//		case *sql.NullFloat64:
//
//			if val.Valid {
//				fmt.Fprintf(w, "%f", val.Float64)
//			}
//
//		default:
//			lg.Debugf("unexpected column value type, treating as default: %T(%v)", val, val)
//			fmt.Fprintf(w, "%v", val)
//		}
//		fmt.Fprintln(w) // Add the new line
//	}
//
//}
