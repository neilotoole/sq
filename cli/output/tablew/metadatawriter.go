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
		"FK",
		"INDEXES",
		"UNIQUE CONSTRAINTS",
	}
	w.tbl.tblImpl.SetHeader(headers)
	w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.String.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, w.tbl.pr.Faint.SprintFunc())
	w.tbl.tblImpl.SetColTrans(2, w.tbl.pr.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(3, w.tbl.pr.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(4, w.tbl.pr.String.SprintFunc())
	w.tbl.tblImpl.SetColTrans(5, w.tbl.pr.Faint.SprintFunc())
	w.tbl.tblImpl.SetColTrans(6, w.tbl.pr.Faint.SprintFunc())
	w.tbl.tblImpl.SetColTrans(7, w.tbl.pr.Faint.SprintFunc())
	w.tbl.tblImpl.SetColTrans(8, w.tbl.pr.Faint.SprintFunc())
	w.tbl.tblImpl.SetColTrans(9, w.tbl.pr.Faint.SprintFunc())

	var rows [][]string
	var row []string

	getPK := func(col *metadata.Column) string {
		if !col.PrimaryKey {
			return ""
		}

		return w.tbl.pr.Bool.Sprint("pk")
	}

	// formatIdxCell turns the per-column index entries into a single
	// comma-joined cell. UC-backing entries are wrapped in parens and
	// styled with [Printing.Subdued] (italic + faint) so they read as
	// secondary citizens: still visible, but visually deemphasized
	// because UNIQUE CONSTRAINTS already names the same thing. PK
	// filtering happens upstream in [indexEntriesByColumn]; nothing
	// to do here for PK-backing entries.
	formatIdxCell := func(entries []indexEntry) string {
		if len(entries) == 0 {
			return ""
		}
		parts := make([]string, len(entries))
		for i, e := range entries {
			if e.backing {
				parts[i] = w.tbl.pr.Subdued.Sprint("(" + e.name + ")")
			} else {
				parts[i] = e.name
			}
		}
		return strings.Join(parts, ", ")
	}

	for _, tbl := range tbls {
		// Build per-column lookups for FKs, indexes, and unique
		// constraints so each column row can list the entries it
		// participates in. PK-backing indexes are dropped (the PK
		// column already conveys that information); UC-backing
		// indexes are kept but tagged so the renderer can mute them.
		// See [indexEntriesByColumn] for the full pairing rules.
		fkByCol := outgoingFKByColumn(tbl)
		idxByCol := indexEntriesByColumn(tbl)
		ucByCol := uniqueNamesByColumn(tbl)

		// Tables with no visible columns (e.g. an empty view, or a
		// SQLite virtual table whose schema isn't reflected through
		// pragma_table_info) would otherwise index out of range on the
		// header row below. Emit a single summary row and continue.
		if len(tbl.Columns) == 0 {
			rows = append(rows, []string{
				tbl.Name,
				tbl.TableType,
				strconv.FormatInt(tbl.RowCount, 10),
				w.tbl.pr.Faint.Sprintf("%d", 0),
				"", "", "", "", "", "",
			})
			continue
		}

		row = []string{
			tbl.Name,
			tbl.TableType,
			strconv.FormatInt(tbl.RowCount, 10),
			w.tbl.pr.Faint.Sprintf("%d", len(tbl.Columns)),
			tbl.Columns[0].Name,
			tbl.Columns[0].BaseType,
			getPK(tbl.Columns[0]),
			formatFKRefs(fkByCol[tbl.Columns[0].Name]),
			formatIdxCell(idxByCol[tbl.Columns[0].Name]),
			strings.Join(ucByCol[tbl.Columns[0].Name], ", "),
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
				formatFKRefs(fkByCol[tbl.Columns[i].Name]),
				formatIdxCell(idxByCol[tbl.Columns[i].Name]),
				strings.Join(ucByCol[tbl.Columns[i].Name], ", "),
			}
			rows = append(rows, row)
		}
	}

	return w.tbl.appendRowsAndRenderAll(context.TODO(), rows)
}

// outgoingFKByColumn returns a column-name → []*ForeignKey lookup for
// tbl, preserving the order each column was encountered in
// [FKGroup.Outgoing]. For composite FKs each member column lists the
// same FK pointer; for a column that participates in multiple
// outgoing FK constraints (unusual but valid), every entry is kept
// so the renderer can show all of them. Returns nil if the table has
// no outgoing FKs.
func outgoingFKByColumn(tbl *metadata.Table) map[string][]*metadata.ForeignKey {
	if tbl.FK == nil || len(tbl.FK.Outgoing) == 0 {
		return nil
	}
	out := make(map[string][]*metadata.ForeignKey, len(tbl.Columns))
	for _, fk := range tbl.FK.Outgoing {
		if fk == nil {
			continue
		}
		for _, colName := range fk.Columns {
			out[colName] = append(out[colName], fk)
		}
	}
	return out
}

// formatFKRef returns a short human-readable description of fk, of
// the form "ref_table(ref_col)" — or for composite FKs,
// "ref_table(ref_col1, ref_col2)". Cross-schema and cross-catalog
// references are qualified ("ref_schema.ref_table(...)" or
// "ref_catalog.ref_schema.ref_table(...)"); same-source references
// stay unqualified because LinkForeignKeys clears RefCatalog /
// RefSchema when they match the owning Source's catalog / schema.
// Returns the empty string when fk is nil.
func formatFKRef(fk *metadata.ForeignKey) string {
	if fk == nil {
		return ""
	}
	target := fk.RefTable
	if fk.RefSchema != "" {
		target = fk.RefSchema + "." + target
	}
	if fk.RefCatalog != "" {
		target = fk.RefCatalog + "." + target
	}
	return target + "(" + strings.Join(fk.RefColumns, ", ") + ")"
}

// formatFKRefs joins one or more [formatFKRef] outputs with ", ".
// Returns the empty string when fks is empty so the table cell stays
// blank for columns with no outgoing FK participation.
func formatFKRefs(fks []*metadata.ForeignKey) string {
	if len(fks) == 0 {
		return ""
	}
	parts := make([]string, 0, len(fks))
	for _, fk := range fks {
		if s := formatFKRef(fk); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}

// indexEntry is a single entry in the INDEXES column. The backing
// flag signals that the index merely backs a UNIQUE constraint
// already listed under UNIQUE CONSTRAINTS — the renderer mutes those
// entries (parens + [Printing.Subdued]) so they read as secondary
// citizens without being hidden entirely.
type indexEntry struct {
	name    string
	backing bool
}

// indexEntriesByColumn returns a column-name → []indexEntry lookup
// for tbl. PK-backing indexes are filtered (the PK column already
// conveys that information); every other physical index appears in
// the result, with backing=true marking the one that backs each
// declared [metadata.UniqueConstraint].
//
// Pairing rules:
//
//   - PK-backing indexes (idx.Primary) are dropped: their existence
//     is conveyed by the PK column, so listing them in INDEXES adds
//     nothing.
//   - UC-backing pairing is column-set based, not name based, so
//     SQLite's auto-named sqlite_autoindex_* entries match cleanly.
//   - At most one matching unique index is bound per UC. A schema
//     may legitimately carry multiple unique indexes over the same
//     columns (e.g. a UNIQUE constraint plus a manually-created
//     CREATE UNIQUE INDEX); only the one acting as the constraint's
//     backing gets backing=true. The rest stay as plain entries.
//   - A user-declared unique index whose columns don't match any
//     declared UC — e.g. CREATE UNIQUE INDEX foo ON t(x) without a
//     matching CONSTRAINT bar UNIQUE (x) — gets backing=false.
//
// JSON / YAML output keeps the full [Table.Indexes] slice unmodified;
// the muting and PK filtering are opinionated to the text renderer.
//
// Order across indexes follows their order in tbl.Indexes.
func indexEntriesByColumn(tbl *metadata.Table) map[string][]indexEntry {
	if len(tbl.Indexes) == 0 {
		return nil
	}

	// \x00 as the join separator can't appear in any real SQL
	// identifier, so the keys are collision-free.
	ucColKeys := make([]string, 0, len(tbl.UniqueConstraints))
	for _, uc := range tbl.UniqueConstraints {
		if uc == nil {
			continue
		}
		ucColKeys = append(ucColKeys, strings.Join(uc.Columns, "\x00"))
	}
	ucMatched := make([]bool, len(ucColKeys))
	isUCBacking := make(map[*metadata.Index]bool)
	for _, idx := range tbl.Indexes {
		if idx == nil || idx.Primary || !idx.Unique {
			continue
		}
		k := strings.Join(idx.Columns, "\x00")
		for i, ucK := range ucColKeys {
			if ucMatched[i] || ucK != k {
				continue
			}
			isUCBacking[idx] = true
			ucMatched[i] = true
			break
		}
	}

	out := make(map[string][]indexEntry, len(tbl.Columns))
	for _, idx := range tbl.Indexes {
		if idx == nil || idx.Primary {
			continue
		}
		entry := indexEntry{name: idx.Name, backing: isUCBacking[idx]}
		for _, colName := range idx.Columns {
			out[colName] = append(out[colName], entry)
		}
	}
	return out
}

// uniqueNamesByColumn returns a column-name → []constraintName lookup
// for tbl. Composite constraints show under each member column.
func uniqueNamesByColumn(tbl *metadata.Table) map[string][]string {
	if len(tbl.UniqueConstraints) == 0 {
		return nil
	}
	out := make(map[string][]string, len(tbl.Columns))
	for _, uc := range tbl.UniqueConstraints {
		if uc == nil {
			continue
		}
		for _, colName := range uc.Columns {
			out[colName] = append(out[colName], uc.Name)
		}
	}
	return out
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
				catalog = pr.Active.Sprint(catalog)
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
			catalog = pr.Active.Sprint(catalog)
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
				s = pr.Active.Sprint(s)
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
			row[0] = pr.Active.Sprint(row[0])
			row[3] = "active"
		}
		rows = append(rows, row)
	}
	return w.tbl.appendRowsAndRenderAll(context.TODO(), rows)
}
