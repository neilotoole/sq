// Package portsakila translates the SQLite sakila SQL files into a form
// that DuckDB can ingest.
package portsakila

import (
	"strings"
)

// Port translates a SQLite sakila SQL file into a DuckDB-compatible form.
// It strips CREATE TRIGGER blocks (DuckDB has no triggers) and removes
// AUTOINCREMENT (DuckDB has no such keyword; explicit IDs in INSERTs cover it).
func Port(in string) (string, error) {
	out := stripTriggers(in)
	out = stripAutoincrement(out)
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
