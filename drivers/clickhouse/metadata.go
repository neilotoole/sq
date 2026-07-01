package clickhouse

import (
	"context"
	"database/sql"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// getSourceMetadata returns metadata for the ClickHouse source, including
// database version, current database name, size, user, and optionally
// table/column metadata.
//
// Parameters:
//   - src: The source configuration
//   - db: The database connection
//   - noSchema: If true, only returns basic metadata without table details
//
// The returned metadata includes:
//   - Handle: The source handle (e.g., "@mydb")
//   - Location: The connection string
//   - Driver/DBDriver: drivertype.ClickHouse
//   - DBVersion: ClickHouse server version from version()
//   - DBProduct: "ClickHouse" plus version string
//   - Schema/Catalog/Name/FQName: Current database from currentDatabase().
//     ClickHouse uses a single "database" concept that maps to both schema
//     and catalog in sq's model.
//   - User: Current user from currentUser()
//   - Size: Total database size from system.tables
//   - Tables: Table metadata with TableCount/ViewCount (if noSchema is false)
func getSourceMetadata(ctx context.Context, src *source.Source, db sqlz.DB, noSchema bool) (*metadata.Source, error) {
	md := &metadata.Source{
		Handle:   src.Handle,
		Location: src.Location,
		Driver:   drivertype.ClickHouse,
		DBDriver: drivertype.ClickHouse,
	}

	// Get database version, current database, current user in one query.
	var version, database, user string
	err := db.QueryRowContext(ctx,
		"SELECT version(), currentDatabase(), currentUser()").
		Scan(&version, &database, &user)
	if err != nil {
		return nil, errw(err)
	}

	md.DBVersion = version
	md.DBProduct = "ClickHouse " + version
	md.Schema = database
	md.Catalog = database
	md.Name = database
	md.FQName = database
	md.User = user

	// Get database size.
	var size sql.NullInt64
	err = db.QueryRowContext(ctx,
		"SELECT SUM(total_bytes) FROM system.tables WHERE database = ?",
		database).Scan(&size)
	if err != nil {
		return nil, errw(err)
	}
	if size.Valid {
		md.Size = &size.Int64
	}

	// DBProperties surfaces driver-level session/version values via the
	// shared SQLDriver helper.
	md.DBProperties, err = getDBProperties(ctx, db)
	if err != nil {
		return nil, err
	}

	if noSchema {
		return md, nil
	}

	md.Tables, err = getTablesMetadata(ctx, db, database)
	if err != nil {
		return nil, err
	}

	log := lg.FromContext(ctx)

	allChecks, err := getClickHouseCheckConstraints(ctx, db, database, "")
	if err != nil {
		return nil, err
	}
	metadata.AssignCheckConstraints(log, md.Tables, allChecks)

	allViewDefs, err := getClickHouseViewDefinitions(ctx, db, database, "")
	if err != nil {
		return nil, err
	}
	for _, tbl := range md.Tables {
		if tbl.TableType == sqlz.TableTypeView {
			tbl.ViewDefinition = allViewDefs[tbl.Name]
		}
	}

	md.RecomputeTableCounts()

	return md, nil
}

// getTablesMetadata returns metadata for all tables and views in the specified
// database by querying the system.tables catalog table.
//
// For each table/view, it retrieves:
//   - name: Table name
//   - engine: ClickHouse engine type (MergeTree, View, MaterializedView, etc.)
//   - total_rows: Row count (may be approximate for some engines)
//   - total_bytes: Storage size in bytes
//
// Views (engine "View" or "MaterializedView") are included with TableType set
// to sqlz.TableTypeView. All other engines are considered tables.
//
// If column metadata retrieval fails for a table, a warning is logged and the
// table is skipped (not included in the result).
func getTablesMetadata(ctx context.Context, db sqlz.DB, dbName string) ([]*metadata.Table, error) {
	const query = `
		SELECT
			name,
			engine,
			total_rows,
			total_bytes
		FROM system.tables
		WHERE database = ?
		ORDER BY name
	`

	rows, err := db.QueryContext(ctx, query, dbName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	var tables []*metadata.Table
	for rows.Next() {
		var tblName, engine string
		var totalRows, totalBytes sql.NullInt64

		if err = rows.Scan(&tblName, &engine, &totalRows, &totalBytes); err != nil {
			return nil, errw(err)
		}

		tblMeta := &metadata.Table{
			Name:        tblName,
			FQName:      dbName + "." + tblName,
			DBTableType: engine,
			TableType:   tableTypeFromEngine(engine),
			RowCount:    totalRows.Int64,
		}

		if totalBytes.Valid {
			bytes := totalBytes.Int64
			tblMeta.Size = &bytes
		}

		// Get column metadata for this table.
		cols, colErr := getColumnsMetadata(ctx, db, dbName, tblName)
		if colErr != nil {
			return nil, errz.Wrapf(colErr, "get column metadata for %s.%s", dbName, tblName)
		}
		tblMeta.Columns = cols

		tables = append(tables, tblMeta)
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return tables, nil
}

// getTableMetadata returns metadata for a specific table, including its
// columns, by querying system.tables and system.columns.
//
// Parameters:
//   - dbName: Database name. If empty, uses currentDatabase().
//   - tblName: Table name to retrieve metadata for.
//
// Returns an error if the table does not exist or cannot be queried.
func getTableMetadata(ctx context.Context, db sqlz.DB, dbName, tblName string) (*metadata.Table, error) {
	// If dbName is empty, use currentDatabase().
	if dbName == "" {
		err := db.QueryRowContext(ctx, "SELECT currentDatabase()").Scan(&dbName)
		if err != nil {
			return nil, errw(err)
		}
	}

	const queryTable = `
		SELECT
			name,
			engine,
			total_rows,
			total_bytes
		FROM system.tables
		WHERE database = ? AND name = ?
	`

	var tableName, engine string
	var totalRows, totalBytes sql.NullInt64

	err := db.QueryRowContext(ctx, queryTable, dbName, tblName).Scan(
		&tableName, &engine, &totalRows, &totalBytes,
	)
	if err != nil {
		return nil, errw(err)
	}

	tblMeta := &metadata.Table{
		Name:        tblName,
		FQName:      dbName + "." + tblName,
		DBTableType: engine,
		TableType:   tableTypeFromEngine(engine),
		RowCount:    totalRows.Int64,
	}

	if totalBytes.Valid {
		bytes := totalBytes.Int64
		tblMeta.Size = &bytes
	}

	// Get column metadata.
	cols, err := getColumnsMetadata(ctx, db, dbName, tblName)
	if err != nil {
		return nil, err
	}
	tblMeta.Columns = cols

	// Enrich with view definition or check constraints.
	if tblMeta.TableType == sqlz.TableTypeView {
		defs, defErr := getClickHouseViewDefinitions(ctx, db, dbName, tblName)
		if defErr != nil {
			return nil, defErr
		}
		tblMeta.ViewDefinition = defs[tblName]
	} else {
		checks, chkErr := getClickHouseCheckConstraints(ctx, db, dbName, tblName)
		if chkErr != nil {
			return nil, chkErr
		}
		tblMeta.CheckConstraints = checks
	}

	return tblMeta, nil
}

// getColumnsMetadata returns metadata for all columns in a table by querying
// getTableColumnNames returns the names of the table's columns in ordinal
// order, querying system.columns scoped to currentDatabase().
func getTableColumnNames(ctx context.Context, db sqlz.DB, tblName string) ([]string, error) {
	const query = `SELECT name FROM system.columns
		WHERE database = currentDatabase() AND table = ?
		ORDER BY position`

	rows, err := db.QueryContext(ctx, query, tblName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	var names []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, errw(err)
		}
		names = append(names, name)
	}
	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}
	return names, nil
}

// the system.columns catalog table.
//
// For each column, it retrieves:
//   - name: Column name
//   - type: ClickHouse type string (e.g., "String", "Nullable(Int64)")
//   - position: Column ordinal position (1-based in ClickHouse)
//   - default_kind: Type of default (e.g., "DEFAULT", "MATERIALIZED")
//   - default_expression: Default value expression
//   - comment: Column comment
//
// The Kind field is derived from the ClickHouse type using kindFromClickHouseType.
// The Nullable field is determined using isNullableType, which checks if the
// outermost type wrapper is Nullable.
//
// Note: ClickHouse doesn't have traditional primary keys. The PrimaryKey field
// is always set to false. The ORDER BY clause in MergeTree tables defines
// sort order but not uniqueness constraints.
func getColumnsMetadata(ctx context.Context, db sqlz.DB, dbName, tblName string) ([]*metadata.Column, error) {
	const query = `
		SELECT
			name,
			type,
			position,
			default_kind,
			default_expression,
			comment
		FROM system.columns
		WHERE database = ? AND table = ?
		ORDER BY position
	`

	rows, err := db.QueryContext(ctx, query, dbName, tblName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	var cols []*metadata.Column
	for rows.Next() {
		var colName, colType string
		var position int
		var defaultKind, defaultExpression, comment sql.NullString

		err = rows.Scan(&colName, &colType, &position, &defaultKind, &defaultExpression, &comment)
		if err != nil {
			return nil, errw(err)
		}

		col := &metadata.Column{
			Name:         colName,
			Position:     int64(position),
			BaseType:     baseTypeFromClickHouseType(colType),
			Kind:         kindFromClickHouseType(colType),
			ColumnType:   colType,
			DefaultValue: defaultExpression.String,
			// Use isNullableType (not isNullableTypeUnwrapped) because system.columns
			// reports the declared type directly. If a column is declared as
			// LowCardinality(Nullable(T)), that's what we get, and the column metadata
			// should reflect the declared nullability, not the unwrapped form.
			Nullable: isNullableType(colType),
			Comment:  comment.String,
		}

		// ClickHouse doesn't have traditional primary keys in the SQL sense
		// The ORDER BY clause defines sort order, not uniqueness
		col.PrimaryKey = false

		cols = append(cols, col)
	}

	return cols, errw(rows.Err())
}

// tableTypeFromEngine returns the canonical table type (sqlz.TableTypeTable or
// sqlz.TableTypeView) based on the ClickHouse engine type. Views have engine
// "View" or "MaterializedView"; all other engines are considered tables.
func tableTypeFromEngine(engine string) string {
	switch engine {
	case "View", "MaterializedView":
		return sqlz.TableTypeView
	default:
		return sqlz.TableTypeTable
	}
}

// checkConstraintRe matches the start of a ClickHouse CONSTRAINT ... CHECK
// declaration inside a CREATE TABLE DDL string. The first capture group is
// the constraint name in one of three ClickHouse identifier forms:
//   - backtick-quoted:  `my ident`
//   - double-quoted:    "my ident"
//   - unquoted ASCII:   plain_name
//
// The match ends immediately after the opening parenthesis of the CHECK
// expression; the caller walks forward from that position to extract the
// balanced-paren clause. \s* before the opening paren (rather than \s+)
// handles the rare CHECK(...) form with no space.
var checkConstraintRe = regexp.MustCompile(
	"CONSTRAINT\\s+(`(?:[^`]|``)*`|\"(?:[^\"]|\"\")*\"|\\w+)\\s+CHECK\\s*\\(",
)

// unquoteCHIdent strips surrounding backtick or double-quote delimiters from a
// ClickHouse identifier captured by checkConstraintRe, unescaping any doubled
// inner quote (a doubled backtick becomes a single backtick; "" becomes ").
// Unquoted identifiers are returned as-is.
func unquoteCHIdent(s string) string {
	if len(s) >= 2 {
		switch {
		case s[0] == '`' && s[len(s)-1] == '`':
			return strings.ReplaceAll(s[1:len(s)-1], "``", "`")
		case s[0] == '"' && s[len(s)-1] == '"':
			return strings.ReplaceAll(s[1:len(s)-1], `""`, `"`)
		}
	}
	return s
}

// getClickHouseCheckConstraints returns the CHECK constraints declared on
// ClickHouse tables in dbName. If tblName is non-empty, only constraints for
// that table are returned; passing an empty tblName returns all tables.
//
// ClickHouse has no dedicated catalog table for CHECK constraints; they are
// extracted by parsing system.tables.create_table_query with a bounded regex
// followed by a parenthesis-depth walk. Graceful: if none are found or parsing
// fails, an empty slice is returned without error.
func getClickHouseCheckConstraints(
	ctx context.Context, db sqlz.DB, dbName, tblName string,
) ([]*metadata.CheckConstraint, error) {
	log := lg.FromContext(ctx)

	query := "SELECT name, create_table_query FROM system.tables" +
		" WHERE database = ? AND engine NOT IN ('View', 'MaterializedView')"
	args := []any{dbName}
	if tblName != "" {
		query += ` AND name = ?`
		args = append(args, tblName)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var checks []*metadata.CheckConstraint
	for rows.Next() {
		var name, createQuery string
		if err = rows.Scan(&name, &createQuery); err != nil {
			return nil, errw(err)
		}
		checks = append(checks, extractClickHouseCheckConstraints(createQuery, name)...)
	}
	return checks, errw(rows.Err())
}

// extractClickHouseCheckConstraints parses a ClickHouse CREATE TABLE DDL
// string and extracts any CONSTRAINT <name> CHECK (<expr>) declarations. The
// returned constraints have their Table field set to tblName.
//
// The extractor uses a two-step approach: a regexp locates each CONSTRAINT ...
// CHECK ( header and captures the constraint name; a parenthesis-depth walk
// then extracts the balanced-paren expression. This avoids trying to match
// nested parens with a single regex.
//
// Graceful: unbalanced parentheses (malformed DDL) yield an empty clause; the
// constraint is still appended so the name is surfaced.
func extractClickHouseCheckConstraints(ddl, tblName string) []*metadata.CheckConstraint {
	matches := checkConstraintRe.FindAllStringSubmatchIndex(ddl, -1)
	if len(matches) == 0 {
		return nil
	}
	checks := make([]*metadata.CheckConstraint, 0, len(matches))
	for _, m := range matches {
		// m[2]:m[3] is the name capture group (may be backtick/double-quoted).
		name := unquoteCHIdent(ddl[m[2]:m[3]])
		// m[1] is the byte position just after the opening '(' of the CHECK
		// expression, i.e. ddl[m[1]-1] == '('. Walk forward to find the
		// matching closing paren.
		openPos := m[1] - 1
		checks = append(checks, &metadata.CheckConstraint{
			Table:  tblName,
			Name:   name,
			Clause: balancedParenContents(ddl, openPos),
		})
	}
	return checks
}

// balancedParenContents returns the text between the opening paren at position
// start and its matching closing paren, exclusive of both parens. Returns an
// empty string if start is out of range, ddl[start] != '(', or the parens are
// unbalanced.
//
// The scanner is string-literal- and identifier-quote-aware: parentheses inside
// single-quoted strings ('...'), backtick-quoted identifiers (`...`), and
// double-quoted identifiers ("...") are not counted toward the depth. Both
// SQL-standard doubled-quote escapes and backslash-escaped quotes are handled
// inside single-quoted strings; doubled backticks and "" (plus backslash
// escapes) are handled inside their respective identifier forms.
func balancedParenContents(ddl string, start int) string {
	if start < 0 || start >= len(ddl) || ddl[start] != '(' {
		return ""
	}
	depth := 0
	inSingleQuote := false
	inBacktick := false
	inDoubleQuote := false
	for i := start; i < len(ddl); i++ {
		ch := ddl[i]
		if inSingleQuote {
			switch ch {
			case '\\':
				i++ // skip backslash-escaped char (e.g. \')
			case '\'':
				// SQL-standard '' escape: two consecutive quotes stay inside string.
				if i+1 < len(ddl) && ddl[i+1] == '\'' {
					i++ // skip second quote; remain inside string
				} else {
					inSingleQuote = false
				}
			}
			continue
		}
		if inBacktick {
			switch ch {
			case '\\':
				i++ // skip backslash-escaped char (e.g. \`)
			case '`':
				// `` doubled-quote escape: two consecutive backticks stay inside.
				if i+1 < len(ddl) && ddl[i+1] == '`' {
					i++ // skip second backtick; remain inside identifier
				} else {
					inBacktick = false
				}
			}
			continue
		}
		if inDoubleQuote {
			switch ch {
			case '\\':
				i++ // skip backslash-escaped char (e.g. \")
			case '"':
				// "" doubled-quote escape: two consecutive double-quotes stay inside.
				if i+1 < len(ddl) && ddl[i+1] == '"' {
					i++ // skip second double-quote; remain inside identifier
				} else {
					inDoubleQuote = false
				}
			}
			continue
		}
		switch ch {
		case '\'':
			inSingleQuote = true
		case '`':
			inBacktick = true
		case '"':
			inDoubleQuote = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return ddl[start+1 : i]
			}
		}
	}
	return "" // unbalanced / malformed DDL
}

// getClickHouseViewDefinitions returns a map of view name -> defining SELECT
// for views in dbName. ClickHouse exposes the SELECT text directly in
// system.tables.as_select; create_table_query is used as a fallback when
// as_select is empty.
//
// If tblName is non-empty, only that view is returned. Passing an empty
// tblName returns all views and materialized views.
func getClickHouseViewDefinitions(ctx context.Context, db sqlz.DB, dbName, tblName string) (map[string]string, error) {
	log := lg.FromContext(ctx)

	query := "SELECT name, as_select, create_table_query FROM system.tables" +
		" WHERE database = ? AND engine IN ('View', 'MaterializedView')"
	args := []any{dbName}
	if tblName != "" {
		query += ` AND name = ?`
		args = append(args, tblName)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	defs := make(map[string]string)
	for rows.Next() {
		var name, asSelect, createQuery string
		if err = rows.Scan(&name, &asSelect, &createQuery); err != nil {
			return nil, errw(err)
		}
		def := asSelect
		if def == "" {
			// Fallback: extract the SELECT from the full CREATE VIEW DDL.
			def = extractViewSelectFromCHDDL(createQuery)
		}
		defs[name] = def
	}
	return defs, errw(rows.Err())
}

// reCHViewAsSelect matches the "AS SELECT" separator case-insensitively in a
// ClickHouse DDL string. The whitespace around AS is matched flexibly (one or
// more spaces, tabs, or newlines) so a DDL rendered with non-single-space
// separators is still recognized. The SELECT keyword is a capture group so
// callers slice from its start, keeping "SELECT ..." in the result. The regex
// operates on the original bytes, so the match index is a valid byte offset
// into the original string regardless of any non-ASCII characters before it.
var reCHViewAsSelect = regexp.MustCompile(`(?i)\sAS\s+(SELECT\b)`)

// reCHViewAs matches the first whitespace-delimited "AS" separator
// case-insensitively (whitespace-flexible, like reCHViewAsSelect). Used as a
// fallback when reCHViewAsSelect finds no match.
var reCHViewAs = regexp.MustCompile(`(?i)\sAS\s+`)

// extractViewSelectFromCHDDL extracts the SELECT portion from a ClickHouse
// CREATE [MATERIALIZED] VIEW DDL string. ClickHouse stores the full DDL in
// create_table_query as "CREATE [MATERIALIZED] VIEW db.name AS SELECT ...".
//
// The function first searches for the more-specific "AS SELECT" marker
// (case-insensitive) to avoid splitting on an "AS" that appears before the
// defining SELECT (e.g. a column alias or TO clause). It falls back to the
// first case-insensitive "AS" separator if "AS SELECT" is not found, which
// also handles views whose body opens with a WITH clause rather than SELECT.
//
// Whitespace around the AS separator is matched flexibly, so DDL rendered with
// multiple spaces, tabs, or newlines between AS and SELECT is still parsed.
//
// Returns "" if neither pattern is present; returning the raw DDL blob when
// no SELECT is found is unhelpful and incorrect.
//
// The match is performed with a regexp on the original ddl string rather than
// on strings.ToUpper(ddl), because strings.ToUpper can change the byte length
// of non-ASCII runes (e.g. the Unicode fi-ligature U+FB01 expands from 3 bytes to
// 2-byte "FI"), making an index from the uppercased copy invalid when applied
// to the original, a source of corrupted output or an out-of-range slice panic.
func extractViewSelectFromCHDDL(ddl string) string {
	// Prefer the specific "AS SELECT" pattern to avoid false splits on an
	// earlier "AS" (e.g. a column alias or a TO clause). loc[2] is the byte
	// offset of the SELECT capture group, so the result begins at "SELECT".
	if loc := reCHViewAsSelect.FindStringSubmatchIndex(ddl); loc != nil {
		return strings.TrimSpace(ddl[loc[2]:])
	}
	// Fallback: first "AS" separator (handles unusual formatting, a WITH-clause
	// body, or non-SELECT views). loc[1] is the byte offset just past the
	// matched separator, robust to however much whitespace it spanned.
	if loc := reCHViewAs.FindStringIndex(ddl); loc != nil {
		return strings.TrimSpace(ddl[loc[1]:])
	}
	// No recognizable AS separator: do not return the raw DDL blob.
	return ""
}

// Type prefix lengths for ClickHouse wrapper types.
const (
	// nullablePrefixLen is the length of the "Nullable(" prefix string.
	nullablePrefixLen = len("Nullable(")

	// lowCardinalityPrefixLen is the length of the "LowCardinality(" prefix string.
	lowCardinalityPrefixLen = len("LowCardinality(")
)

// isNullableType checks if a ClickHouse type string has Nullable as its
// outermost wrapper. It returns true for "Nullable(String)" but false for
// "LowCardinality(Nullable(String))" because the outer wrapper is LowCardinality.
//
// Use this function when reading from system.columns, where the type string
// reflects the declared schema type. In system.columns, a column declared as
// "Nullable(String)" will have exactly that type string, making direct prefix
// matching correct.
//
// For query result processing where the ClickHouse driver may report types like
// "LowCardinality(Nullable(String))", use isNullableTypeUnwrapped instead.
func isNullableType(typeName string) bool {
	return len(typeName) > nullablePrefixLen && typeName[:nullablePrefixLen] == "Nullable("
}

// isNullableTypeUnwrapped checks if a ClickHouse type is nullable after
// stripping any LowCardinality wrapper. It returns true for both "Nullable(T)"
// and "LowCardinality(Nullable(T))".
//
// ClickHouse's LowCardinality is a storage optimization that can wrap Nullable
// types. When processing query results via sql.ColumnType.DatabaseTypeName(),
// the driver reports the full type including LowCardinality. For example, a
// column declared as "LowCardinality(Nullable(String))" will be reported with
// that exact type string.
//
// The standard sql.ColumnType.Nullable() method may not correctly report
// nullability for ClickHouse types, so we must detect it from the type name.
// This function handles both patterns:
//   - "Nullable(String)" -> true
//   - "LowCardinality(Nullable(String))" -> true
//   - "LowCardinality(String)" -> false
//   - "String" -> false
//
// Use this function in recordMetaFromColumnTypes when processing query results.
// For schema metadata from system.columns, use isNullableType instead.
func isNullableTypeUnwrapped(typeName string) bool {
	// Strip LowCardinality wrapper if present.
	if len(typeName) > lowCardinalityPrefixLen+1 && typeName[:lowCardinalityPrefixLen] == "LowCardinality(" {
		typeName = typeName[lowCardinalityPrefixLen : len(typeName)-1]
	}

	return isNullableType(typeName)
}

// baseTypeFromClickHouseType strips wrapper types (LowCardinality, Nullable)
// from a ClickHouse type string and returns the unwrapped base type.
//
// Examples:
//   - "Nullable(UInt16)" -> "UInt16"
//   - "LowCardinality(Nullable(String))" -> "String"
//   - "LowCardinality(String)" -> "String"
//   - "String" -> "String"
func baseTypeFromClickHouseType(chType string) string {
	// Strip LowCardinality wrapper if present. Must be done first since
	// LowCardinality can wrap Nullable: LowCardinality(Nullable(String)).
	if len(chType) > lowCardinalityPrefixLen+1 && chType[:lowCardinalityPrefixLen] == "LowCardinality(" {
		chType = chType[lowCardinalityPrefixLen : len(chType)-1]
	}

	// Strip Nullable wrapper if present. After stripping LowCardinality above,
	// we can use isNullableType which checks for direct Nullable(...) prefix.
	if isNullableType(chType) {
		chType = chType[nullablePrefixLen : len(chType)-1]
	}

	return chType
}

// kindFromClickHouseType maps ClickHouse type names to sq kind.Kind values.
// It handles wrapped types like LowCardinality(T) and Nullable(T) by
// first calling [baseTypeFromClickHouseType] to strip those wrappers,
// then matching the resulting base type.
//
// Matching strategy:
//
// The function uses a two-tier approach. Bare type names (e.g. "DateTime",
// "Int64") are matched exactly via a switch statement for performance.
// Parameterized types that ClickHouse reports with parenthesized arguments
// are matched via [strings.HasPrefix] in the default branch. This is
// necessary because ClickHouse's system.columns table reports these types
// with their full parameterization, not as bare names:
//
//   - DateTime64(3), DateTime64(6,'UTC'), DateTime('Europe/Berlin')
//   - FixedString(10), FixedString(255)
//   - Decimal(18,4), Decimal128(10)
//   - Array(String), Array(Array(Int32))
//
// The prefix-based matching in the default branch is ordered so that
// "DateTime" is checked first (to catch DateTime64(N) and DateTime('tz')
// variants that didn't match the exact switch cases).
//
// Type mapping:
//
//	ClickHouse Type                  -> sq Kind
//	---------------------------------------------------
//	Int8, Int16, Int32, Int64        -> kind.Int
//	UInt8, UInt16, UInt32, UInt64    -> kind.Int
//	Float32, Float64                 -> kind.Float
//	String                           -> kind.Text
//	FixedString(N)                   -> kind.Text
//	Bool                             -> kind.Bool
//	Date, Date32                     -> kind.Date
//	DateTime, DateTime64             -> kind.Datetime
//	DateTime('tz'), DateTime64(N)    -> kind.Datetime
//	DateTime64(N,'tz')               -> kind.Datetime
//	UUID                             -> kind.Text
//	Decimal(P,S)                     -> kind.Decimal
//	Array(T)                         -> kind.Text (serialized as CSV text)
//	Unknown types                    -> kind.Text (safe fallback)
//
// Wrappers are stripped before mapping:
//   - LowCardinality(Nullable(String)) -> "String" -> kind.Text
//   - Nullable(Int64) -> "Int64" -> kind.Int
func kindFromClickHouseType(chType string) kind.Kind {
	chType = baseTypeFromClickHouseType(chType)

	switch chType {
	case "UInt8", "UInt16", "UInt32", "UInt64":
		return kind.Int
	case "Int8", "Int16", "Int32", "Int64":
		return kind.Int
	case "Float32", "Float64":
		return kind.Float
	case "String":
		return kind.Text
	case "Bool":
		return kind.Bool
	case "Date", "Date32":
		return kind.Date
	case "DateTime", "DateTime64":
		return kind.Datetime
	case "UUID":
		return kind.Text
	default:
		// Check for DateTime/DateTime64 with parameters, e.g.
		// DateTime('UTC'), DateTime64(3), DateTime64(6,'UTC').
		if strings.HasPrefix(chType, "DateTime") {
			return kind.Datetime
		}
		// Check for FixedString(N) types - ClickHouse returns "FixedString(10)" not "FixedString".
		if strings.HasPrefix(chType, "FixedString") {
			return kind.Text
		}
		// Check for Decimal types.
		if strings.HasPrefix(chType, "Decimal") {
			return kind.Decimal
		}
		// Check for Array types.
		if strings.HasPrefix(chType, "Array") {
			return kind.Text // Arrays serialized as text for now
		}
		// Default to text for unknown types.
		return kind.Text
	}
}

// recordMetaFromColumnTypes creates record metadata from SQL column types
// returned by a query. This metadata is used to properly scan and transform
// query results into sq records.
//
// For each column, it:
//  1. Gets the database type name from sql.ColumnType.DatabaseTypeName()
//  2. Maps the ClickHouse type to an sq kind using kindFromClickHouseType
//  3. Determines nullability using isNullableTypeUnwrapped (handles
//     LowCardinality(Nullable(T)) patterns)
//  4. Sets the appropriate scan type based on kind and nullability
//
// The function uses isNullableTypeUnwrapped rather than isNullableType because
// the ClickHouse driver reports full type strings like
// "LowCardinality(Nullable(String))" where Nullable is not the outermost
// wrapper. See isNullableTypeUnwrapped documentation for details.
func recordMetaFromColumnTypes(ctx context.Context, colTypes []*sql.ColumnType) (record.Meta, error) {
	sColTypeData := make([]*record.ColumnTypeData, len(colTypes))
	ogColNames := make([]string, len(colTypes))
	for i, colType := range colTypes {
		dbTypeName := colType.DatabaseTypeName()
		knd := kindFromClickHouseType(dbTypeName)
		colTypeData := record.NewColumnTypeData(colType, knd)

		// The ClickHouse driver's sql.ColumnType.Nullable() method may not correctly
		// report nullability, so we detect it from DatabaseTypeName(). We use
		// isNullableTypeUnwrapped (not isNullableType) because the driver reports
		// full type strings like "LowCardinality(Nullable(String))" where Nullable
		// is not the outermost wrapper. Without unwrapping, we'd incorrectly treat
		// such columns as non-nullable, causing scan errors when NULL values appear.
		if isNullableTypeUnwrapped(dbTypeName) {
			colTypeData.Nullable = true
			colTypeData.HasNullable = true
		}

		setScanType(colTypeData, knd, colTypeData.Nullable)

		// ClickHouse Array types need special handling. The driver returns slices
		// (e.g., []string for Array(String)), but we've mapped them to kind.Text.
		// Override the scan type to use any, which can accept slice values.
		// The getNewRecordFunc will convert these to strings.
		if strings.HasPrefix(dbTypeName, "Array") {
			colTypeData.ScanType = sqlz.RTypeAny
		}

		sColTypeData[i] = colTypeData

		ogColNames[i] = colTypeData.Name
	}

	ogColNames = resolveQualifiedColNames(ogColNames)

	mungedColNames, err := driver.MungeResultColNames(ctx, ogColNames)
	if err != nil {
		return nil, err
	}

	recMeta := make(record.Meta, len(colTypes))
	for i := range sColTypeData {
		recMeta[i] = record.NewFieldMeta(sColTypeData[i], mungedColNames[i])
	}

	return recMeta, nil
}

// resolveQualifiedColNames strips the table qualifier (everything up to the
// final dot) from a result-column name, but only when doing so resolves a
// collision with another column. For a duplicate-column JOIN, ClickHouse
// disambiguates the colliding columns by keeping their table qualifier (e.g.
// "film_actor.actor_id"), unlike most databases which return the bare name.
// Stripping the qualifier lets the downstream dedup mechanism
// (driver.MungeResultColNames) rename the duplicate consistently (e.g.
// "actor_id_1"), matching the other drivers.
//
// ClickHouse only ever qualifies a name to disambiguate a duplicate, so this
// mirrors that: a dotted name is collapsed to its trailing segment only when
// that segment also occurs as another column's name. Names whose dot is part of
// an alias or expression (e.g. "avg(.actor_id)", "a.b", ".lead") collide with
// nothing and are preserved verbatim. When two qualified columns collide on
// their trailing segment (e.g. "sales.amount" and "returns.amount"), both are
// reduced to the bare name and then deduped ("amount", "amount_1"); the
// distinguishing qualifiers are intentionally dropped to match the other
// drivers. See https://github.com/neilotoole/sq/issues/834.
//
// Comparison is case-insensitive to match MungeResultColNames, which detects
// duplicates with strings.EqualFold.
func resolveQualifiedColNames(names []string) []string {
	// trailing returns the segment after the final dot, or the whole name when
	// there's no dot. This is the bare column name a qualifier would reduce to.
	trailing := func(name string) string {
		if idx := strings.LastIndex(name, "."); idx != -1 {
			return name[idx+1:]
		}
		return name
	}

	// Count how many columns share each trailing segment, case-insensitively
	// (matching MungeResultColNames). A segment that occurs more than once marks
	// a collision that a qualifier would disambiguate.
	freq := make(map[string]int, len(names))
	for _, name := range names {
		freq[strings.ToLower(trailing(name))]++
	}

	out := make([]string, len(names))
	for i, name := range names {
		// bare != name iff name is qualified (contains a dot). Strip the
		// qualifier only when the bare name collides with another column.
		if bare := trailing(name); bare != name && freq[strings.ToLower(bare)] > 1 {
			out[i] = bare
		} else {
			out[i] = name
		}
	}

	return out
}

// getNewRecordFunc returns a NewRecordFunc that transforms scanned row data
// into sq records. The function uses the provided record.Meta to interpret
// the raw scanned values and convert them to the appropriate Go types.
//
// This is used by RecordMeta to provide the transformation function that
// processes each row returned by a query.
//
// Special handling for ClickHouse Array types: When a column is an Array type,
// its scan type is set to any (to accept []T values from the driver). This
// function converts those slice values to comma-separated string representation
// before passing them to NewRecordFromScanRow.
func getNewRecordFunc(rowMeta record.Meta) driver.NewRecordFunc {
	return func(row []any) (record.Record, error) {
		// Convert any slice values (from Array columns) to strings.
		// The row contains pointers (like *any) from the scan operation,
		// so we need to dereference before checking for slice types.
		for i, val := range row {
			if val == nil {
				continue
			}

			// Dereference *any if needed to get the actual value
			actual := val
			if ptr, ok := val.(*any); ok && ptr != nil {
				actual = *ptr
			}

			// Convert slices to strings
			converted := convertArrayToString(actual)
			if converted != actual {
				// If conversion happened, update the pointer's value
				if ptr, ok := val.(*any); ok {
					*ptr = converted
				} else {
					row[i] = converted
				}
			}
		}
		rec, skipped := driver.NewRecordFromScanRow(rowMeta, row, nil)
		if len(skipped) > 0 {
			return nil, errz.Errorf("expected zero skipped cols but have %d: %v", len(skipped), skipped)
		}
		return rec, nil
	}
}

// convertArrayToString converts slice values to a comma-separated string
// representation. If the value is not a slice, it is returned unchanged.
// This is used to handle ClickHouse Array types which are scanned as Go slices
// but need to be stored as text in sq's record system.
func convertArrayToString(val any) any {
	switch v := val.(type) {
	case []string:
		return strings.Join(v, ",")
	case []int:
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = strconv.Itoa(n)
		}
		return strings.Join(parts, ",")
	case []int8:
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = strconv.FormatInt(int64(n), 10)
		}
		return strings.Join(parts, ",")
	case []int16:
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = strconv.FormatInt(int64(n), 10)
		}
		return strings.Join(parts, ",")
	case []int32:
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = strconv.FormatInt(int64(n), 10)
		}
		return strings.Join(parts, ",")
	case []int64:
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = strconv.FormatInt(n, 10)
		}
		return strings.Join(parts, ",")
	case []uint:
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = strconv.FormatUint(uint64(n), 10)
		}
		return strings.Join(parts, ",")
	case []uint8:
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = strconv.FormatUint(uint64(n), 10)
		}
		return strings.Join(parts, ",")
	case []uint16:
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = strconv.FormatUint(uint64(n), 10)
		}
		return strings.Join(parts, ",")
	case []uint32:
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = strconv.FormatUint(uint64(n), 10)
		}
		return strings.Join(parts, ",")
	case []uint64:
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = strconv.FormatUint(n, 10)
		}
		return strings.Join(parts, ",")
	case []float32:
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = strconv.FormatFloat(float64(n), 'f', -1, 32)
		}
		return strings.Join(parts, ",")
	case []float64:
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = strconv.FormatFloat(n, 'f', -1, 64)
		}
		return strings.Join(parts, ",")
	case []bool:
		parts := make([]string, len(v))
		for i, b := range v {
			parts[i] = strconv.FormatBool(b)
		}
		return strings.Join(parts, ",")
	default:
		return val
	}
}

// setScanType sets the appropriate Go reflect.Type for scanning a column's
// values. The scan type determines what Go type will be used to receive values
// from the database driver during row scanning.
//
// For nullable columns, it uses sql.Null* types (e.g., sql.NullString,
// sql.NullInt64) which can represent NULL values. For non-nullable columns,
// it uses the corresponding primitive types (string, int64, etc.).
//
// Scan type mapping:
//
//	Kind        | Nullable         | Non-Nullable
//	------------|------------------|-------------
//	Text        | sql.NullString   | string
//	Int         | sql.NullInt64    | int64
//	Float       | sql.NullFloat64  | float64
//	Bool        | sql.NullBool     | bool
//	Datetime    | sql.NullTime     | time.Time
//	Date        | sql.NullTime     | time.Time
//	Time        | sql.NullTime     | time.Time
//	Decimal     | NullDecimal      | string
//	Bytes       | []byte           | []byte
func setScanType(colTypeData *record.ColumnTypeData, knd kind.Kind, nullable bool) {
	if nullable {
		// Use nullable scan types to properly handle NULL values
		switch knd {
		case kind.Unknown, kind.Null, kind.Text:
			colTypeData.ScanType = sqlz.RTypeNullString
		case kind.Decimal:
			colTypeData.ScanType = sqlz.RTypeNullDecimal
		case kind.Int:
			colTypeData.ScanType = sqlz.RTypeNullInt64
		case kind.Float:
			colTypeData.ScanType = sqlz.RTypeNullFloat64
		case kind.Bool:
			colTypeData.ScanType = sqlz.RTypeNullBool
		case kind.Datetime, kind.Date, kind.Time:
			colTypeData.ScanType = sqlz.RTypeNullTime
		case kind.Bytes:
			colTypeData.ScanType = sqlz.RTypeBytes // []byte handles nil naturally
		}
		return
	}

	// Non-nullable columns use regular scan types
	switch knd {
	case kind.Unknown, kind.Null, kind.Text, kind.Decimal:
		colTypeData.ScanType = sqlz.RTypeString
	case kind.Int:
		colTypeData.ScanType = sqlz.RTypeInt64
	case kind.Float:
		colTypeData.ScanType = sqlz.RTypeFloat64
	case kind.Bool:
		colTypeData.ScanType = sqlz.RTypeBool
	case kind.Datetime, kind.Date, kind.Time:
		colTypeData.ScanType = sqlz.RTypeTime
	case kind.Bytes:
		colTypeData.ScanType = sqlz.RTypeBytes
	}
}

// semverRx matches a leading dotted-numeric version token (up to three parts).
var semverRx = regexp.MustCompile(`^v?(\d+(?:\.\d+){0,2})`)

// parseSemver normalizes a ClickHouse version() string to canonical semver
// (e.g. "v25.3.2"). ClickHouse versions are four-part; the regex caps at three.
func parseSemver(raw string) (string, error) {
	m := semverRx.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return "", errz.Errorf("no semver in clickhouse version string: %q", raw)
	}
	v := semver.Canonical("v" + m[1])
	if !semver.IsValid(v) {
		return "", errz.Errorf("invalid clickhouse semver %q from %q", v, raw)
	}
	return v, nil
}

// DBSemver implements driver.SQLDriver.
func (d *driveri) DBSemver(ctx context.Context, db sqlz.DB) (string, error) {
	var raw string
	if err := db.QueryRowContext(ctx, "SELECT version()").Scan(&raw); err != nil {
		return "", errw(err)
	}
	return parseSemver(raw)
}
