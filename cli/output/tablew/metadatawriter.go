package tablew

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/cli/output/yamlw"

	"github.com/neilotoole/sq/libsq/core/kind"

	"github.com/samber/lo"
	"golang.org/x/exp/slices"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

var _ output.MetadataWriter = (*mdWriter)(nil)

type mdWriter struct {
	tbl *table
}

// NewMetadataWriter returns a new output.MetadataWriter instance
// that outputs metadata in table format.
func NewMetadataWriter(out io.Writer, pr *output.Printing) output.MetadataWriter {
	tbl := &table{out: out, pr: pr, header: true}
	w := &mdWriter{tbl: tbl}
	w.tbl.reset()
	return w
}

// DriverMetadata implements output.MetadataWriter.
func (w *mdWriter) DriverMetadata(drvrs []driver.Metadata) error {
	headers := []string{"DRIVER", "DESCRIPTION", "USER-DEFINED", "DOC"}
	w.tbl.tblImpl.SetHeader(headers)
	w.tbl.tblImpl.SetColTrans(2, w.tbl.pr.Bool.SprintFunc())

	var rows [][]string
	for _, md := range drvrs {
		row := []string{string(md.Type), md.Description, strconv.FormatBool(md.UserDefined), md.Doc}
		rows = append(rows, row)
	}
	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}

// TableMetadata implements output.MetadataWriter.
func (w *mdWriter) TableMetadata(tblMeta *source.TableMetadata) error {
	var headers []string
	var rows [][]string

	colNames := make([]string, len(tblMeta.Columns))
	colTypes := make([]string, len(tblMeta.Columns))

	for i, col := range tblMeta.Columns {
		colNames[i] = col.Name
		colTypes[i] = col.ColumnType
	}

	size := "-"
	if tblMeta.Size != nil {
		size = stringz.ByteSized(*tblMeta.Size, 1, "")
	}

	if w.tbl.pr.Verbose {
		headers = []string{"TABLE", "ROWS", "TYPE", "SIZE", "NUM COLS", "COL NAMES", "COL TYPES"}

		w.tbl.tblImpl.SetHeader(headers)
		w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.Handle.SprintFunc())
		w.tbl.tblImpl.SetColTrans(1, w.tbl.pr.Number.SprintFunc())
		w.tbl.tblImpl.SetColTrans(3, w.tbl.pr.Number.SprintFunc())
		w.tbl.tblImpl.SetColTrans(4, w.tbl.pr.Number.SprintFunc())

		row := []string{
			tblMeta.Name,
			fmt.Sprintf("%d", tblMeta.RowCount),
			tblMeta.TableType,
			size,
			fmt.Sprintf("%d", len(tblMeta.Columns)),
			strings.Join(colNames, ", "),
			strings.Join(colTypes, ", "),
		}
		rows = append(rows, row)
	} else {
		headers = []string{"TABLE", "ROWS", "COL NAMES"}

		w.tbl.tblImpl.SetHeader(headers)
		w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.Handle.SprintFunc())
		w.tbl.tblImpl.SetColTrans(1, w.tbl.pr.Number.SprintFunc())

		row := []string{
			tblMeta.Name,
			fmt.Sprintf("%d", tblMeta.RowCount),
			strings.Join(colNames, ", "),
		}
		rows = append(rows, row)
	}

	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}

// SourceMetadata implements output.MetadataWriter.
func (w *mdWriter) SourceMetadata(md *source.Metadata, showSchema bool) error {
	if !showSchema {
		return w.doSourceMetaNoSchema(md)
	}

	return w.doSourceMetaFull(md)
}

func (w *mdWriter) doSourceMetaNoSchema(md *source.Metadata) error {
	headers := []string{"HANDLE", "DRIVER", "NAME", "FQ NAME", "SIZE", "LOCATION"}
	w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.Handle.SprintFunc())
	w.tbl.tblImpl.SetColTrans(5, w.tbl.pr.Number.SprintFunc())
	row := []string{
		md.Handle,
		md.Driver.String(),
		md.Name,
		md.FQName,
		stringz.ByteSized(md.Size, 1, ""),
		source.RedactLocation(md.Location),
	}

	w.tbl.tblImpl.SetHeader(headers)
	w.tbl.renderRow(row)
	return nil
}

func (w *mdWriter) printSourceTablesVerbose(tblMeta []*source.TableMetadata) error {
	w.tbl.reset()

	headers := []string{"TABLE", "ROWS", "TYPE", "SIZE", "NUM COLS", "COL NAMES", "COL TYPES"}
	w.tbl.tblImpl.SetHeader(headers)
	w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.Handle.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, w.tbl.pr.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(3, w.tbl.pr.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(4, w.tbl.pr.Number.SprintFunc())

	var rows [][]string
	var row []string

	for _, tbl := range tblMeta {
		colNames := make([]string, len(tbl.Columns))
		colTypes := make([]string, len(tbl.Columns))

		for i, col := range tbl.Columns {
			colNames[i] = col.Name
			colTypes[i] = col.ColumnType
		}

		size := "-"
		if tbl.Size != nil {
			size = stringz.ByteSized(*tbl.Size, 1, "")
		}

		row = []string{
			tbl.Name,
			fmt.Sprintf("%d", tbl.RowCount),
			tbl.TableType,
			size,
			fmt.Sprintf("%d", len(tbl.Columns)),
			strings.Join(colNames, ", "),
			strings.Join(colTypes, ", "),
		}

		rows = append(rows, row)
	}

	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}

func (w *mdWriter) printSourceTables(tables []*source.TableMetadata) error {
	w.tbl.reset()

	headers := []string{"TABLE", "ROWS", "COL NAMES"}
	w.tbl.tblImpl.SetHeader(headers)
	w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.Handle.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, w.tbl.pr.Number.SprintFunc())

	var rows [][]string
	var row []string

	for _, tbl := range tables {
		colNames := make([]string, len(tbl.Columns))
		colTypes := make([]string, len(tbl.Columns))

		for i, col := range tbl.Columns {
			colNames[i] = col.Name
			colTypes[i] = col.ColumnType
		}

		row = []string{
			tbl.Name,
			fmt.Sprintf("%d", tbl.RowCount),
			strings.Join(colNames, ", "),
		}

		rows = append(rows, row)
	}

	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}

func (w *mdWriter) doSourceMetaFull(md *source.Metadata) error {
	var headers []string
	var row []string

	headers = []string{"HANDLE", "DRIVER", "NAME", "FQ NAME", "SIZE", "TABLES", "LOCATION"}
	w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.Handle.SprintFunc())
	w.tbl.tblImpl.SetColTrans(4, w.tbl.pr.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(5, w.tbl.pr.Number.SprintFunc())
	row = []string{
		md.Handle,
		md.Driver.String(),
		md.Name,
		md.FQName,
		stringz.ByteSized(md.Size, 1, ""),
		fmt.Sprintf("%d", len(md.Tables)),
		source.RedactLocation(md.Location),
	}

	w.tbl.tblImpl.SetHeader(headers)
	w.tbl.renderRow(row)

	if len(md.Tables) == 0 {
		return nil
	}

	fmt.Fprintln(w.tbl.out)
	w.tbl.reset()

	if w.tbl.pr.Verbose {
		return w.printSourceTablesVerbose(md.Tables)
	}

	return w.printSourceTables(md.Tables)
}

// DBProperties implements output.MetadataWriter.
func (w *mdWriter) DBProperties(props map[string]any) error {
	if len(props) == 0 {
		return nil
	}

	// For nested values, we make use of yamlw's rendering.
	yamlPr := w.tbl.pr.Clone()
	yamlPr.Key = yamlPr.Faint

	headers := []string{"KEY", "VALUE"}
	w.tbl.tblImpl.SetHeader(headers)
	w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.Key.SprintFunc())

	rows := make([][]string, 0, len(props))

	keys := lo.Keys(props)
	slices.Sort(keys)
	for _, key := range keys {
		val, ok := props[key]
		if !ok || val == nil {
			continue
		}

		var row []string

		// Most properties have scalar values. However, some are nested
		// arrays of maps (I'm looking at you, SQLite). YAML output is preferred
		// for this sort of nested structure, but we'll hack an ugly solution
		// here for text output.
		switch val := val.(type) {
		case map[string]any:
			s := fmt.Sprintf("%v", val)
			row = []string{key, s}
		case []any:
			var elements []string

			for _, item := range val {
				switch item := item.(type) {
				case map[string]any:
					s, err := yamlw.MarshalToString(yamlPr, item)
					if err != nil {
						return err
					}

					s = strings.ReplaceAll(s, "\n", "  ")
					elements = append(elements, s)
				case []string:
					s := strings.Join(item, " ")
					elements = append(elements, s)
				default:
					s := w.tbl.renderResultCell(kind.Text, item)
					elements = append(elements, s)
				}
			}

			row = []string{key, strings.Join(elements, "\n")}
		default:
			s := w.tbl.renderResultCell(kind.Text, val)
			row = []string{key, s}
		}

		rows = append(rows, row)
	}

	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}
