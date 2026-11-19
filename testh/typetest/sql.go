package typetest

import (
	"strings"
)

// rowIDCol is the name of the integer primary key column the harness adds to
// every test table, so that rows can be read back in a known order.
const rowIDCol = "row_id"

// DefaultColumnDecl declares a column of type decl for variant v:
// decl + " NOT NULL" for NotNull, decl + " NULL" for Nullable, and
// decl + " DEFAULT " + defaultExpr + " NOT NULL" for Defaulted. NULL is
// explicit because some engines make a column NOT NULL by default for some
// types (MySQL 5.6 and 5.7 TIMESTAMP, for example). DEFAULT comes before
// NOT NULL because Oracle requires that order. defaultExpr is ignored for
// the other variants.
func DefaultColumnDecl(decl string, v Variant, defaultExpr string) string {
	var clause string
	switch v {
	case NotNull:
		clause = "NOT NULL"
	case Nullable:
		clause = "NULL"
	case Defaulted:
		clause = "DEFAULT " + defaultExpr + " NOT NULL"
	default:
		clause = "NOT NULL"
	}
	if decl == "" {
		// A column with no type, which SQLite accepts.
		return clause
	}
	return decl + " " + clause
}

// CreateTable returns the CREATE TABLE statement for tbl: the row_id column
// declared as rowIDDecl, then cols in order, each declared via colDecl
// (given the column's DefaultExpr), then suffix if it's not empty
// (ClickHouse, for example, requires an ENGINE clause). cols must have been
// passed through ResolveDefaultExprs: a Defaulted column's DefaultExpr is
// empty until then.
// Identifiers are not quoted: Resolve restricts column names to
// [a-z0-9_], and tbl is expected to come from stringz.UniqTableName.
func CreateTable(tbl, rowIDDecl string, cols []Column, colDecl func(decl string, v Variant, defaultExpr string) string,
	suffix string,
) string {
	var sb strings.Builder
	sb.WriteString("CREATE TABLE " + tbl + " (" + rowIDCol + " " + rowIDDecl)
	for _, col := range cols {
		sb.WriteString(", " + col.Name + " " + colDecl(col.Case.Decl, col.Variant, col.DefaultExpr))
	}
	sb.WriteString(")")
	if suffix != "" {
		sb.WriteString(" " + suffix)
	}
	return sb.String()
}

// Insert returns the INSERT statement for row (0, 1 or 2) of tbl, with
// row_id = row+1, and its bound args. It leaves out the columns that aren't
// Inserted, the Defaulted ones, so they take their defaults. A SQL input is written into the
// statement as is; every other input is bound. placeholders is the
// dialect's placeholder func (see dialect.Dialect.Placeholders). Insert
// relies on its format, a parenthesized list of tokens joined by
// driver.Comma, which every dialect uses ("?", "$n", ":n", "@pN").
func Insert(tbl string, cols []Column, row int, placeholders func(numCols, numRows int) string) (
	query string, args []any,
) {
	args = []any{row + 1}
	for _, col := range cols {
		if !col.Inserted() {
			continue
		}
		if _, ok := col.In[row].(SQL); !ok {
			args = append(args, col.In[row])
		}
	}

	toks := strings.Split(strings.TrimSuffix(strings.TrimPrefix(placeholders(len(args), 1), "("), ")"), ", ")

	names := []string{rowIDCol}
	vals := []string{toks[0]}
	next := 1
	for _, col := range cols {
		if !col.Inserted() {
			continue
		}
		names = append(names, col.Name)
		if expr, ok := col.In[row].(SQL); ok {
			vals = append(vals, string(expr))
			continue
		}
		vals = append(vals, toks[next])
		next++
	}

	query = "INSERT INTO " + tbl + " (" + strings.Join(names, ", ") + ") VALUES (" +
		strings.Join(vals, ", ") + ")"
	return query, args
}

// SelectAllNames returns the result column names of SelectAll, in order:
// row_id, then each column without QueryErr set.
func SelectAllNames(cols []Column) []string {
	names := []string{rowIDCol}
	for _, col := range cols {
		if !col.QueryErr.Set {
			names = append(names, col.Name)
		}
	}
	return names
}

// SelectColumn returns the query that reads col alone, with row_id, in row
// order.
func SelectColumn(tbl string, col Column) string {
	return "SELECT " + rowIDCol + ", " + col.Name + " FROM " + tbl + " ORDER BY " + rowIDCol
}

// SelectAll returns the query that reads every column together, in row
// order. When no column expects a query error, that's a literal SELECT *.
// Otherwise it's an explicit projection of row_id and the columns that
// don't, in the order given, since SQL can't exclude columns from *.
func SelectAll(tbl string, cols []Column) string {
	names := SelectAllNames(cols)
	if len(names) == len(cols)+1 {
		return "SELECT * FROM " + tbl + " ORDER BY " + rowIDCol
	}
	return "SELECT " + strings.Join(names, ", ") + " FROM " + tbl + " ORDER BY " + rowIDCol
}
