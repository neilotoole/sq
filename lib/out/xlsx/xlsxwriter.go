package xlsx

import (
	"io"
	"os"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/lib/common"
	"github.com/neilotoole/sq/lib/drvr"
	"github.com/neilotoole/sq/lib/util"
	"github.com/tealeg/xlsx"
)

var w io.Writer = os.Stdout

type XLSXWriter struct {
	header   bool
	xlsxFile *xlsx.File
}

func NewWriter(header bool) *XLSXWriter {

	return &XLSXWriter{header: header}
}

func (w *XLSXWriter) Metadata(meta *drvr.SourceMetadata) error {
	return util.Errorf("not implemented")
}

func (w *XLSXWriter) Open() error {
	lg.Debugf("Open()")

	w.xlsxFile = xlsx.NewFile()

	return nil
}
func (w *XLSXWriter) Close() error {

	lg.Debugf("Close()")

	if w.xlsxFile == nil {
		return util.Errorf("unable to write nil XLSX: must be first opened")
	}

	err := w.xlsxFile.Write(os.Stderr)
	if err != nil {
		return util.Errorf("unable to write XLSX: %v", err)
	}
	return nil
}

func (w *XLSXWriter) ResultRows(rows []*common.ResultRow) error {

	lg.Debugf("ResultRows()")

	if w.xlsxFile == nil {
		return util.Errorf("unable to write nil XLSX file: must be first opened")
	}
	if len(rows) == 0 {
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

	return nil
}
