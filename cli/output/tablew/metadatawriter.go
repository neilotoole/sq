package tablew

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/samber/lo"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/yamlw"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source/location"
	"github.com/neilotoole/sq/libsq/source/metadata"
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
	return w.tbl.appendRowsAndRenderAll(context.TODO(), rows)
}

// TableMetadata implements output.MetadataWriter.
func (w *mdWriter) TableMetadata(tblMeta *metadata.Table) error {
	if w.tbl.pr.Verbose {
		return w.doTableMetaVerbose(tblMeta)
	}

	return w.doTableMeta(tblMeta)
}

func (w *mdWriter) doTableMeta(md *metadata.Table) error {
	var headers []string
	var rows [][]string

	colNames := make([]string, len(md.Columns))
	colTypes := make([]string, len(md.Columns))

	for i, col := range md.Columns {
		colNames[i] = col.Name
		colTypes[i] = col.ColumnType
	}

	headers = []string{"NAME", "TYPE", "ROWS", "COLS"}

	w.tbl.tblImpl.SetHeader(headers)
	w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.String.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, w.tbl.pr.Faint.SprintFunc())
	w.tbl.tblImpl.SetColTrans(2, w.tbl.pr.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(3, w.tbl.pr.String.SprintFunc())

	row := []string{
		md.Name,
		md.TableType,
		strconv.FormatInt(md.RowCount, 10),
		strings.Join(colNames, ", "),
	}
	rows = append(rows, row)

	return w.tbl.appendRowsAndRenderAll(context.TODO(), rows)
}

func (w *mdWriter) doTableMetaVerbose(tblMeta *metadata.Table) error {
	return w.printTablesVerbose([]*metadata.Table{tblMeta})
}

// SourceMetadata implements output.MetadataWriter.
func (w *mdWriter) SourceMetadata(md *metadata.Source, showSchema bool) error {
	if !showSchema {
		return w.doSourceMetaNoSchema(md)
	}

	return w.doSourceMetaFull(md)
}

func (w *mdWriter) doSourceMetaNoSchema(md *metadata.Source) error {
	headers := []string{
		"SOURCE",
		"DRIVER",
		"NAME",
		"FQ NAME",
		"SIZE",
		"LOCATION",
	}
	w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.Handle.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, w.tbl.pr.Faint.SprintFunc())
	w.tbl.tblImpl.SetColTrans(2, w.tbl.pr.String.SprintFunc())
	w.tbl.tblImpl.SetColTrans(3, w.tbl.pr.Faint.SprintFunc())
	w.tbl.tblImpl.SetColTrans(4, w.tbl.pr.Faint.SprintFunc())
	w.tbl.tblImpl.SetColTrans(5, w.tbl.pr.Faint.SprintFunc())

	loc := md.Location
	if w.tbl.pr.Redact {
		loc = location.Redact(loc)
	}
	row := []string{
		md.Handle,
		md.Driver.String(),
		md.Name,
		md.FQName,
		w.tbl.pr.Number.Sprint(stringz.ByteSized(md.Size, 1, "")),
		loc,
	}

	w.tbl.tblImpl.SetHeader(headers)
	return w.tbl.writeRow(context.TODO(), row)
}

func (w *mdWriter) printTablesVerbose(tbls []*metadata.Table) error {
	w.tbl.reset()

	headers := []string{
		"NAME",
		"TYPE",
		"ROWS",
		"COLS",
		"NAME",
		"TYPE",
		"PK",
	}
	w.tbl.tblImpl.SetHeader(headers)
	w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.String.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, w.tbl.pr.Faint.SprintFunc())
	w.tbl.tblImpl.SetColTrans(2, w.tbl.pr.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(3, w.tbl.pr.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(4, w.tbl.pr.String.SprintFunc())
	w.tbl.tblImpl.SetColTrans(5, w.tbl.pr.Faint.SprintFunc())
	w.tbl.tblImpl.SetColTrans(6, w.tbl.pr.Faint.SprintFunc())

	var rows [][]string
	var row []string

	getPK := func(col *metadata.Column) string {
		if !col.PrimaryKey {
			return ""
		}

		return w.tbl.pr.Bool.Sprint("pk")
	}

	for _, tbl := range tbls {
		row = []string{
			tbl.Name,
			tbl.TableType,
			strconv.FormatInt(tbl.RowCount, 10),
			w.tbl.pr.Faint.Sprintf("%d", len(tbl.Columns)),
			tbl.Columns[0].Name,
			tbl.Columns[0].BaseType,
			getPK(tbl.Columns[0]),
		}

		rows = append(rows, row)

		for i := 1; i < len(tbl.Columns); i++ {
			row = []string{
				"",
				"",
				"",
				"",
				tbl.Columns[i].Name,
				tbl.Columns[i].BaseType,
				getPK(tbl.Columns[i]),
			}
			rows = append(rows, row)
		}
	}

	return w.tbl.appendRowsAndRenderAll(context.TODO(), rows)
}

func (w *mdWriter) printTables(tables []*metadata.Table) error {
	w.tbl.reset()

	headers := []string{"NAME", "TYPE", "ROWS", "COLS"}
	w.tbl.tblImpl.SetHeader(headers)
	w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.String.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, w.tbl.pr.Faint.SprintFunc())
	w.tbl.tblImpl.SetColTrans(2, w.tbl.pr.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(3, w.tbl.pr.Faint.SprintFunc())

	var rows [][]string
	var row []string

	for _, tbl := range tables {
		colNames := make([]string, len(tbl.Columns))

		for i, col := range tbl.Columns {
			colNames[i] = col.Name
		}

		row = []string{
			tbl.Name,
			tbl.TableType,
			strconv.FormatInt(tbl.RowCount, 10),
			w.tbl.pr.String.Sprint(strings.Join(colNames, ", ")),
		}

		rows = append(rows, row)
	}

	return w.tbl.appendRowsAndRenderAll(context.TODO(), rows)
}

func (w *mdWriter) doSourceMetaFull(md *metadata.Source) error {
	var headers []string
	var row []string

	headers = []string{
		"SOURCE",
		"DRIVER",
		"NAME",
		"FQ NAME",
		"SIZE",
		"TABLES",
		"VIEWS",
		"LOCATION",
	}
	w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.Handle.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, w.tbl.pr.Faint.SprintFunc())
	w.tbl.tblImpl.SetColTrans(2, w.tbl.pr.String.SprintFunc())
	w.tbl.tblImpl.SetColTrans(3, w.tbl.pr.Faint.SprintFunc())
	w.tbl.tblImpl.SetColTrans(4, w.tbl.pr.Faint.SprintFunc())
	w.tbl.tblImpl.SetColTrans(5, w.tbl.pr.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(6, w.tbl.pr.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(7, w.tbl.pr.Faint.SprintFunc())

	loc := md.Location
	if w.tbl.pr.Redact {
		loc = location.Redact(loc)
	}
	row = []string{
		md.Handle,
		md.Driver.String(),
		md.Name,
		md.FQName,
		w.tbl.pr.Number.Sprint(stringz.ByteSized(md.Size, 1, "")),
		strconv.FormatInt(md.TableCount, 10),
		strconv.FormatInt(md.ViewCount, 10),
		loc,
	}

	w.tbl.tblImpl.SetHeader(headers)
	if err := w.tbl.writeRow(context.TODO(), row); err != nil {
		return err
	}

	if len(md.Tables) == 0 {
		return nil
	}

	fmt.Fprintln(w.tbl.out)
	w.tbl.reset()

	// Sort by type (view/table) and name
	slices.SortFunc(md.Tables, func(a, b *metadata.Table) int {
		if a.TableType == b.TableType {
			return cmp.Compare(a.Name, b.Name)
		}

		return cmp.Compare(a.TableType, b.TableType)
	})

	if w.tbl.pr.Verbose {
		return w.printTablesVerbose(md.Tables)
	}

	return w.printTables(md.Tables)
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

	return w.tbl.appendRowsAndRenderAll(context.TODO(), rows)
}

// Catalogs implements output.MetadataWriter.
func (w *mdWriter) Catalogs(currentCatalog string, catalogs []string) error {
	if len(catalogs) == 0 {
		return nil
	}
	pr := w.tbl.pr

	if !pr.Verbose {
		if pr.ShowHeader {
			headers := []string{"CATALOG"}
			w.tbl.tblImpl.SetHeader(headers)
		}
		w.tbl.tblImpl.SetColTrans(0, pr.String.SprintFunc())

		var rows [][]string
		for _, catalog := range catalogs {
			if catalog == currentCatalog {
				catalog = pr.Active.Sprintf(catalog)
			}
			rows = append(rows, []string{catalog})
		}
		return w.tbl.appendRowsAndRenderAll(context.TODO(), rows)
	}

	// Verbose mode
	if pr.ShowHeader {
		headers := []string{"CATALOG", "ACTIVE"}
		w.tbl.tblImpl.SetHeader(headers)
	}

	w.tbl.tblImpl.SetColTrans(0, pr.String.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, pr.Bool.SprintFunc())

	var rows [][]string
	for _, catalog := range catalogs {
		var active string
		if catalog == currentCatalog {
			catalog = pr.Active.Sprintf(catalog)
			active = pr.Bool.Sprint("active")
		}
		rows = append(rows, []string{catalog, active})
	}
	return w.tbl.appendRowsAndRenderAll(context.TODO(), rows)
}

// Schemata implements output.MetadataWriter.
func (w *mdWriter) Schemata(currentSchema string, schemas []*metadata.Schema) error {
	if len(schemas) == 0 {
		return nil
	}
	pr := w.tbl.pr
	if !pr.Verbose {
		if pr.ShowHeader {
			headers := []string{"SCHEMA"}
			w.tbl.tblImpl.SetHeader(headers)
		}
		w.tbl.tblImpl.SetColTrans(0, pr.String.SprintFunc())
		var rows [][]string
		for _, schema := range schemas {
			s := schema.Name
			if schema.Name == currentSchema {
				s = pr.Active.Sprintf(s)
			}
			rows = append(rows, []string{s})
		}
		return w.tbl.appendRowsAndRenderAll(context.TODO(), rows)
	}

	// Verbose mode
	if pr.ShowHeader {
		headers := []string{"SCHEMA", "CATALOG", "OWNER", "ACTIVE"}
		w.tbl.tblImpl.SetHeader(headers)
	}

	w.tbl.tblImpl.SetColTrans(0, pr.String.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, pr.String.SprintFunc())
	w.tbl.tblImpl.SetColTrans(2, pr.String.SprintFunc())
	w.tbl.tblImpl.SetColTrans(3, pr.Bool.SprintFunc())

	var rows [][]string
	for _, schema := range schemas {
		row := []string{schema.Name, schema.Catalog, schema.Owner, ""}

		if schema.Name == currentSchema {
			row[0] = pr.Active.Sprintf(row[0])
			row[3] = "active"
		}
		rows = append(rows, row)
	}
	return w.tbl.appendRowsAndRenderAll(context.TODO(), rows)
}
