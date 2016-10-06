package json

import (
	"fmt"

	"github.com/neilotoole/sq/lib/common"
	"github.com/neilotoole/sq/lib/driver"
	"github.com/neilotoole/sq/lib/out/json/pretty"
)

type JSONWriter struct {
	formatter *pretty.Formatter
}

func NewWriter() *JSONWriter {

	j := &JSONWriter{}
	j.formatter = pretty.NewFormatter()
	return j
}

func (w *JSONWriter) ResultRows(rows []*common.ResultRow) error {

	bytes, err := w.formatter.FormatRows(rows)
	if err != nil {
		return err
	}

	fmt.Println(string(bytes))
	return nil
}

func (w *JSONWriter) Metadata(meta *driver.SourceMetadata) error {

	//om := &common.OrderedMap{}

	// TODO: convert this to use an ordered map
	bytes, err := w.formatter.Marshal(meta)
	if err != nil {
		return err
	}

	fmt.Println(string(bytes))
	return nil
}
