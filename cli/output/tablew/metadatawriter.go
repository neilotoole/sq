package tablew

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/stringz"
)

type mdWriter struct {
	tbl *table
}

// NewMetadataWriter returns a new output.MetadataWriter instance
// that outputs metadata in table format.
func NewMetadataWriter(out io.Writer, fm *output.Formatting) output.MetadataWriter {
	tbl := &table{out: out, fm: fm, header: true}
	w := &mdWriter{tbl: tbl}
	w.tbl.reset()
	return w
}

// DriverMetadata implements output.MetadataWriter.
func (w *mdWriter) DriverMetadata(drvrs []driver.Metadata) error {
	headers := []string{"DRIVER", "DESCRIPTION", "USER-DEFINED", "DOC"}
	w.tbl.tblImpl.SetHeader(headers)
	w.tbl.tblImpl.SetColTrans(2, w.tbl.fm.Bool.SprintFunc())

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
	headers := []string{"TABLE", "ROWS", "TYPE", "SIZE", "NUM COLS", "COL NAMES", "COL TYPES"}

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

	w.tbl.tblImpl.SetHeader(headers)
	w.tbl.tblImpl.SetColTrans(0, w.tbl.fm.Handle.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, w.tbl.fm.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(3, w.tbl.fm.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(4, w.tbl.fm.Number.SprintFunc())

	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}

// SourceMetadata implements output.MetadataWriter.
func (w *mdWriter) SourceMetadata(meta *source.Metadata) error {
	var headers []string
	var row []string

	if meta.Name == meta.FQName {
		headers = []string{"HANDLE", "DRIVER", "NAME", "SIZE", "TABLES", "LOCATION"}
		w.tbl.tblImpl.SetColTrans(0, w.tbl.fm.Handle.SprintFunc())
		w.tbl.tblImpl.SetColTrans(3, w.tbl.fm.Number.SprintFunc())
		w.tbl.tblImpl.SetColTrans(4, w.tbl.fm.Number.SprintFunc())
		row = []string{
			meta.Handle,
			meta.SourceType.String(),
			meta.Name,
			stringz.ByteSized(meta.Size, 1, ""),
			fmt.Sprintf("%d", len(meta.Tables)),
			meta.Location,
		}
	} else {
		headers = []string{"HANDLE", "DRIVER", "NAME", "FQ NAME", "SIZE", "TABLES", "LOCATION"}
		w.tbl.tblImpl.SetColTrans(0, w.tbl.fm.Handle.SprintFunc())
		w.tbl.tblImpl.SetColTrans(4, w.tbl.fm.Number.SprintFunc())
		w.tbl.tblImpl.SetColTrans(5, w.tbl.fm.Number.SprintFunc())
		row = []string{
			meta.Handle,
			meta.SourceType.String(),
			meta.Name,
			meta.FQName,
			stringz.ByteSized(meta.Size, 1, ""),
			fmt.Sprintf("%d", len(meta.Tables)),
			meta.Location,
		}
	}

	w.tbl.tblImpl.SetHeader(headers)
	w.tbl.renderRow(row)
	w.tbl.reset()
	fmt.Fprintln(w.tbl.out)

	headers = []string{"TABLE", "ROWS", "TYPE", "SIZE", "NUM COLS", "COL NAMES", "COL TYPES"}

	var rows [][]string

	for _, tbl := range meta.Tables {
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

	w.tbl.tblImpl.SetHeader(headers)
	w.tbl.tblImpl.SetColTrans(0, w.tbl.fm.Handle.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, w.tbl.fm.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(3, w.tbl.fm.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(4, w.tbl.fm.Number.SprintFunc())

	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}
