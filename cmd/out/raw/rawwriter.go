package raw

import (
	"fmt"
	"io"
	"os"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/drvr/sqlh"
	"github.com/neilotoole/sq/libsq/util"
)

var w io.Writer = os.Stdout

type RawWriter struct {
}

func NewWriter() *RawWriter {

	return &RawWriter{}
}

func (rw *RawWriter) Metadata(meta *drvr.SourceMetadata) error {
	return util.Errorf("not implemented")
}

func (rw *RawWriter) Close() error {
	return nil
}

func (rw *RawWriter) Records(rows []*sqlh.Record) error {

	if len(rows) == 0 {
		return nil
	}

	for _, row := range rows {

		for _, val := range row.Values {
			switch val := val.(type) {
			case nil:
			case *[]byte:
				w.Write(*val)
			case *sql.NullString:
				if val.Valid {
					fmt.Fprintf(w, val.String)
				}
			case *sql.NullBool:

				if val.Valid {
					fmt.Fprintf(w, "%t", val.Bool)
				}

			case *sql.NullInt64:

				if val.Valid {
					fmt.Fprintf(w, "%d", val.Int64)
				}
			case *sql.NullFloat64:

				if val.Valid {
					fmt.Fprintf(w, "%f", val.Float64)
				}

			default:
				lg.Debugf("unexpected column value type, treating as default: %T(%v)", val, val)
				fmt.Fprintf(w, "%v", val)
			}
			fmt.Fprintln(w) // Add the new line
		}

	}

	return nil
}
