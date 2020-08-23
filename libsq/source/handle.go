package source

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/libsq/core/errz"
)

var (
	handlePattern = regexp.MustCompile(`\A[@][a-zA-Z][a-zA-Z0-9_]*$`)
	tablePattern  = regexp.MustCompile(`\A[a-zA-Z_][a-zA-Z0-9_]*$`)
)

// VerifyLegalHandle returns an error if handle is
// not an acceptable source handle value.
// Valid input must match:
//
//   \A[@][a-zA-Z][a-zA-Z0-9_]*$
func VerifyLegalHandle(handle string) error {
	matches := handlePattern.MatchString(handle)
	if !matches {
		return errz.Errorf(`invalid data source handle %q: must begin with @, followed by a letter, followed by zero or more letters, digits, or underscores, e.g. "@my_db1"`, handle)
	}

	return nil
}

// verifyLegalTableName returns an error if table is not an
// acceptable table name. Valid input must match:
//
//   \A[a-zA-Z_][a-zA-Z0-9_]*$`
//
func verifyLegalTableName(table string) error {
	matches := tablePattern.MatchString(table)
	if !matches {
		return errz.Errorf(`invalid table name %q: must begin a letter or underscore, followed by zero or more letters, digits, or underscores, e.g. "tbl1" or "_tbl2"`, table)
	}
	return nil
}

// handleTypeAliases is a map of type names to the
// more user-friendly suffix returned by SuggestHandle.
var handleTypeAliases = map[string]string{
	typeSL3.String(): "sqlite",
	typePg.String():  "pg",
	typeMS.String():  "mssql",
	typeMy.String():  "my",
}

// SuggestHandle suggests a handle based on location and type.
// If typ is TypeNone, the type will be inferred from loc.
// The takenFn is used to determine if a suggested handle
// is free to be used (e.g. "@sakila_csv" -> "@sakila_csv_1", etc).
//
// If the base name (derived from loc) contains illegal handle runes,
// those are replaced with underscore. If the handle would start with
// a number or underscore, it will be prefixed with "h" (for "handle").
// Thus "123.xlsx" becomes "@h123_xlsx".
func SuggestHandle(typ Type, loc string, takenFn func(string) bool) (string, error) {
	ploc, err := parseLoc(loc)
	if err != nil {
		return "", err
	}

	if typ == TypeNone {
		typ = ploc.typ
	}

	// use the type name as the _ext suffix if possible
	ext := typ.String()
	if ext == "" {
		if len(ploc.ext) > 0 {
			ext = ploc.ext[1:] // trim the leading period in ".xlsx" etc
		}
	}

	if alias, ok := handleTypeAliases[ext]; ok {
		ext = alias
	}
	// make sure there's nothing funky in ext or name
	ext = stringz.SanitizeAlphaNumeric(ext, '_')
	name := stringz.SanitizeAlphaNumeric(ploc.name, '_')

	// if the name is empty, we use "h" (for "handle"), e.g "@h".
	if name == "" {
		name = "h"
	} else if !unicode.IsLetter([]rune(name)[0]) {
		// If the first rune is not a letter, we prepend "h".
		// So "123" becomes "h123", or "_123" becomes "h_123".
		name = "h" + name
	}

	base := "@" + name
	if ext != "" {
		base += "_" + ext
	}

	// Beginning with base as candidate, check if
	// candidate is taken; if so, append _N, where
	// N is a count starting at 1.
	candidate := base
	var count int
	for {
		if count > 0 {
			candidate = base + "_" + strconv.Itoa(count)
		}

		if !takenFn(candidate) {
			return candidate, nil
		}

		count++
	}
}

// ParseTableHandle attempts to parse a SLQ source handle and/or table name.
// Surrounding whitespace is trimmed. Examples of valid input values are:
//
//   @handle.tblName
//   @handle
//   .tblName
func ParseTableHandle(input string) (handle, table string, err error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", "", errz.New("empty input")
	}

	if strings.Contains(trimmed, ".") {
		if trimmed[0] == '.' {
			// starts with a period; so it's only the table name
			err = verifyLegalTableName(trimmed[1:])
			if err != nil {
				return "", "", err
			}
			return "", trimmed[1:], nil
		}

		// input contains both handle and table
		parts := strings.Split(trimmed, ".")
		if len(parts) != 2 {
			return "", "", errz.Errorf("invalid handle/table input %q", input)
		}

		err = VerifyLegalHandle(parts[0])
		if err != nil {
			return "", "", err
		}

		err = verifyLegalTableName(parts[1])
		if err != nil {
			return "", "", err
		}

		return parts[0], parts[1], nil
	}

	// input does not contain a period, therefore it must be a handle by itself
	err = VerifyLegalHandle(trimmed)
	if err != nil {
		return "", "", err
	}

	return trimmed, "", err
}
