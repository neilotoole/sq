package csv

import (
	"encoding/csv"
	"os"

	"fmt"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq/lib/common"
	"github.com/neilotoole/sq/lib/drvr"
	"github.com/neilotoole/sq/lib/util"
)

type CSVWriter struct {
	csv         *csv.Writer
	header      bool
	needsHeader bool
}

func NewWriter(header bool, sep rune) *CSVWriter {

	lg.Debugf("header: %v  sep: %v", sep)
	csv := csv.NewWriter(os.Stdout)
	csv.Comma = sep
	return &CSVWriter{header: header, needsHeader: header, csv: csv}
}

func (w *CSVWriter) Metadata(meta *drvr.SourceMetadata) error {

	return util.Errorf("not implemented")
}

func (w *CSVWriter) Open() error {

	return nil
}
func (w *CSVWriter) Close() error {

	w.csv.Flush()
	return nil
}

func (w *CSVWriter) ResultRows(rows []*common.ResultRow) error {

	lg.Debugf("row count: %v", len(rows))
	for _, row := range rows {

		if w.needsHeader {

			colTypes := row.Fields

			headerRow := make([]string, len(colTypes))

			for i, colType := range row.Fields {
				headerRow[i] = colType.Name
			}

			w.csv.Write(headerRow)
			w.needsHeader = false
		}

		vals := row.Values

		cells := make([]string, len(vals))

		for i, val := range vals {

			switch val := val.(type) {
			case nil:
			case *[]byte:
				cells[i] = fmt.Sprintf("%v", *val)
			case *sql.NullString:
				cells[i] = val.String
			case *sql.NullBool:

				if val.Valid {
					cells[i] = fmt.Sprintf("%v", val.Bool)
				}

			case *sql.NullInt64:

				if val.Valid {
					cells[i] = fmt.Sprintf("%v", val.Int64)

				}
			case *sql.NullFloat64:

				if val.Valid {
					cells[i] = fmt.Sprintf("%v", val.Float64)
				}
				// TODO: support datetime

			default:
				cells[i] = fmt.Sprintf("%v", val)
				lg.Debugf("unexpected column value type, treating as default: %T(%v)", val, val)
			}

		}

		lg.Debugf("writing cells: %v", cells)
		err := w.csv.Write(cells)
		if err != nil {
			return util.WrapError(err)
		}

	}

	w.csv.Flush()
	return nil
}
