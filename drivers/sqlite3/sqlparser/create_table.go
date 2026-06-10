package sqlparser

import (
	"sort"

	antlr "github.com/antlr4-go/antlr/v4"

	"github.com/neilotoole/sq/drivers/sqlite3/sqlparser/sqlite"
	"github.com/neilotoole/sq/libsq/ast/antlrz"
	"github.com/neilotoole/sq/libsq/core/errz"
)

func parseCreateTableStmt(input string) (*sqlite.Create_table_stmtContext, error) {
	lex := sqlite.NewSQLiteLexer(antlr.NewInputStream(input))
	lex.RemoveErrorListeners() // the generated lexer has default listeners we don't want
	lexErrs := &antlrErrorListener{name: "lexer"}
	lex.AddErrorListener(lexErrs)

	p := sqlite.NewSQLiteParser(antlr.NewCommonTokenStream(lex, 0))
	p.RemoveErrorListeners() // the generated parser has default listeners we don't want
	parseErrs := &antlrErrorListener{name: "parser"}
	p.AddErrorListener(parseErrs)

	qCtx := p.Create_table_stmt()

	if err := lexErrs.error(); err != nil {
		return nil, errz.Err(err)
	}

	if err := parseErrs.error(); err != nil {
		return nil, errz.Err(err)
	}

	return qCtx.(*sqlite.Create_table_stmtContext), nil
}

// TableIdent holds an extracted table identifier from a CREATE TABLE
// statement, including the byte offsets of the schema and table tokens
// in the original input.
type TableIdent struct {
	// Schema is the table's schema, with any of SQLite's four legal
	// identifier-quoting styles (double-quote, single-quote, backtick,
	// square brackets) stripped. Empty if no schema part was present.
	Schema string

	// Table is the table name, with any of SQLite's four legal
	// identifier-quoting styles stripped. Always non-empty.
	Table string

	// RawSchema is the raw text of the schema token as it appeared in the
	// input, preserving any surrounding quotes. Empty if no schema part was
	// present.
	RawSchema string

	// RawTable is the raw text of the table token as it appeared in the
	// input, preserving any surrounding quotes.
	RawTable string

	// SchemaOffset is the byte offset of RawSchema in the input.
	// -1 if no schema part was present.
	SchemaOffset int

	// TableOffset is the byte offset of RawTable in the input.
	TableOffset int
}

// ExtractTableIdentFromCreateTableStmt extracts the table identifier
// (schema, table, and the byte offsets of each token in the input) from
// a CREATE TABLE statement.
//
//	CREATE TABLE "sakila"."actor" ( actor_id INTEGER NOT NULL)
//	--> &TableIdent{Schema:"sakila", Table:"actor", RawSchema:`"sakila"`,
//	                RawTable:`"actor"`, SchemaOffset:13, TableOffset:22}
//
// Returns an error if no table identifier can be extracted.
func ExtractTableIdentFromCreateTableStmt(stmt string) (*TableIdent, error) {
	stmtCtx, err := parseCreateTableStmt(stmt)
	if err != nil {
		return nil, err
	}

	tokx := antlrz.NewTokenExtractor(stmt)
	ident := &TableIdent{SchemaOffset: -1}

	if n, ok := stmtCtx.Schema_name().(*sqlite.Schema_nameContext); ok {
		if n.Any_name() != nil && !n.Any_name().IsEmpty() && n.Any_name().IDENTIFIER() != nil {
			ident.RawSchema = tokx.Extract(n)
			ident.Schema = trimIdentQuotes(ident.RawSchema)
			ident.SchemaOffset, _ = tokx.Offset(n)
		}
	}

	if x, ok := stmtCtx.Table_name().(*sqlite.Table_nameContext); ok {
		if x.Any_name() != nil && !x.Any_name().IsEmpty() && x.Any_name().IDENTIFIER() != nil {
			ident.RawTable = tokx.Extract(x)
			ident.Table = trimIdentQuotes(ident.RawTable)
			ident.TableOffset, _ = tokx.Offset(x)
		}
	}

	if ident.Table == "" {
		return nil, errz.Errorf("failed to extract table name from CREATE TABLE statement")
	}

	return ident, nil
}

// ForeignTableRef describes a single REFERENCES <table> occurrence inside a
// CREATE TABLE statement's foreign-key clauses, with the byte offset of the
// table token in the original input. Both the column-constraint form
// (`col INTEGER REFERENCES other(id)`) and the table-constraint form
// (`FOREIGN KEY(col) REFERENCES other(id)`) funnel through the same
// foreign_table grammar rule, so both produce ForeignTableRef entries.
type ForeignTableRef struct {
	// RawTable is the raw text of the referenced table token as it appeared
	// in the input, preserving any of SQLite's four legal identifier-quote
	// styles (double-quote, single-quote, backtick, square brackets).
	RawTable string

	// Table is the referenced table name with quotes stripped. Always
	// non-empty.
	Table string

	// TableOffset is the byte offset of RawTable in the input. The token
	// ends at TableOffset+len(RawTable).
	TableOffset int
}

// ExtractForeignTableRefsFromCreateTableStmt returns one ForeignTableRef per
// REFERENCES <table> occurrence in a CREATE TABLE statement. Refs are
// returned in source order. An empty slice (not an error) is returned when
// the statement has no foreign-key clauses.
func ExtractForeignTableRefsFromCreateTableStmt(stmt string) ([]ForeignTableRef, error) {
	stmtCtx, err := parseCreateTableStmt(stmt)
	if err != nil {
		return nil, err
	}

	tokx := antlrz.NewTokenExtractor(stmt)
	var refs []ForeignTableRef
	collectForeignTables(stmtCtx, tokx, &refs)
	return refs, nil
}

// collectForeignTables descends node depth-first, appending one
// ForeignTableRef per Foreign_tableContext encountered. The walk stops
// descending into a foreign_table once found, since its only child is the
// table-name any_name (a leaf for our purposes).
func collectForeignTables(node antlr.Tree, tokx *antlrz.TokenExtractor, out *[]ForeignTableRef) {
	if ft, ok := node.(*sqlite.Foreign_tableContext); ok {
		raw := tokx.Extract(ft)
		if raw == "" {
			return
		}
		offset, _ := tokx.Offset(ft)
		*out = append(*out, ForeignTableRef{
			RawTable:    raw,
			Table:       trimIdentQuotes(raw),
			TableOffset: offset,
		})
		return
	}
	for _, child := range node.GetChildren() {
		collectForeignTables(child, tokx, out)
	}
}

// ExtractCreateTableStmtColDefs extracts the column definitions from a CREATE
// TABLE statement.
func ExtractCreateTableStmtColDefs(stmt string) ([]*ColDef, error) {
	stmtCtx, err := parseCreateTableStmt(stmt)
	if err != nil {
		return nil, err
	}

	var colDefs []*ColDef

	tokx := antlrz.NewTokenExtractor(stmt)
	for _, child := range stmtCtx.GetChildren() {
		if defCtx, ok := child.(*sqlite.Column_defContext); ok {
			if defCtx == nil || defCtx.Column_name() == nil {
				// Shouldn't happen
				continue
			}

			if defCtx.Type_name() == nil || defCtx.Type_name().GetText() == "" {
				// Shouldn't happen
				continue
			}

			colDef := &ColDef{
				DefCtx:  defCtx,
				Raw:     tokx.Extract(defCtx),
				RawName: tokx.Extract(defCtx.Column_name()),
				Name:    trimIdentQuotes(defCtx.Column_name().GetText()),
				RawType: tokx.Extract(defCtx.Type_name()),
				Type:    defCtx.Type_name().GetText(),
			}

			colDef.InputOffset, _ = tokx.Offset(defCtx)
			colDef.RawNameOffset, _ = tokx.Offset(defCtx.Column_name())
			colDef.RawTypeOffset, _ = tokx.Offset(defCtx.Type_name())

			colDefs = append(colDefs, colDef)
		}
	}

	return colDefs, nil
}

// ColDef represents a column definition in a CREATE TABLE statement.
type ColDef struct {
	// DefCtx is the antlr context for the column definition.
	DefCtx *sqlite.Column_defContext

	// Raw is the raw text of the entire column definition.
	Raw string

	// RawName is the raw text of the column name as it appeared in the input.
	// It may be quoted using any of SQLite's four legal identifier-quoting
	// styles: double-quote, single-quote, backtick, or square brackets.
	RawName string

	// Name is the column name, stripped of any of SQLite's four legal
	// identifier-quoting styles: double-quote ("name"), single-quote ('name'),
	// backtick (`name`), and square brackets ([name]).
	Name string

	// RawType is the raw text of the column type as it appeared in the input.
	RawType string

	// Type is the canonicalized column type.
	Type string

	// InputOffset is the byte offset of the column definition (Raw) in the
	// input. The def ends at InputOffset+len(Raw).
	InputOffset int

	// RawNameOffset is the byte offset of RawName in the input. The name
	// token ends at RawNameOffset+len(RawName). Used by drivers to splice
	// rewrites at exact positions without relying on substring matching.
	RawNameOffset int

	// RawTypeOffset is the byte offset of RawType in the input. The type
	// token ends at RawTypeOffset+len(RawType). Used by drivers to splice
	// rewrites at exact positions without relying on substring matching.
	RawTypeOffset int
}

// String returns the raw text of the column definition.
func (cd *ColDef) String() string {
	return cd.Raw
}

// Edit describes a single non-overlapping byte-range replacement within an
// input string. Used by ApplyEdits to splice multiple parser-derived
// rewrites into a CREATE TABLE statement.
type Edit struct {
	// Replacement is the text to splice in place of input[Start:End].
	Replacement string

	// Start is the byte offset where the replacement begins (inclusive).
	Start int

	// End is the byte offset where the replacement ends (exclusive).
	// End must be >= Start.
	End int
}

// ApplyEdits returns input with each edit's [Start:End) range replaced by
// its Replacement. Edits may be supplied in any order; they are applied in
// reverse start order so pre-edit offsets remain valid throughout.
//
// Returns an error if any edit has End < Start, any range falls outside
// [0, len(input)], two edits overlap, or two edits share the same Start
// (which would make the relative apply order ambiguous when one or both
// is an insertion, since sort.Slice is not stable on equal Start values).
func ApplyEdits(input string, edits []Edit) (string, error) {
	if len(edits) == 0 {
		return input, nil
	}

	sorted := make([]Edit, len(edits))
	copy(sorted, edits)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start < sorted[j].Start })

	for i, e := range sorted {
		if e.Start < 0 || e.End > len(input) || e.End < e.Start {
			return "", errz.Errorf("sqlparser: ApplyEdits: edit out of range [%d:%d) for input length %d",
				e.Start, e.End, len(input))
		}
		if i == 0 {
			continue
		}
		prev := sorted[i-1]
		if e.Start < prev.End {
			return "", errz.Errorf("sqlparser: ApplyEdits: edits overlap at byte offset %d", e.Start)
		}
		if e.Start == prev.Start {
			return "", errz.Errorf("sqlparser: ApplyEdits: two edits share Start=%d (ambiguous when one is an insertion)",
				e.Start)
		}
	}

	out := input
	for i := len(sorted) - 1; i >= 0; i-- {
		e := sorted[i]
		out = out[:e.Start] + e.Replacement + out[e.End:]
	}
	return out, nil
}
