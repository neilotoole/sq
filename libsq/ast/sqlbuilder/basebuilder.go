package sqlbuilder

import (
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
)

const (
	singleQuote = '\''
	sp          = ' '
)

// renderSelectorNode renders a selector such as ".actor.first_name"
// or ".last_name".
func renderSelectorNode(quote string, node ast.Node) (string, error) {
	// FIXME: switch to using enquote
	switch node := node.(type) {
	case *ast.ColSelectorNode:
		return fmt.Sprintf(
			"%s%s%s",
			quote,
			node.ColName(),
			quote,
		), nil
	case *ast.TblColSelectorNode:
		return fmt.Sprintf(
			"%s%s%s.%s%s%s",
			quote,
			node.TblName(),
			quote,
			quote,
			node.ColName(),
			quote,
		), nil
	case *ast.TblSelectorNode:
		return fmt.Sprintf(
			"%s%s%s",
			quote,
			node.TblName(),
			quote,
		), nil

	default:
		return "", errz.Errorf(
			"expected selector node type, but got %T: %s",
			node,
			node.Text(),
		)
	}
}

// sqlAppend is a convenience function for building the SQL string.
// The main purpose is to ensure that there's always a consistent amount
// of whitespace. Thus, if existing has a space suffix and add has a
// space prefix, the returned string will only have one space. If add
// is the empty string or just whitespace, this function simply
// returns existing.
func sqlAppend(existing, add string) string {
	add = strings.TrimSpace(add)
	if add == "" {
		return existing
	}

	existing = strings.TrimSpace(existing)
	return existing + " " + add
}

// quoteTableOrColSelector returns a quote table, col, or table/col
// selector for use in a SQL statement. For example:
//
//	.table     -->  "table"
//	.col       -->  "col"
//	.table.col -->  "table"."col"
//
// Thus, the selector must have exactly one or two periods.
//
// Deprecated: use renderSelectorNode.
func quoteTableOrColSelector(quote, selector string) (string, error) {
	if len(selector) < 2 || selector[0] != '.' {
		return "", errz.Errorf("invalid selector: %s", selector)
	}

	parts := strings.Split(selector[1:], ".")
	switch len(parts) {
	case 1:
		return quote + parts[0] + quote, nil
	case 2:
		return quote + parts[0] + quote + "." + quote + parts[1] + quote, nil
	default:
		return "", errz.Errorf("invalid selector: %s", selector)
	}
}

// escapeLiteral escapes the single quotes in s.
//
//	jessie's girl  -->  jessie''s girl
func escapeLiteral(s string) string {
	sb := strings.Builder{}
	for _, r := range s {
		if r == singleQuote {
			_, _ = sb.WriteRune(singleQuote)
		}

		_, _ = sb.WriteRune(r)
	}

	return sb.String()
}

// unquoteLiteral returns true if s is a double-quoted string, and also returns
// the value with the quotes stripped. An error is returned if the string
// is malformed.
//
// REVISIT: why not use strconv.Unquote or such?
func unquoteLiteral(s string) (val string, ok bool, err error) {
	hasPrefix := strings.HasPrefix(s, `"`)
	hasSuffix := strings.HasSuffix(s, `"`)

	if hasPrefix && hasSuffix {
		val = strings.TrimPrefix(s, `"`)
		val = strings.TrimSuffix(val, `"`)
		return val, true, nil
	}

	if hasPrefix != hasSuffix {
		return "", false, errz.Errorf("malformed literal: %s", s)
	}

	return s, false, nil
}
