package xml

import (
	"fmt"

	"bytes"

	"encoding/base64"

	"github.com/clbanning/mxj"
	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq/lib/common"
	"github.com/neilotoole/sq/lib/drvr"
	"github.com/neilotoole/sq/lib/util"
)

type XMLWriter struct {
	buf *bytes.Buffer
}

func NewWriter() *XMLWriter {

	return &XMLWriter{buf: &bytes.Buffer{}}

}

func (w *XMLWriter) Metadata(meta *drvr.SourceMetadata) error {

	return util.Errorf("not implemented")
}

func (w *XMLWriter) Open() error {

	return nil
}
func (w *XMLWriter) Close() error {

	w.buf.WriteString("</rows>")

	_, err := fmt.Println(w.buf.String())
	return err
}

func (w *XMLWriter) ResultRows(rows []*common.ResultRow) error {

	if w.buf.Len() == 0 {
		w.buf.WriteString("<rows>\n")
	}

	// REVISIT (neilotoole): the library used for marshalling is a bit non-go-like.
	// Consider changing it in future.

	// TODO (neilotoole): the output should be colorized

	for _, row := range rows {

		m := make(map[string]interface{})

		vals := row.Values

		cells := make([]string, len(vals))

		colTypes := row.Fields

		for i, val := range vals {

			switch val := val.(type) {
			case nil:
				m[colTypes[i].Name] = ""
			case *[]byte:
				// do some base64 on this shit
				m[colTypes[i].Name] = base64.StdEncoding.EncodeToString(*val)
			case *sql.NullString:
				m[colTypes[i].Name] = val.String
			case *sql.NullBool:

				if val.Valid {
					cells[i] = fmt.Sprintf("%v", val.Bool)
				} else {
					cells[i] = ""
				}

			case *sql.NullInt64:

				if val.Valid {
					m[colTypes[i].Name] = fmt.Sprintf("%v", val.Int64)
				} else {
					m[colTypes[i].Name] = ""
				}
			case *sql.NullFloat64:

				if val.Valid {
					m[colTypes[i].Name] = fmt.Sprintf("%v", val.Float64)
				} else {
					m[colTypes[i].Name] = ""
				}
				// TODO: support datetime

			default:

				m[colTypes[i].Name] = fmt.Sprintf("%v", val)
				lg.Debugf("unexpected column value type, treating as default: %T(%v)", val, val)
			}

		}

		item := mxj.Map(m)
		bytes, err := item.XmlIndent("  ", "  ", "row")
		if err != nil {
			return util.Errorf("unable to marshall XML: %v", err)
		}

		w.buf.WriteString(string(bytes) + "\n")
	}

	//w.csv.Flush()
	return nil
}

//
//func main() {
//
//	//type doc struct {
//	//	rows []map[string]interface{}
//	//}
//	//
//	//d := &doc{}
//
//	buf := &bytes.Buffer{}
//	buf.WriteString("<rows>\n")
//
//	items := []mxj.Map{}
//
//	m1 := make(map[string]interface{})
//	m1["uid"] = 1
//	m1["username"] = "neilotoole"
//
//	m2 := make(map[string]interface{})
//	m2["uid"] = 2
//	m2["username"] = "ksoze"
//
//	items = append(items, mxj.Map(m1))
//	items = append(items, mxj.Map(m2))
//
//	for _, item := range items {
//		bytes, err := item.XmlIndent("  ", "  ", "row")
//
//		//bytes, err := xml.MarshalIndent(m, "", "  ")
//		if err != nil {
//			panic(err)
//		}
//
//		buf.WriteString(string(bytes) + "\n")
//
//	}
//
//	//d.rows = append(d.rows, m1)
//	//d.rows = append(d.rows, m2)
//	//
//	//mxStruct, err := mxj.NewMapStruct(d)
//	//if err != nil {
//	//	panic(err)
//	//}
//	//
//	//bytes, err := mxStruct.XmlIndent("", "  ", "rows")
//
//	//mxMap := mxj.Map(m1)
//	////
//	//////mx, err := mxj.NewMapStruct(m)
//	//////if err != nil {
//	//////	panic(err)
//	//////}
//	////
//	//bytes, err := mxMap.XmlIndent("", "  ", "row")
//	//
//	////bytes, err := xml.MarshalIndent(m, "", "  ")
//	//if err != nil {
//	//	panic(err)
//	//}
//	//
//	//buf.WriteString(string(bytes))
//
//	buf.WriteString("</rows>")
//
//	fmt.Println(buf.String())
//}
