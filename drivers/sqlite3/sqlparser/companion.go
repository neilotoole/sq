package sqlparser

import (
	"strings"

	antlr "github.com/antlr4-go/antlr/v4"

	"github.com/neilotoole/sq/drivers/sqlite3/sqlparser/sqlite"
	"github.com/neilotoole/sq/libsq/ast/antlrz"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// This file implements DDL rewrites for a table's companion objects:
// the indexes and triggers that accompany a table as separate
// sqlite_master rows. When a table is copied, its companion DDL must be
// re-targeted at the copy: the companion's own name must change (index
// and trigger names are schema-global in SQLite) and its table
// references must point at the destination table.

func parseCreateIndexStmt(input string) (*sqlite.Create_index_stmtContext, error) {
	lex := sqlite.NewSQLiteLexer(antlr.NewInputStream(input))
	lex.RemoveErrorListeners() // the generated lexer has default listeners we don't want
	lexErrs := &antlrErrorListener{name: "lexer"}
	lex.AddErrorListener(lexErrs)

	p := sqlite.NewSQLiteParser(antlr.NewCommonTokenStream(lex, 0))
	p.RemoveErrorListeners() // the generated parser has default listeners we don't want
	parseErrs := &antlrErrorListener{name: "parser"}
	p.AddErrorListener(parseErrs)

	qCtx := p.Create_index_stmt()

	if err := lexErrs.error(); err != nil {
		return nil, errz.Err(err)
	}

	if err := parseErrs.error(); err != nil {
		return nil, errz.Err(err)
	}

	return qCtx.(*sqlite.Create_index_stmtContext), nil
}

func parseCreateTriggerStmt(input string) (*sqlite.Create_trigger_stmtContext, error) {
	lex := sqlite.NewSQLiteLexer(antlr.NewInputStream(input))
	lex.RemoveErrorListeners() // the generated lexer has default listeners we don't want
	lexErrs := &antlrErrorListener{name: "lexer"}
	lex.AddErrorListener(lexErrs)

	p := sqlite.NewSQLiteParser(antlr.NewCommonTokenStream(lex, 0))
	p.RemoveErrorListeners() // the generated parser has default listeners we don't want
	parseErrs := &antlrErrorListener{name: "parser"}
	p.AddErrorListener(parseErrs)

	qCtx := p.Create_trigger_stmt()

	if err := lexErrs.error(); err != nil {
		return nil, errz.Err(err)
	}

	if err := parseErrs.error(); err != nil {
		return nil, errz.Err(err)
	}

	return qCtx.(*sqlite.Create_trigger_stmtContext), nil
}

// RewriteCreateIndexStmt rewrites a CREATE INDEX statement (as read from
// sqlite_master) so that it applies to a different table. The index's own
// identifier, including any schema qualifier, is replaced with
// newIndexIdent, and the ON <table> target is replaced with
// newTableIdent.
//
// Both replacement arguments must be pre-rendered SQL identifier text
// (i.e. already quoted as needed). newIndexIdent may carry a schema
// qualifier (`"sch"."idx_copy"`); newTableIdent must be a bare table
// identifier because SQLite's CREATE INDEX ... ON clause does not accept
// a schema qualifier (the table is resolved in the index's own schema).
//
// Column references in the indexed-column list and in a partial index's
// WHERE clause are left untouched: SQLite does not permit table-qualified
// references there, so they cannot name the source table.
func RewriteCreateIndexStmt(stmt, newIndexIdent, newTableIdent string) (string, error) {
	stmtCtx, err := parseCreateIndexStmt(stmt)
	if err != nil {
		return "", err
	}

	tokx := antlrz.NewTokenExtractor(stmt)

	nameCtx, ok := stmtCtx.Index_name().(*sqlite.Index_nameContext)
	if !ok || !anyNameIsExtractable(nameCtx.Any_name()) {
		return "", errz.Errorf("failed to extract index name from CREATE INDEX statement")
	}
	nameOffset, _ := tokx.Offset(nameCtx)
	nameEnd := nameOffset + len(tokx.Extract(nameCtx))
	nameStart := nameOffset
	if sn, snOK := stmtCtx.Schema_name().(*sqlite.Schema_nameContext); snOK && anyNameIsExtractable(sn.Any_name()) {
		nameStart, _ = tokx.Offset(sn)
	}

	tblCtx, ok := stmtCtx.Table_name().(*sqlite.Table_nameContext)
	if !ok || !anyNameIsExtractable(tblCtx.Any_name()) {
		return "", errz.Errorf("failed to extract table name from CREATE INDEX statement")
	}
	tblOffset, _ := tokx.Offset(tblCtx)
	tblRaw := tokx.Extract(tblCtx)

	edits := []Edit{
		{Start: nameStart, End: nameEnd, Replacement: newIndexIdent},
		{Start: tblOffset, End: tblOffset + len(tblRaw), Replacement: newTableIdent},
	}
	return ApplyEdits(stmt, edits)
}

// RewriteCreateTriggerStmt rewrites a CREATE TRIGGER statement (as read
// from sqlite_master) so that it applies to a different table. The
// trigger's own identifier, including any schema qualifier, is replaced
// with newTriggerIdent. The ON <table> target, and every table-name
// position in the trigger body that names the same table
// (case-insensitively), are replaced with newTableIdent; body references
// to other tables are left untouched. This mirrors the self-FK rewrite
// semantics of CopyTable: the copied trigger operates on the copied
// table, while cross-table references (e.g. inserts into an audit table)
// continue to point at their original targets.
//
// Both replacement arguments must be pre-rendered SQL identifier text
// (i.e. already quoted as needed). newTriggerIdent may carry a schema
// qualifier (`"sch"."trg_copy"`); newTableIdent must be a bare table
// identifier because SQLite's CREATE TRIGGER ... ON clause does not
// accept a schema qualifier (the table is resolved in the trigger's own
// schema).
func RewriteCreateTriggerStmt(stmt, newTriggerIdent, newTableIdent string) (string, error) {
	stmtCtx, err := parseCreateTriggerStmt(stmt)
	if err != nil {
		return "", err
	}

	tokx := antlrz.NewTokenExtractor(stmt)

	nameCtx, ok := stmtCtx.Trigger_name().(*sqlite.Trigger_nameContext)
	if !ok || !anyNameIsExtractable(nameCtx.Any_name()) {
		return "", errz.Errorf("failed to extract trigger name from CREATE TRIGGER statement")
	}
	nameOffset, _ := tokx.Offset(nameCtx)
	nameEnd := nameOffset + len(tokx.Extract(nameCtx))
	nameStart := nameOffset
	if sn, snOK := stmtCtx.Schema_name().(*sqlite.Schema_nameContext); snOK && anyNameIsExtractable(sn.Any_name()) {
		nameStart, _ = tokx.Offset(sn)
	}

	// The grammar's create_trigger_stmt rule has exactly one direct
	// table_name child: the ON <table> target. Body table references are
	// nested inside the body's update/insert/delete/select statements.
	onTblCtx, ok := stmtCtx.Table_name().(*sqlite.Table_nameContext)
	if !ok || !anyNameIsExtractable(onTblCtx.Any_name()) {
		return "", errz.Errorf("failed to extract table name from CREATE TRIGGER statement")
	}
	srcTable := trimIdentQuotes(tokx.Extract(onTblCtx))

	edits := []Edit{{Start: nameStart, End: nameEnd, Replacement: newTriggerIdent}}

	// Walk the whole statement (the ON target included) and rewrite every
	// table-name position matching the trigger's own table.
	collectMatchingTableNameEdits(stmtCtx, tokx, srcTable, newTableIdent, &edits)

	return ApplyEdits(stmt, edits)
}

// collectMatchingTableNameEdits descends node depth-first, appending one
// Edit per Table_nameContext whose unquoted text equals match
// (case-insensitively, consistent with SQLite's identifier resolution).
// The walk does not descend into a matched Table_nameContext: its only
// child is the name's any_name, and re-entering would risk a
// double-record.
func collectMatchingTableNameEdits(node antlr.Tree, tokx *antlrz.TokenExtractor,
	match, replacement string, edits *[]Edit,
) {
	if tn, ok := node.(*sqlite.Table_nameContext); ok {
		raw := tokx.Extract(tn)
		if strings.EqualFold(trimIdentQuotes(raw), match) {
			offset, _ := tokx.Offset(tn)
			*edits = append(*edits, Edit{
				Start:       offset,
				End:         offset + len(raw),
				Replacement: replacement,
			})
		}
		return
	}
	for _, child := range node.GetChildren() {
		collectMatchingTableNameEdits(child, tokx, match, replacement, edits)
	}
}
