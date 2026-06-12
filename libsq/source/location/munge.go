package location

import (
	"path/filepath"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// MungeForDriver applies driver-specific location canonicalization for
// the file-based DB types (SQLite, DuckDB); other driver types pass
// through unchanged. For SQLite, each of these forms is allowed:
//
//	sqlite3:///path/to/sakila.db  --> sqlite3:///path/to/sakila.db
//	sqlite3:sakila.db             --> sqlite3:///current/working/dir/sakila.db
//	sqlite3:/sakila.db            --> sqlite3:///sakila.db
//	sqlite3:./sakila.db           --> sqlite3:///current/working/dir/sakila.db
//	sakila.db                     --> sqlite3:///current/working/dir/sakila.db
//	/path/to/sakila.db            --> sqlite3:///path/to/sakila.db
//
// DuckDB accepts the same forms with the "duckdb:" scheme, plus the
// ":memory:" sentinel (optionally scheme-prefixed), which passes
// through as "duckdb://:memory:".
//
// An optional "?key=val[&...]" connection-string suffix is preserved
// verbatim. The first '?' is always treated as the path/query
// separator, so paths whose POSIX filename legally contains '?' are
// not supported.
//
// MungeForDriver is idempotent: an already-canonical location is
// returned unchanged. This matters because it runs both at add time
// (on the literal location the user typed) and at connect time (on a
// location resolved from secret placeholders, which may already be in
// canonical form); see driver.ResolveSourceSecrets.
//
// Errors returned by MungeForDriver do not echo the location: at
// connect time the location bytes are resolved secret material.
//
// Note that this function is OS-dependent, due to the use of pkg
// filepath. Thus, on Windows, this is seen:
//
//	C:/Users/sq/sakila.db  --> sqlite3://C:/Users/sq/sakila.db
//
// But that input location gets mangled on non-Windows OSes. This
// probably isn't a problem in practice, but longer-term it may make
// sense to rewrite the munging to be OS-independent.
func MungeForDriver(typ drivertype.Type, loc string) (string, error) {
	switch typ { //nolint:exhaustive // all other driver types pass through via default
	case drivertype.SQLite:
		return mungeFileDBLocation("sqlite3://", loc, false)
	case drivertype.DuckDB:
		return mungeFileDBLocation("duckdb://", loc, true)
	default:
		return loc, nil
	}
}

// mungeFileDBLocation canonicalizes a file-based DB location per
// MungeForDriver. Arg prefix is the canonical location prefix, e.g.
// "sqlite3://". If allowMemory is true, the ":memory:" path sentinel
// is preserved instead of being treated as a file path.
func mungeFileDBLocation(prefix, loc string, allowMemory bool) (string, error) {
	loc2 := strings.TrimSpace(loc)
	if loc2 == "" {
		return "", errz.New("location must not be empty")
	}

	loc2 = strings.TrimPrefix(loc2, prefix)
	// Also trim the bare scheme form, e.g. "sqlite3:sakila.db".
	loc2 = strings.TrimPrefix(loc2, strings.TrimSuffix(prefix, "//"))

	pathPart, queryPart, hasQuery := strings.Cut(loc2, "?")
	if allowMemory && pathPart == ":memory:" {
		if hasQuery {
			return prefix + ":memory:?" + queryPart, nil
		}
		return prefix + ":memory:", nil
	}

	fp, err := filepath.Abs(pathPart)
	if err != nil {
		// Don't echo the path: it may be resolved secret material, or
		// carry secret connection params in a "?key=val" suffix.
		return "", errz.Wrap(err, "invalid location")
	}

	fp = filepath.ToSlash(fp)
	if hasQuery {
		return prefix + fp + "?" + queryPart, nil
	}
	return prefix + fp, nil
}
