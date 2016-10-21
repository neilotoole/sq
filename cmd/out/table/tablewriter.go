package table

import (
	"fmt"
	"os"

	"strconv"

	"strings"

	"github.com/fatih/color"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq/cmd/out"
	"github.com/neilotoole/sq/cmd/out/json/pretty"
	"github.com/neilotoole/sq/cmd/out/table/internal"
	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/drvr/sqlh"
	"github.com/neilotoole/sq/libsq/util"
)

type TextWriter struct {
	tbl    *internal.Table
	f      *pretty.Formatter
	header bool
}

func NewWriter(header bool) *TextWriter {

	t := &TextWriter{
		header: header,
	}

	t.Reset()
	return t
}

func (t *TextWriter) Reset() {

	t.tbl = internal.NewWriter(os.Stdout)
	t.setTableWriterOptions()
	t.f = pretty.NewFormatter()
	t.tbl.SetAutoFormatHeaders(false)
	t.tbl.SetAutoWrapText(false)
}

func (t *TextWriter) setTableWriterOptions() {
	t.tbl.SetAlignment(internal.AlignLeft)
	t.tbl.SetAutoWrapText(true)
	t.tbl.SetBorder(false)
	t.tbl.SetHeaderAlignment(internal.AlignLeft)
	t.tbl.SetCenterSeparator("")
	t.tbl.SetColumnSeparator("")
	t.tbl.SetRowSeparator("")
	t.tbl.SetBorders(internal.Border{Left: false, Top: false, Right: false, Bottom: false})
	t.tbl.SetAutoFormatHeaders(false)
	t.tbl.SetHeaderDisable(!t.header)
}

func (t *TextWriter) Value(message string, key string, value interface{}) {

	if message == "" {
		fmt.Printf("%v\n", value)
		return
	}

	fmt.Printf("%v: %v\n", message, value)
}

func (t *TextWriter) SourceSet(ss *drvr.SourceSet, active *drvr.Source) error {
	var rows [][]string

	for i, src := range ss.Items {

		row := []string{
			src.Handle,
			string(src.Type),
			src.Location,
			t.renderSrcOptions(src)}

		if active != nil && src.Handle == active.Handle {
			// TODO: Add "SetRowTrans" or AddRowTrans
			t.tbl.SetCellTrans(i, 0, out.Trans.Bold)
			t.tbl.SetCellTrans(i, 1, out.Trans.Bold)
			t.tbl.SetCellTrans(i, 2, out.Trans.Bold)
			t.tbl.SetCellTrans(i, 3, out.Trans.Bold)
			t.tbl.SetCellTrans(i, 4, out.Trans.Bold)
		}

		rows = append(rows, row)
	}

	// TODO: currently we always output headers for sources
	t.tbl.SetHeaderDisable(false)
	t.tbl.SetColTrans(0, out.Trans.Number)
	t.tbl.SetHeader([]string{"HANDLE", "DRIVER", "LOCATION", "OPTIONS"})
	t.renderRows(rows)
	t.tbl.SetHeaderDisable(t.header)
	return nil
}

func (t *TextWriter) renderSrcOptions(src *drvr.Source) string {
	if src == nil || src.Options == nil || len(src.Options) == 0 {
		return ""
	}

	opts := make([]string, 0, len(src.Options))

	for key, vals := range src.Options {
		if key == "" {
			continue
		}
		v := strings.Join(vals, ",")
		// TODO: add color here to distinguish the keys/values
		opts = append(opts, fmt.Sprintf("%s=%s", key, v))
	}
	return strings.Join(opts, " ")
}

func (t *TextWriter) Source(src *drvr.Source) error {

	var rows [][]string

	row := []string{
		src.Handle,
		string(src.Type),
		src.Location,
		t.renderSrcOptions(src)}
	rows = append(rows, row)

	t.tbl.SetColTrans(0, out.Trans.Number)
	// TODO: currently we always output headers for sources
	t.tbl.SetHeaderDisable(false)
	t.tbl.SetHeader([]string{"HANDLE", "DRIVER", "LOCATION", "OPTIONS"})
	t.renderRows(rows)
	t.tbl.SetHeaderDisable(t.header)
	return nil
}

// Write out a set of generic rows. Optional provide an array of column transformers.
//func (t *TextWriter) Rows(rows [][]string, colTrans []out.TextTransformer) {
//
//	if len(rows) == 0 {
//		return
//	}
//
//	if colTrans != nil && len(colTrans) > 0 {
//		for i := 0; i < len(rows[0]); i++ {
//			if i < len(colTrans) && colTrans[i] != nil {
//				t.tbl.SetColTrans(i, colTrans[i])
//			}
//		}
//	}
//
//	t.renderRows(rows)
//}

func (t *TextWriter) Error(err error) {
	fmt.Println(out.Trans.Error(fmt.Sprintf("Error: %v", err)))
}

func (t *TextWriter) Help(text string) error {
	fmt.Println(text)
	return nil
}

func (t *TextWriter) renderRows(rows [][]string) {
	for _, v := range rows {
		t.tbl.Append(v)
	}
	t.tbl.Render()
}

func (t *TextWriter) renderRow(row []string) {
	t.tbl.Append(row)
	t.tbl.Render() // Send output
}

func (t *TextWriter) Metadata(meta *drvr.SourceMetadata) error {

	var headers []string
	var row []string

	if meta.Name == meta.FullyQualifiedName {
		headers = []string{"HANDLE", "NAME", "SIZE", "TABLES", "LOCATION"}
		t.tbl.SetColTrans(3, out.Trans.Number)
		row = []string{
			meta.Handle,
			meta.Name,
			util.ByteSized(meta.Size, 1, ""),
			fmt.Sprintf("%d", len(meta.Tables)),
			meta.Location,
		}

	} else {
		headers = []string{"HANDLE", "NAME", "FQ NAME", "SIZE", "TABLES", "LOCATION"}
		t.tbl.SetColTrans(4, out.Trans.Number)
		row = []string{
			meta.Handle,
			meta.Name,
			meta.FullyQualifiedName,
			util.ByteSized(meta.Size, 1, ""),
			fmt.Sprintf("%d", len(meta.Tables)),
			meta.Location,
		}
	}

	t.tbl.SetHeader(headers)
	//t.tbl.SetColTrans(4, out.Trans.Number)
	t.renderRow(row)
	t.Reset()
	fmt.Println()

	headers = []string{"TABLE", "ROWS", "SIZE", "NUM COLS", "COL NAMES", "COL TYPES"}

	var rows [][]string

	for _, tbl := range meta.Tables {

		colNames := make([]string, len(tbl.Columns))
		colTypes := make([]string, len(tbl.Columns))

		for i, col := range tbl.Columns {
			colNames[i] = col.Name
			colTypes[i] = col.ColType
		}

		size := "-"
		if tbl.Size != -1 {
			size = util.ByteSized(tbl.Size, 1, "")
		}

		row := []string{
			tbl.Name,
			fmt.Sprintf("%d", tbl.RowCount),
			size,
			fmt.Sprintf("%d", len(tbl.Columns)),
			strings.Join(colNames, ", "),
			strings.Join(colTypes, ", "),
		}
		rows = append(rows, row)
	}

	t.tbl.SetHeader(headers)
	t.tbl.SetColTrans(1, out.Trans.Number)
	t.tbl.SetColTrans(3, out.Trans.Number)

	t.renderRows(rows)
	return nil
}

func (rw *TextWriter) Close() error {
	return nil
}

func (t *TextWriter) Records(records []*sqlh.Record) error {
	if len(records) == 0 {
		fmt.Println()
		return nil
	}

	t.tbl.SetAutoWrapText(false)

	var rows [][]string

	for _, rsRow := range records {

		row := make([]string, len(rsRow.Values))

		for i, val := range rsRow.Values {
			row[i] = t.renderResultCell(val)
		}

		rows = append(rows, row)
	}

	header := make([]string, len(records[0].Fields))
	for i, field := range records[0].Fields {
		header[i] = field.Name
	}

	t.tbl.SetHeader(header)
	t.renderRows(rows)

	return nil
}

func (t *TextWriter) renderResultCell(val interface{}) string {

	switch val := val.(type) {
	case string:
		return val
	case *sql.NullString:
		if !val.Valid {
			return t.sprintNull()
		}
		return fmt.Sprintf("%s", val.String)
	case *string:
		if val == nil {
			return t.sprintNull()
		}
		return *val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case *float64:
		if val == nil {
			return t.sprintNull()
		}
		//return strconv.FormatFloat(*val, 'f', -1, 64)
		return t.sprintFloat64(*val)
	case *sql.NullFloat64:
		if !val.Valid {
			return t.sprintNull()
		}
		//return strconv.FormatFloat(val.Float64, 'f', -1, 32)
		return t.sprintFloat64(val.Float64)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case *float32:
		if val == nil {
			return t.sprintNull()
		}
		return strconv.FormatFloat(float64(*val), 'f', -1, 32)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case *int:
		if val == nil {
			return t.sprintNull()
		}
		return fmt.Sprintf("%d", *val)
	case *int8:
		if val == nil {
			return t.sprintNull()
		}
		return fmt.Sprintf("%d", *val)
	case *int16:
		if val == nil {
			return t.sprintNull()
		}
		return fmt.Sprintf("%d", *val)
	case *int32:
		if val == nil {
			return t.sprintNull()
		}
		return fmt.Sprintf("%d", *val)
	case *int64:
		if val == nil {
			return t.sprintNull()
		}
		return t.sprintInt(*val)
	case *uint:
		if val == nil {
			return t.sprintNull()
		}
		return fmt.Sprintf("%d", *val)
	case *uint8:
		if val == nil {
			return t.sprintNull()
		}
		return fmt.Sprintf("%d", *val)
	case *uint16:
		if val == nil {
			return t.sprintNull()
		}
		return fmt.Sprintf("%d", *val)
	case *uint32:
		if val == nil {
			return t.sprintNull()
		}
		return fmt.Sprintf("%d", *val)
	case *uint64:
		if val == nil {
			return t.sprintNull()
		}
		return fmt.Sprintf("%d", *val)
	case *sql.NullInt64:
		if !val.Valid {
			return t.sprintNull()
		}
		return t.sprintInt(val.Int64)
	case bool:
		return t.f.SprintfColor(t.f.BoolColor, strconv.FormatBool(val))
	case *bool:
		if val == nil {
			return t.sprintNull()
		}
		return t.f.SprintfColor(t.f.BoolColor, strconv.FormatBool(*val))
	case *sql.NullBool:
		if !val.Valid {
			return t.sprintNull()
		}
		return t.f.SprintfColor(t.f.BoolColor, strconv.FormatBool(val.Bool))
	case nil:
		return t.sprintNull()
	case []byte:
		if val == nil {
			t.sprintNull()
		}
		return t.processBinary(val)
	case *[]byte:
		if val == nil || *val == nil {
			return t.sprintNull()
		}
		return t.processBinary(*val)
	}
	return ""

}

func (t *TextWriter) processBinary(bytes []byte) string {
	s := fmt.Sprintf("[%d]", len(bytes))
	c := color.New(color.Faint)
	return c.SprintFunc()(s)
}

func (t *TextWriter) sprintNull() string {
	return t.f.SprintfColor(t.f.NullColor, "null")
}

func (t *TextWriter) sprintInt(num int64) string {
	return t.f.SprintfColor(t.f.NumberColor, "%d", num)
}
func (t *TextWriter) sprintFloat64(num float64) string {
	s := strconv.FormatFloat(num, 'f', -1, 64)
	return t.f.SprintfColor(t.f.NumberColor, "%s", s)
}
