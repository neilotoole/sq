package sqlz

import (
	"regexp"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// IsSQLFuncExpr returns true if s is of the form "FUNC(something)".
// NOTE: this is a rudimentary implementation (using regex instead
//  of the AST). There are many ways to defeat this function.
func IsSQLFuncExpr(s string) bool {
	s = strings.TrimSpace(s)
	re := regexp.MustCompile(`([a-zA-Z_{1}][a-zA-Z0-9_]+)(\s*\()(.*)(\s*\))`)
	b := re.Find([]byte(s))

	if len(b) == 0 {
		return false
	}

	return true
}

// ExtractOuterFuncNameFromSQLExpr extracts the outermost function name from
// a text string such as "ST_Binary(col_name)".
//
// NOTE: This func is pretty limited, it uses a simple regex and will be
//  defeated by inner parentheses etc. A sturdier impl would use AST
//  instead of regex.
func ExtractOuterFuncNameFromSQLExpr(s string) (string, error) {
	s = strings.TrimSpace(s)

	// A better man would use the AST for this
	ok := IsSQLFuncExpr(s)
	if !ok {
		return "", errz.Errorf("not a valid SQL func expr: %s", s)
	}

	re := regexp.MustCompile(`([a-zA-Z_{1}][a-zA-Z0-9_]+)(\s*\()(.*)(\s*\))`)
	b := re.FindAllStringSubmatch(s, -1)

	if len(b) < 1 {
		return "", errz.Errorf("not a valid SQL func expr: %s", s)
	}

	m := stringz.StripEmptyOrSpaceFromSlice(b[0])

	switch {
	case len(m) < 4:
		return "", errz.Errorf("not a valid SQL func expr: %s", s)
	case m[2] != "(", m[len(m)-1] != ")":
		return "", errz.Errorf("not a valid SQL func expr: %s", s)
	default:
	}

	return m[1], nil
}
