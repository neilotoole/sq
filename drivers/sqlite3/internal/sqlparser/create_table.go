package sqlparser

import (
	"slices"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/neilotoole/sq/drivers/sqlite3/internal/sqlparser/sqlite"
	"github.com/neilotoole/sq/libsq/ast/antlrz"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
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

// ExtractTableIdentFromCreateTableStmt extracts table name (and the
// table's schema if specified) from a CREATE TABLE statement.
// If err is nil, table is guaranteed to be non-empty. If arg unescape is
// true, any surrounding quotation chars are trimmed from the returned values.
//
//	CREATE TABLE "sakila"."actor" ( actor_id INTEGER NOT NULL)  -->  sakila, actor, nil
func ExtractTableIdentFromCreateTableStmt(stmt string, unescape bool) (schema, table string, err error) {
	stmtCtx, err := parseCreateTableStmt(stmt)
	if err != nil {
		return "", "", err
	}

	if n, ok := stmtCtx.Schema_name().(*sqlite.Schema_nameContext); ok {
		if n.Any_name() != nil && !n.Any_name().IsEmpty() && n.Any_name().IDENTIFIER() != nil {
			schema = n.Any_name().IDENTIFIER().GetText()
			if unescape {
				schema = trimIdentQuotes(schema)
			}
		}
	}

	if x, ok := stmtCtx.Table_name().(*sqlite.Table_nameContext); ok {
		if x.Any_name() != nil && !x.Any_name().IsEmpty() && x.Any_name().IDENTIFIER() != nil {
			table = x.Any_name().IDENTIFIER().GetText()
			if unescape {
				table = trimIdentQuotes(table)
			}
		}
	}

	if table == "" {
		return "", "", errz.Errorf("failed to extract table name from CREATE TABLE statement")
	}

	return schema, table, nil
}

// ExtractColNameAndTypeFromCreateStmt extracts the column name and type from
// a CREATE TABLE statement. If the column name is quoted in the original, it
// is returned quote. If the column name is not found, an error is returned.
func ExtractColNameAndTypeFromCreateStmt(stmt, colName string) (name, typ string, err error) {
	stmtCtx, err := parseCreateTableStmt(stmt)
	if err != nil {
		return "", "", err
	}

	for _, child := range stmtCtx.GetChildren() {
		if colDef, ok := child.(*sqlite.Column_defContext); ok {
			if colDef == nil || colDef.Column_name() == nil {
				continue
			}

			gotColNameText := colDef.Column_name().GetText()
			if gotColNameText == colName || gotColNameText == stringz.DoubleQuote(colName) {
				if colDef.Type_name() == nil {
					continue
				}
				return gotColNameText, colDef.Type_name().GetText(), nil
			}
		}
	}

	return "", "", errz.Errorf("sqlite: column {%s} not found in stmt", colName)
}

func ExtractColNamesAndTypesFromCreateStmt(stmt string) (names, types []string, err error) {
	stmtCtx, err := parseCreateTableStmt(stmt)
	if err != nil {
		return nil, nil, err
	}

	tokEx := antlrz.NewTokenExtractor(stmt)
	for _, child := range stmtCtx.GetChildren() {
		if colDef, ok := child.(*sqlite.Column_defContext); ok {
			if colDef == nil || colDef.Column_name() == nil {
				continue
			}

			if colDef.Type_name() == nil || colDef.Type_name().GetText() == "" {
				continue
			}

			raw := tokEx.Extract(colDef)
			_ = raw

			names = append(names, colDef.Column_name().GetText())
			types = append(types, colDef.Type_name().GetText())
		}
	}

	return names, types, nil
}

func ExtractCreateStmtColDefs(stmt string) ([]*ColDef, error) {
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
				Name:    stringz.StripDoubleQuote(defCtx.Column_name().GetText()),
				RawType: tokx.Extract(defCtx.Type_name()),
				Type:    defCtx.Type_name().GetText(),
			}

			colDef.InputOffset, _ = tokx.Offset(defCtx)

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
	// It may be double-quoted.
	RawName string

	// Name is the column name, stripped of any double-quotes.
	Name string

	// RawType is the raw text of the column type as it appeared in the input.
	RawType string

	// Type is the canonicalized column type.
	Type string

	// InputOffset is the character start index of the column definition in the
	// input. The def ends at InputOffset+len(Raw).
	InputOffset int
}

// String returns the raw text of the column definition.
func (cd *ColDef) String() string {
	return cd.Raw
}

func CanonicalizeCreateStmtColNames(stmt string) (string, error) {
	colNames, _, err := ExtractColNamesAndTypesFromCreateStmt(stmt)
	if err != nil {
		return "", err
	}

	// Sort in order of length.
	slices.SortFunc(colNames, func(i, j string) int {
		switch {
		case len(i) == len(j):
			return 0
		case len(i) < len(j):
			return 1
		default:
			return -1
		}
	})

	for _, colName := range colNames {
		canonicalColName := colName
		if canonicalColName[0] != '"' {
			canonicalColName = stringz.DoubleQuote(canonicalColName)
		}
		switch {
		case strings.Contains(stmt, colName+" "):
			stmt = strings.Replace(stmt, colName+" ", canonicalColName+" ", 1)
		case strings.Contains(stmt, colName+"\n"):
			stmt = strings.Replace(stmt, colName+"\n", canonicalColName+"\n", 1)
		case strings.Contains(stmt, colName+"\r\n"):
			stmt = strings.Replace(stmt, colName+"\r\n", canonicalColName+"\r\n", 1)
		case strings.Contains(stmt, colName+"\t"):
			stmt = strings.Replace(stmt, colName+"\t", canonicalColName+"\t", 1)
		}
	}

	return stmt, nil
}
