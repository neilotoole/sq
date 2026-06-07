package duckdb

import (
	"net/url"
	"strings"
)

// accessModeKey is the DuckDB DSN query-string key controlling open mode.
const accessModeKey = "access_mode"

// ApplyReadOnlyToLocation returns loc with access_mode=READ_ONLY appended
// to its query string, plus a flag reporting whether the rewrite happened.
//
// The rewrite is skipped (loc returned unchanged, changed=false) when:
//   - loc does not start with the duckdb:// scheme. This is defensive:
//     Grips.doOpen should only call this for DuckDB sources, but the helper
//     is total.
//   - loc's path component is the in-memory sentinel ":memory:" (an empty
//     in-memory DB can't usefully be opened READ_ONLY),
//   - loc already specifies access_mode (the user's explicit choice wins),
//   - loc is empty.
//
// The helper is intentionally tolerant: if anything is malformed it returns
// the input unchanged. The driver's existing dsnFromLocation will surface
// the real parsing error downstream.
func ApplyReadOnlyToLocation(loc string) (out string, changed bool) {
	if !strings.HasPrefix(loc, Prefix) {
		return loc, false
	}
	bare := loc[len(Prefix):]
	pathPart, queryPart, hasQuery := strings.Cut(bare, "?")
	if pathPart == ":memory:" {
		return loc, false
	}
	if hasQuery {
		vals, err := url.ParseQuery(queryPart)
		if err != nil {
			return loc, false
		}
		if vals.Has(accessModeKey) {
			return loc, false
		}
		// Append; preserve existing query string verbatim to avoid
		// reordering or re-escaping the user's params.
		return loc + "&" + accessModeKey + "=READ_ONLY", true
	}
	return loc + "?" + accessModeKey + "=READ_ONLY", true
}

// ExplicitAccessMode returns the access_mode value the user set in loc's
// query string, if any. Used by the CLI to detect --readonly vs URL
// conflicts. Value is returned as-is (no case normalization) so callers
// can echo what the user typed in error messages.
func ExplicitAccessMode(loc string) (mode string, ok bool) {
	if !strings.HasPrefix(loc, Prefix) {
		return "", false
	}
	_, queryPart, hasQuery := strings.Cut(loc[len(Prefix):], "?")
	if !hasQuery {
		return "", false
	}
	vals, err := url.ParseQuery(queryPart)
	if err != nil {
		return "", false
	}
	if !vals.Has(accessModeKey) {
		return "", false
	}
	return vals.Get(accessModeKey), true
}
