// Package portsakila translates the SQLite sakila SQL files into a form
// that DuckDB can ingest.
package portsakila

import (
	"regexp"
	"strings"
)

// Port translates a SQLite sakila SQL file into a DuckDB-compatible form.
// It strips CREATE TRIGGER blocks (DuckDB has no triggers), removes
// AUTOINCREMENT (DuckDB has no such keyword; explicit IDs in INSERTs cover
// it), strips FOREIGN KEY constraint lines (DuckDB enforces forward-reference
// checking at CREATE TABLE time, which the circular store/staff/customer
// references cannot satisfy), and replaces the Firebird-heritage
// "BLOB SUB_TYPE TEXT" type with TEXT.
func Port(in string) (string, error) {
	out := stripTriggers(in)
	out = stripAutoincrement(out)
	out = stripFKConstraintLines(out)
	out = replaceBlobSubTypeText(out)
	return out, nil
}

// stripTriggers removes "CREATE TRIGGER ... END;" blocks. Trigger bodies
// contain semicolons, so we tokenize on the keyword pair, not on ";".
func stripTriggers(s string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, "CREATE TRIGGER")
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		// Find the matching "END;" terminator. The canonical sakila uses one
		// of three slight variations.
		rest := s[i:]
		end := strings.Index(rest, "END\n;")
		if end < 0 {
			end = strings.Index(rest, "END ;")
		}
		if end < 0 {
			end = strings.Index(rest, "END;")
		}
		if end < 0 {
			// Malformed; emit the rest verbatim.
			b.WriteString(rest)
			return b.String()
		}
		afterEnd := end
		switch {
		case strings.HasPrefix(rest[end:], "END\n;"):
			afterEnd += len("END\n;")
		case strings.HasPrefix(rest[end:], "END ;"):
			afterEnd += len("END ;")
		case strings.HasPrefix(rest[end:], "END;"):
			afterEnd += len("END;")
		}
		s = rest[afterEnd:]
	}
}

// stripAutoincrement removes the AUTOINCREMENT keyword (with surrounding spaces).
func stripAutoincrement(s string) string {
	return strings.ReplaceAll(s, " AUTOINCREMENT", "")
}

// reFKConstraintLine matches a CONSTRAINT ... FOREIGN KEY ... line (with an
// optional trailing comma). DuckDB enforces that all referenced tables exist
// at CREATE TABLE time; the sakila schema has circular references between
// store, staff, and customer that cannot be resolved by reordering. Stripping
// the FK constraints is the least-intrusive fix for a read-mostly test fixture.
var reFKConstraintLine = regexp.MustCompile(
	`(?m)^\s*CONSTRAINT\s+\S+\s+FOREIGN\s+KEY[^\n]*\n?`,
)

// stripFKConstraintLines removes FOREIGN KEY constraint lines from CREATE
// TABLE blocks. DuckDB validates forward references at CREATE TABLE time, so
// circular FK relationships (store ↔ staff ↔ customer) cannot be represented
// without first creating all tables and then adding constraints via ALTER
// TABLE — which is more invasive than simply omitting FKs from a test fixture.
func stripFKConstraintLines(s string) string {
	out := reFKConstraintLine.ReplaceAllString(s, "")
	// After stripping a FK line, the previous column definition may now have
	// a trailing comma before the closing paren. Fix "  ,\n)" → "\n)".
	out = reTrailingCommaBeforeParen.ReplaceAllString(out, "\n)")
	return out
}

// reTrailingCommaBeforeParen matches a trailing comma left behind after a FK
// constraint line is stripped from the last column before ")".
var reTrailingCommaBeforeParen = regexp.MustCompile(`(?m),\s*\n\s*\)`)

// replaceBlobSubTypeText replaces the Firebird-heritage "BLOB SUB_TYPE TEXT"
// column type with plain TEXT, which is valid in DuckDB.
func replaceBlobSubTypeText(s string) string {
	return strings.ReplaceAll(s, "BLOB SUB_TYPE TEXT", "TEXT")
}
