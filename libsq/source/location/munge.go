package location

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/secret"
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
// MungeForDriver operates on literal location bytes: loc is used
// verbatim, with no placeholder-template interpretation. It is the
// connect-time munge, applied to a location whose secret placeholders
// and '$$' escapes have already been resolved to literal form (see
// driver.ResolveSourceSecrets). For the add-time munge of a typed
// location, which is a placeholder template, use
// MungeTemplateForDriver.
//
// MungeForDriver is idempotent: an already-canonical location is
// returned unchanged.
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
	return mungeForDriver(typ, loc, false)
}

// MungeTemplateForDriver is the placeholder-template counterpart of
// MungeForDriver, for use at add time on the location the user typed.
// The typed location is a placeholder template ('$$' escapes a
// literal '$'), and absolutizing a relative path splices in literal
// filesystem bytes from the current working directory; those
// cwd-derived bytes are escaped (secret.Escape) before joining, while
// the user's typed bytes are preserved exactly as typed. See
// absTemplatePath for the attribution rule. Otherwise the
// canonicalization is identical to MungeForDriver, including
// idempotence: re-munging a stored template does not re-escape it,
// because an already-absolute path acquires no cwd bytes.
//
// loc must be a ref-free template (no well-formed ${scheme:path}
// placeholders): placeholder-bearing locations are opaque at add time
// and must not be munged at all. For the file-DB types, this contract
// is enforced: a loc bearing a placeholder, or with malformed
// placeholder syntax, returns an error rather than being munged.
// Pass-through driver types stay pass-through and are not inspected.
// As elsewhere in this file, the error does not echo the location.
func MungeTemplateForDriver(typ drivertype.Type, loc string) (string, error) {
	switch typ { //nolint:exhaustive // only the file-DB types munge; others pass through
	case drivertype.SQLite, drivertype.DuckDB:
		// Enforce the ref-free contract before any path bytes are
		// touched: munging a placeholder-bearing template would
		// absolutize text the user meant as a placeholder, silently
		// changing the template's semantics. The errors don't echo the
		// location or the parse detail: templates can carry
		// sensitive-adjacent text.
		refs, err := secret.ExtractRefs(loc)
		if err != nil {
			return "", errz.New("cannot munge location: invalid placeholder syntax")
		}
		if len(refs) > 0 {
			return "", errz.New("cannot munge location: template contains ${scheme:path} placeholders")
		}
	}
	return mungeForDriver(typ, loc, true)
}

func mungeForDriver(typ drivertype.Type, loc string, template bool) (string, error) {
	switch typ { //nolint:exhaustive // all other driver types pass through via default
	case drivertype.SQLite:
		return mungeFileDBLocation("sqlite3://", loc, false, template)
	case drivertype.DuckDB:
		return mungeFileDBLocation("duckdb://", loc, true, template)
	default:
		return loc, nil
	}
}

// mungeFileDBLocation canonicalizes a file-based DB location per
// MungeForDriver. Arg prefix is the canonical location prefix, e.g.
// "sqlite3://". If allowMemory is true, the ":memory:" path sentinel
// is preserved instead of being treated as a file path. If template
// is true, loc is placeholder-template bytes and absolutization is
// escape-aware (see MungeTemplateForDriver); otherwise loc is literal
// bytes.
func mungeFileDBLocation(prefix, loc string, allowMemory, template bool) (string, error) {
	loc2 := strings.TrimSpace(loc)
	if loc2 == "" {
		return "", errz.New("location must not be empty")
	}

	loc2 = strings.TrimPrefix(loc2, prefix)
	// Also trim the bare scheme form, e.g. "sqlite3:sakila.db".
	loc2 = strings.TrimPrefix(loc2, strings.TrimSuffix(prefix, "//"))

	pathPart, queryPart, hasQuery := strings.Cut(loc2, "?")
	if pathPart == "" {
		// Reject explicitly: filepath.Abs("") would resolve to the
		// current working directory, silently canonicalizing a prefix-only
		// location (or an empty resolved placeholder value) to a DB
		// location pointing at the cwd.
		return "", errz.New("location path must not be empty")
	}
	if allowMemory && pathPart == ":memory:" {
		if hasQuery {
			return prefix + ":memory:?" + queryPart, nil
		}
		return prefix + ":memory:", nil
	}

	var fp string
	var err error
	if template {
		fp, err = absTemplatePath(pathPart)
	} else {
		fp, err = filepath.Abs(pathPart)
	}
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

// absTemplatePath absolutizes a path that is placeholder-template
// bytes ('$$' escapes a literal '$'), preserving the template
// semantics of the result (gh #797). Stored source locations are
// templates, but the cwd that absolutization joins in is literal
// filesystem bytes: a directory literally named "q$$exports" or
// "${env:X}" must land in the template escaped, or the stored
// location acquires escapes (or worse, well-formed refs) the user
// never typed.
//
// The attribution rule: tmplPath is the user's typed bytes and is
// preserved exactly as typed; the cwd is filesystem-derived and gets
// secret.Escape before joining. Attribution is exact (not
// heuristic) because the join is computed here rather than recovered
// from filepath.Abs output:
//
//   - An absolute tmplPath acquires no cwd bytes; it is only
//     filepath.Clean-ed, matching filepath.Abs. Clean is safe on
//     template bytes: its decisions depend only on separators and
//     "." / ".." segments, and no segment containing '$' can be "."
//     or "..".
//   - A relative tmplPath becomes filepath.Join(Escape(cwd),
//     tmplPath). Escape never adds or removes separators or changes
//     whether a segment equals "." or "..", so the Clean inside Join
//     makes identical structural decisions on the escaped and
//     unescaped forms ("../" segments in tmplPath consume whole
//     escaped cwd segments). And '$' runs cannot span the '/'
//     boundary between the two parts, so unescaping the joined
//     result unescapes each part independently. Hence the round
//     trip is exact: Unescape(Join(Escape(cwd), t)) ==
//     Join(cwd, Unescape(t)) for any ref-free template t.
//
// Known limitation: on Windows, a drive-relative tmplPath (e.g.
// "C:foo") is resolved against the per-drive working directory by
// filepath.Abs, which this join does not replicate. When the cwd
// contains no '$' (always, in practice), the function defers to
// filepath.Abs and behavior is unchanged; a '$'-bearing cwd plus a
// drive-relative path gets plain Join(cwd, path) semantics. The munge
// code is already documented as OS-dependent.
//
// tmplPath must be ref-free: MungeTemplateForDriver rejects locations
// bearing well-formed ${scheme:path} placeholders before absolutizing,
// and isFpath excludes them.
func absTemplatePath(tmplPath string) (string, error) {
	if filepath.IsAbs(tmplPath) {
		return filepath.Clean(tmplPath), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", errz.Err(err)
	}
	if !strings.Contains(cwd, "$") {
		// Escape(cwd) == cwd, so the escape-aware join below reduces
		// to filepath.Abs. Call it directly so behavior on '$'-free
		// cwds (the overwhelmingly common case) is bit-for-bit
		// unchanged, including Windows drive-relative handling.
		fp, err := filepath.Abs(tmplPath)
		if err != nil {
			return "", errz.Err(err)
		}
		return fp, nil
	}
	return filepath.Join(secret.Escape(cwd), tmplPath), nil
}
