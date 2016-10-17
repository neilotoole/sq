package json

import (
	"fmt"

	"github.com/neilotoole/sq/cmd/out/json/pretty"
	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/drvr/sqlh"
)

type JSONWriter struct {
	formatter *pretty.Formatter
}

func NewWriter() *JSONWriter {

	j := &JSONWriter{}
	j.formatter = pretty.NewFormatter()
	return j
}

func (w *JSONWriter) Records(rows []*sqlh.Record) error {

	bytes, err := w.formatter.FormatRecords(rows)
	if err != nil {
		return err
	}

	fmt.Println(string(bytes))
	return nil
}

func (w *JSONWriter) Metadata(meta *drvr.SourceMetadata) error {

	//om := &common.OrderedMap{}

	// TODO: convert this to use an ordered map
	bytes, err := w.formatter.Marshal(meta)
	if err != nil {
		return err
	}

	fmt.Println(string(bytes))
	return nil
}

func (rw *JSONWriter) Open() error {
	return nil
}

func (rw *JSONWriter) Close() error {
	return nil
}
