package duckdb

import (
	"net/url"
	"strings"
)

// accessModeKey is the DuckDB DSN query-string key controlling open mode.
const accessModeKey = "access_mode"

// ApplyReadOnlyToLocation returns loc rewritten so that DuckDB opens it
// READ_ONLY, plus a flag reporting whether the rewrite happened. The
// explicit arg distinguishes explicit user intent (the --readonly flag)
// from the implicit read-only hint that non-writing commands (sq, inspect,
// diff, ping) set.
//
// The access_mode policy matrix (values compared case-insensitively):
//   - no access_mode in loc: READ_ONLY appended (implicit and explicit).
//   - access_mode=AUTOMATIC: overridden to READ_ONLY when explicit
//     (AUTOMATIC expresses no strong preference, while --readonly does);
//     unchanged when implicit.
//   - access_mode=READ_ONLY: unchanged (already read-only).
//   - access_mode=READ_WRITE: unchanged. The user's explicit choice wins;
//     for the explicit --readonly flag, the CLI surfaces the conflict as
//     an error before the driver is reached.
//
// The rewrite is also skipped (loc returned unchanged, changed=false) when:
//   - loc does not start with the duckdb:// scheme. This is defensive:
//     the only caller is the DuckDB driver's own doOpen, which always
//     passes a duckdb-scheme location, but the helper is total.
//   - loc's path component is the in-memory sentinel ":memory:" or is
//     empty: go-duckdb treats both as in-memory (see dsnFromLocation),
//     and an empty in-memory DB can't usefully be opened READ_ONLY.
//   - loc is empty.
//
// The helper is intentionally tolerant: if anything is malformed it returns
// the input unchanged. The driver's existing dsnFromLocation will surface
// the real parsing error downstream.
func ApplyReadOnlyToLocation(loc string, explicit bool) (out string, changed bool) {
	if !strings.HasPrefix(loc, Prefix) {
		return loc, false
	}
	bare := loc[len(Prefix):]
	pathPart, queryPart, hasQuery := strings.Cut(bare, "?")
	if pathPart == ":memory:" || pathPart == "" {
		return loc, false
	}
	if hasQuery {
		vals, err := url.ParseQuery(queryPart)
		if err != nil {
			return loc, false
		}
		if vals.Has(accessModeKey) {
			if explicit && strings.EqualFold(vals.Get(accessModeKey), "AUTOMATIC") {
				// AUTOMATIC defaults to READ_WRITE for a local file, which
				// would silently defeat --readonly: override it. Re-encoding
				// may reorder params, but only on this rare override path.
				vals.Set(accessModeKey, "READ_ONLY")
				return Prefix + pathPart + "?" + vals.Encode(), true
			}
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
