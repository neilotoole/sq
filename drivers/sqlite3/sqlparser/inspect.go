package sqlparser

import (
	"strings"

	antlr "github.com/antlr4-go/antlr/v4"

	"github.com/neilotoole/sq/drivers/sqlite3/sqlparser/sqlite"
	"github.com/neilotoole/sq/libsq/ast/antlrz"
)

// This file implements DDL extraction used by the sqlite3 (and rqlite)
// driver's inspect metadata path. SQLite has no catalog views for CHECK
// constraints or triggers: the structured detail lives only in the
// CREATE DDL text stored in sqlite_master.sql, so these functions parse
// that DDL with the vendored ANTLR grammar.

// CheckClause is a single CHECK constraint extracted from a CREATE TABLE
// statement. SQLite rarely names CHECK constraints, so Name is usually
// empty (it is only set when the DDL uses the CONSTRAINT <name> CHECK
// form). Clause is the raw expression text as it appeared between the
// CHECK parentheses, e.g. "price > 0".
type CheckClause struct {
	// Name is the constraint name, or empty if the CHECK was declared
	// without a CONSTRAINT <name> prefix (the common case in SQLite).
	Name string

	// Clause is the raw CHECK expression text, with quotes/whitespace
	// preserved as in the source DDL. It does not include the enclosing
	// parentheses or the CHECK keyword.
	Clause string
}

// ExtractCheckConstraints returns one CheckClause per CHECK constraint
// declared in a CREATE TABLE statement, in source order. Both column-level
// (`price INTEGER CHECK (price > 0)`) and table-level
// (`CHECK (price > discount)`) forms are returned. A nil slice (and nil
// error) is returned when the statement has no CHECK constraints.
//
// Returns an error if stmt cannot be parsed as a CREATE TABLE statement;
// callers should fall back to leaving the structured Clause empty and
// keeping any raw DDL.
func ExtractCheckConstraints(stmt string) ([]CheckClause, error) {
	stmtCtx, err := parseCreateTableStmt(stmt)
	if err != nil {
		return nil, err
	}

	tokx := antlrz.NewTokenExtractor(stmt)
	var checks []CheckClause
	collectCheckClauses(stmtCtx, tokx, &checks)
	return checks, nil
}

// collectCheckClauses descends node depth-first, appending one CheckClause
// per column- or table-constraint context that declares a CHECK.
func collectCheckClauses(node antlr.Tree, tokx *antlrz.TokenExtractor, out *[]CheckClause) {
	switch ctx := node.(type) {
	case *sqlite.Column_constraintContext:
		if ctx.CHECK_() != nil && ctx.Expr() != nil {
			*out = append(*out, CheckClause{
				Name:   constraintName(ctx.CONSTRAINT_(), ctx.Name()),
				Clause: strings.TrimSpace(tokx.Extract(ctx.Expr())),
			})
		}
		// A column_constraint has no nested constraint contexts worth
		// descending into for CHECK purposes.
		return
	case *sqlite.Table_constraintContext:
		if ctx.CHECK_() != nil && ctx.Expr() != nil {
			*out = append(*out, CheckClause{
				Name:   constraintName(ctx.CONSTRAINT_(), ctx.Name()),
				Clause: strings.TrimSpace(tokx.Extract(ctx.Expr())),
			})
		}
		return
	}

	for _, child := range node.GetChildren() {
		collectCheckClauses(child, tokx, out)
	}
}

// constraintName returns the trimmed constraint name when both the
// CONSTRAINT keyword and a name context are present, else "".
func constraintName(kw antlr.TerminalNode, name sqlite.INameContext) string {
	if kw == nil || name == nil || name.IsEmpty() {
		return ""
	}
	return trimIdentQuotes(name.GetText())
}

// ExtractTriggerTimingEvents parses a CREATE TRIGGER statement and returns
// its timing ("BEFORE", "AFTER", or "INSTEAD OF"; empty if none was
// declared) and its firing events (a subset of "INSERT", "UPDATE",
// "DELETE"). A SQLite trigger fires on exactly one event, so events
// usually has a single element, but the slice form matches the
// cross-driver metadata model.
//
// Returns an error if stmt cannot be parsed as a CREATE TRIGGER statement;
// callers should fall back to leaving Timing/Events empty and keeping the
// raw trigger DDL.
func ExtractTriggerTimingEvents(stmt string) (timing string, events []string, err error) {
	stmtCtx, err := parseCreateTriggerStmt(stmt)
	if err != nil {
		return "", nil, err
	}

	switch {
	case stmtCtx.INSTEAD_() != nil:
		timing = "INSTEAD OF"
	case stmtCtx.BEFORE_() != nil:
		timing = "BEFORE"
	case stmtCtx.AFTER_() != nil:
		timing = "AFTER"
	}

	// The DELETE_/INSERT_/UPDATE_ terminals on the trigger statement are
	// the event keyword that precedes ON <table>; body statements live in
	// nested delete/insert/update_stmt contexts, so these don't collide.
	if stmtCtx.INSERT_() != nil {
		events = append(events, "INSERT")
	}
	if stmtCtx.UPDATE_() != nil {
		events = append(events, "UPDATE")
	}
	if stmtCtx.DELETE_() != nil {
		events = append(events, "DELETE")
	}

	return timing, events, nil
}

// ColumnDDLInfo carries the best-effort, DDL-only column attributes that
// SQLite does not expose via pragma: whether a column is declared
// INTEGER PRIMARY KEY AUTOINCREMENT, and the generated-column expression
// text. The generated flag itself is sourced from pragma_table_xinfo by
// the driver; only the expression requires the DDL.
type ColumnDDLInfo struct {
	// GeneratedExpr is the raw expression text of a GENERATED ALWAYS AS
	// (expr) / AS (expr) column, without the enclosing parentheses. Empty
	// for non-generated columns.
	GeneratedExpr string

	// AutoIncrement reports whether the column carries the AUTOINCREMENT
	// keyword (only legal on an INTEGER PRIMARY KEY column).
	AutoIncrement bool
}

// ExtractColumnDDLInfo parses a CREATE TABLE statement and returns a map
// keyed by unquoted column name carrying the best-effort, DDL-only column
// attributes (AUTOINCREMENT and generated-column expression). Columns with
// no such attributes are omitted from the map.
//
// Returns an error if stmt cannot be parsed; callers should fall back to
// leaving these fields at their zero values.
func ExtractColumnDDLInfo(stmt string) (map[string]ColumnDDLInfo, error) {
	stmtCtx, err := parseCreateTableStmt(stmt)
	if err != nil {
		return nil, err
	}

	tokx := antlrz.NewTokenExtractor(stmt)
	out := map[string]ColumnDDLInfo{}

	for _, child := range stmtCtx.GetChildren() {
		defCtx, ok := child.(*sqlite.Column_defContext)
		if !ok || defCtx.Column_name() == nil {
			continue
		}
		name := trimIdentQuotes(defCtx.Column_name().GetText())
		if name == "" {
			continue
		}

		var info ColumnDDLInfo
		for _, cc := range defCtx.AllColumn_constraint() {
			ccCtx, ok := cc.(*sqlite.Column_constraintContext)
			if !ok {
				continue
			}
			if ccCtx.AUTOINCREMENT_() != nil {
				info.AutoIncrement = true
			}
			// A generated column uses [GENERATED ALWAYS] AS (expr); the
			// AS keyword with an expression is the reliable marker.
			if ccCtx.AS_() != nil && ccCtx.Expr() != nil {
				info.GeneratedExpr = strings.TrimSpace(tokx.Extract(ccCtx.Expr()))
			}
		}

		if info.AutoIncrement || info.GeneratedExpr != "" {
			out[name] = info
		}
	}

	return out, nil
}
