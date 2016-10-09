package xlsx

import (
	"io"
	"os"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/lib/common"
	"github.com/neilotoole/sq/lib/drvr"
	"github.com/neilotoole/sq/lib/util"
)

var w io.Writer = os.Stdout

type XLSXWriter struct {
}

func NewWriter() *XLSXWriter {

	return &XLSXWriter{}
}

func (rw *XLSXWriter) Metadata(meta *drvr.SourceMetadata) error {
	return util.Errorf("not implemented")
}

func (rw *XLSXWriter) Open() error {
	lg.Debugf("Open()")
	return nil
}
func (rw *XLSXWriter) Close() error {
	lg.Debugf("Close()")
	return nil
}

func (rw *XLSXWriter) ResultRows(rows []*common.ResultRow) error {

	lg.Debugf("ResultRows()")
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
