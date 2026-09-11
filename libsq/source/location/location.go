// Package location contains functionality related to source location.
package location

// NOTE: This package contains code from several eras. There's a bunch of
// overlap and duplication. It should be consolidated.

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/xo/dburl"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// dbSchemes is a list of known SQL driver schemes.
var dbSchemes = []string{
	"mysql",
	"sqlserver",
	"postgres",
	"sqlite3",
	"rqlite",
	"duckdb",
	"clickhouse",
	"oracle",
}

// fileDBSchemes is the set of file-based DB driver schemes whose bare
// "scheme:path" form (no "://") denotes a DSN rather than a file path:
// "sqlite3:f.db", "duckdb:f.db", and the legacy "sqlite:f.db" spelling.
// Network driver schemes (postgres, mysql, etc.) and http/https are
// deliberately absent: they only ever take the "scheme://..." form, which
// isFpath already excludes via its "://" check. Listing them here would
// wrongly exclude a real file whose name begins "postgres:" or "http:"
// (the gh #859 false-exclusion class).
var fileDBSchemes = map[string]bool{
	"sqlite3": true,
	"sqlite":  true,
	"duckdb":  true,
}

// isFileDBScheme reports whether scheme is a file-based DB driver scheme
// whose bare "scheme:path" form isFpath must treat as a DSN, not a file
// path. Matching is case-sensitive, consistent with the rest of the
// location pipeline: IsSQL, Parse, and munging all compare scheme
// prefixes case-sensitively, so an off-case spelling like "SQLITE3:" is
// not a DSN sq recognizes and must be left as an ordinary filename (which
// isFpath then absolutizes).
func isFileDBScheme(scheme string) bool {
	return fileDBSchemes[scheme]
}

// Filename returns the final component of the file/URL path.
func Filename(loc string) (string, error) {
	if IsSQL(loc) {
		return "", errz.Errorf("location is not a file: %s", loc)
	}

	fields, err := Parse(loc)
	if err != nil {
		return "", err
	}

	return fields.Name + fields.Ext, nil
}

// IsSQL returns true if source location loc seems to be
// a DSN for a SQL driver.
func IsSQL(loc string) bool {
	for _, dbScheme := range dbSchemes {
		if strings.HasPrefix(loc, dbScheme+"://") {
			return true
		}
	}

	return false
}

// WithPassword returns the location string with the password
// value set, overriding any existing password. If loc is not a URL
// (e.g. it's a file path), it is returned unmodified.
//
// Returns an error when passw is non-empty but loc has no username
// in its userinfo: producing "scheme://:passw@host/db" is almost
// never intentional, so callers get a clear error instead.
func WithPassword(loc, passw string) (string, error) {
	if _, ok := isFpath(loc); ok {
		return loc, nil
	}

	u, err := url.ParseRequestURI(loc)
	if err != nil {
		return "", errz.Err(err)
	}

	if passw != "" && (u.User == nil || u.User.Username() == "") {
		return "", errz.Errorf(
			"cannot set password: location has no username (got %q)",
			Redact(loc),
		)
	}

	if passw == "" {
		if u.User == nil {
			return loc, nil
		}
		u.User = url.User(u.User.Username())
	} else {
		u.User = url.UserPassword(u.User.Username(), passw)
	}

	return u.String(), nil
}

// Short returns a short location string. For example, the
// base name (data.xlsx) for a file, or for a DSN, user@host[:port]/db.
//
// Locations that start with a ${scheme:path} placeholder are returned
// verbatim — they aren't filesystem paths or URLs and must not be run
// through filepath.Base/dburl.Parse (which would, for example, slice
// ${file:/abs/path/pg.dsn} down to "pg.dsn}").
func Short(loc string) string {
	if strings.HasPrefix(loc, "${") {
		return loc
	}
	if !IsSQL(loc) {
		// NOT a SQL location, must be a document (local filepath or URL).

		// Let's check if it's http
		u, ok := isHTTP(loc)
		if ok {
			name := path.Base(u.Path)
			if name == "" || name == "/" || name == "." {
				// Well, if we don't have a good name from u.Path, we'll
				// fall back to the hostname.
				name = u.Hostname()
			}
			return name
		}

		// It's not an HTTP URL. If it has a scheme separator, it's likely
		// a URL with an unknown driver scheme (not a filepath); redact
		// best-effort rather than mangle it via filepath.Base, which would
		// echo inline credentials like "user:pass@host" verbatim.
		if strings.Contains(loc, "://") {
			return redactBestEffort(loc)
		}

		// True filepath.
		loc = filepath.Clean(loc)
		return shortFileName(filepath.Base(loc))
	}

	// It's a SQL driver.
	//
	// rqlite is a network SQL driver, but xo/dburl doesn't know its
	// scheme, so Parse-ing through dburl returns an error and the
	// fallback would echo inline credentials. Handle it via
	// url.ParseRequestURI here, mirroring the user@host[:port] shape
	// used for the other DSN drivers below.
	if strings.HasPrefix(loc, "rqlite://") {
		ru, err := url.ParseRequestURI(loc)
		if err != nil {
			// Couldn't parse; fall back to best-effort credential masking
			// rather than returning loc verbatim.
			return redactBestEffort(loc)
		}
		sb := strings.Builder{}
		if ru.User != nil && len(ru.User.Username()) > 0 {
			sb.WriteString(ru.User.Username())
			sb.WriteString("@")
		}
		sb.WriteString(ru.Host)
		return sb.String()
	}

	u, err := dburl.Parse(loc)
	if err != nil {
		// dburl rejected the scheme. Don't echo loc verbatim — it
		// may carry inline credentials. redactBestEffort applies
		// regex masking of user:pass@ and PWD= shapes.
		return redactBestEffort(loc)
	}

	if u.Scheme == "sqlite3" || u.Scheme == "duckdb" {
		// Special handling for file-based DBs (sqlite3, duckdb). u.DSN
		// carries the query string, which may hold secret params (e.g.
		// SQLCipher's _auth_pass); drop it before taking the base name.
		dsn, _, _ := strings.Cut(u.DSN, "?")
		return shortFileName(path.Base(dsn))
	}

	sb := strings.Builder{}
	if u.User != nil && len(u.User.Username()) > 0 {
		sb.WriteString(u.User.Username())
		sb.WriteString("@")
	}

	sb.WriteString(u.Host)
	if u.Path != "" {
		sb.WriteString(u.Path)
		// path contains the db name
		return sb.String()
	}

	// Else path is empty, db name was prob part of params.
	// On any parse failure, fall back to the user@host form already in
	// sb rather than returning loc verbatim: loc may carry inline
	// credentials that must not leak from a display string.
	u2, err := url.ParseRequestURI(loc)
	if err != nil {
		return sb.String()
	}
	vals, err := url.ParseQuery(u2.RawQuery)
	if err != nil {
		return sb.String()
	}

	db := vals.Get("database")
	if db == "" {
		// This can happen for an MSSQL URL, of the form
		// "sqlserver://sq:***@localhost" (without the "?database=db" part).
		return sb.String()
	}

	sb.WriteRune('/')
	sb.WriteString(db)
	return sb.String()
}

// shortFileName masks credential-shaped text in a file's base name for
// display. The mask only runs when the name actually contains a
// credential-shaped byte ('@' or '='), so ordinary filenames skip the
// regex entirely. This keeps the common Short() path cheap while still
// masking a pathological name that embeds userinfo, e.g. "user:pw@x.db".
func shortFileName(name string) string {
	if strings.ContainsAny(name, "@=") {
		return redactBestEffort(name)
	}
	return name
}

// Fields is a parsed representation of a source location.
type Fields struct {
	// Loc is the original unparsed location value.
	Loc string

	// DriverType is the associated source driver type, which may
	// be empty until later determination.
	DriverType drivertype.Type

	// Scheme is the original location Scheme.
	Scheme string

	// User is the username, if applicable.
	User string

	// Pass is the password, if applicable.
	Pass string

	// Hostname is the Hostname, if applicable.
	Hostname string

	// Name is the "source Name", e.g. "sakila". Typically this
	// is the database Name, but for a file location such
	// as "/path/to/things.xlsx" it would be "things".
	Name string

	// Ext is the file extension, if applicable.
	Ext string

	// DSN is the connection "data source Name" that can be used in a
	// call to sql.Open. Empty for non-SQL locations.
	DSN string

	// Port is the Port number or 0 if not applicable.
	Port int
}

// Parse parses a location string, returning a Fields instance representing
// the decomposition of the location. On return the Fields.DriverType field
// may not be set: further processing may be required.
func Parse(loc string) (*Fields, error) {
	fields := &Fields{Loc: loc}

	if !strings.Contains(loc, "://") {
		if strings.Contains(loc, ":/") {
			// malformed location, such as "sqlite3:/path/to/file"
			return nil, errz.Errorf("parse location: invalid scheme: %s", redactBestEffort(loc))
		}

		// no scheme: it's just a regular file path for a document such as an Excel file
		name := filepath.Base(loc)
		fields.Ext = filepath.Ext(name)
		if fields.Ext != "" {
			name = name[:len(name)-len(fields.Ext)]
		}

		fields.Name = name
		return fields, nil
	}

	if u, ok := isHTTP(loc); ok {
		// It's an HTTP or HTTPS URL.
		fields.Scheme = u.Scheme
		fields.Hostname = u.Hostname()
		if u.Port() != "" {
			var err error
			fields.Port, err = strconv.Atoi(u.Port())
			if err != nil {
				return nil, errz.Wrapf(err, "parse location: invalid port {%s}: {%s}", u.Port(), loc)
			}
		}

		name := path.Base(u.Path)
		fields.Ext = path.Ext(name)
		if fields.Ext != "" {
			name = name[:len(name)-len(fields.Ext)]
		}

		fields.Name = name
		return fields, nil
	}

	// sqlite3 and duckdb are special cases: they are file-based DBs and their
	// URIs don't follow the network-URL pattern used by postgres/mysql/etc.
	const (
		sqlitePrefix = "sqlite3://"
		duckdbPrefix = "duckdb://"
	)
	if after, ok := strings.CutPrefix(loc, sqlitePrefix); ok {
		fpath := after

		fields.Scheme = "sqlite3"
		fields.DriverType = drivertype.SQLite
		fields.DSN = fpath

		// fpath could include params, e.g. "sqlite3://C:\sakila.db?param=val"
		if i := strings.IndexRune(fpath, '?'); i >= 0 {
			// Snip off the params
			fpath = fpath[:i]
		}

		name := filepath.Base(fpath)
		fields.Ext = filepath.Ext(name)
		if fields.Ext != "" {
			name = name[:len(name)-len(fields.Ext)]
		}

		fields.Name = name
		return fields, nil
	}

	// rqlite is a network SQL driver, but xo/dburl doesn't know its
	// scheme, so we parse it here rather than fall through to
	// dburl.Parse below.
	if strings.HasPrefix(loc, "rqlite://") {
		return parseRqlite(loc, fields)
	}

	if after, ok := strings.CutPrefix(loc, duckdbPrefix); ok {
		fpath := after

		fields.Scheme = "duckdb"
		fields.DriverType = drivertype.DuckDB
		fields.DSN = fpath

		// fpath could include params, e.g. "duckdb://C:\sakila.duckdb?param=val"
		if i := strings.IndexRune(fpath, '?'); i >= 0 {
			// Snip off the params
			fpath = fpath[:i]
		}

		name := filepath.Base(fpath)
		fields.Ext = filepath.Ext(name)
		if fields.Ext != "" {
			name = name[:len(name)-len(fields.Ext)]
		}

		fields.Name = name
		return fields, nil
	}

	u, err := dburl.Parse(loc)
	if err != nil {
		// dburl's error may embed the raw input URL with inline
		// credentials. Wrap with a redacted-loc message instead of
		// surfacing dburl's err verbatim.
		return nil, errz.Errorf("parse location: invalid: %s", redactBestEffort(loc))
	}

	fields.Scheme = u.OriginalScheme
	fields.DSN = u.DSN
	fields.User = u.User.Username()
	fields.Pass, _ = u.User.Password()
	fields.Hostname = u.Hostname()
	if u.Port() != "" {
		fields.Port, err = strconv.Atoi(u.Port())
		if err != nil {
			return nil, errz.Wrapf(err, "parse location: invalid port {%s}: %s", u.Port(), loc)
		}
	}

	switch fields.Scheme {
	default:
		return nil, errz.Errorf("parse location: invalid scheme: %s", redactBestEffort(loc))
	case "sqlserver":
		fields.DriverType = drivertype.MSSQL

		u2, err := url.ParseRequestURI(loc)
		if err != nil {
			return nil, errz.Wrapf(err, "parse location: %s", loc)
		}

		vals, err := url.ParseQuery(u2.RawQuery)
		if err != nil {
			return nil,
				errz.Wrapf(err, "parse location: %s", loc)
		}
		fields.Name = vals.Get("database")
	case "postgres":
		fields.DriverType = drivertype.Pg
		fields.Name = strings.TrimPrefix(u.Path, "/")
	case "mysql":
		fields.DriverType = drivertype.MySQL
		fields.Name = strings.TrimPrefix(u.Path, "/")
	case "clickhouse":
		fields.DriverType = drivertype.ClickHouse
		fields.Name = strings.TrimPrefix(u.Path, "/")
		if fields.Name == "" {
			// ClickHouse also supports specifying the database via the
			// ?database= query parameter (e.g.
			// "clickhouse://localhost:9000?database=mydb").
			u2, err := url.ParseRequestURI(loc)
			if err == nil {
				fields.Name = u2.Query().Get("database")
			}
		}
	case "oracle":
		fields.DriverType = drivertype.Oracle
		fields.Name = strings.TrimPrefix(u.Path, "/")
	}

	return fields, nil
}

// parseRqlite parses an rqlite:// location into fields. xo/dburl doesn't
// recognize the rqlite scheme, so this bypass uses url.ParseRequestURI
// directly. fields is partially populated by the caller; this function
// fills in the rqlite-specific bits.
func parseRqlite(loc string, fields *Fields) (*Fields, error) {
	u, err := url.ParseRequestURI(loc)
	if err != nil {
		// url.Error embeds the raw input URL, which may carry inline
		// credentials. Strip that wrapper so only the redacted loc
		// appears in the message, but keep the underlying cause.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return nil, errz.Wrapf(err, "parse location: %s", redactBestEffort(loc))
	}

	fields.Scheme = u.Scheme
	fields.DriverType = drivertype.Rqlite
	fields.DSN = loc
	fields.Hostname = u.Hostname()
	if u.User != nil {
		fields.User = u.User.Username()
		fields.Pass, _ = u.User.Password()
	}
	if u.Port() != "" {
		fields.Port, err = strconv.Atoi(u.Port())
		if err != nil {
			return nil, errz.Wrapf(err, "parse location: invalid port {%s}: %s", u.Port(),
				redactBestEffort(loc))
		}
	}
	fields.Name = strings.TrimPrefix(u.Path, "/")
	return fields, nil
}

// Abs returns the absolute path of loc. That is, relative
// paths etc. are resolved. If loc is not a file path or
// it cannot be processed, loc is returned unmodified.
//
// Abs treats loc as placeholder-template bytes (the form in which
// source locations are stored): the cwd bytes joined in when
// absolutizing a relative path are literal filesystem bytes, so any
// '$' they contain is escaped via secret.Escape, while loc's typed
// bytes pass through exactly as typed. See absTemplatePath.
func Abs(loc string) string {
	if fpath, ok := isFpath(loc); ok {
		return fpath
	}

	return loc
}

// isFpath returns the absolute filepath and true if loc is a file
// path. The returned fpath is template bytes: cwd-derived bytes are
// escaped per absTemplatePath.
func isFpath(loc string) (fpath string, ok bool) {
	// This is not exactly an industrial-strength algorithm...

	// Excludes well-formed ${scheme:path} placeholders (e.g.
	// ${env:DSN}, ${keyring:abc}) — those resolve at use time and
	// must not be filepath-ified. Excludes malformed placeholder
	// syntax (refsErr != nil) too: a file literally named with
	// unclosed "${" would otherwise be opened as a path here, and
	// then fail much later with an opaque OS-level error. Bailing
	// here lets downstream surface a proper placeholder-parse
	// error. Files containing an escaped "$${...}" parse cleanly
	// (no refs) and are still treated as paths.
	if refs, refsErr := secret.ExtractRefs(loc); refsErr != nil || len(refs) > 0 {
		return "", false
	}

	// Inspect the leading "scheme:" token (the bytes before the first
	// colon), anchoring the URL/DSN checks on that token rather than
	// substring-matching anywhere in loc.
	if scheme, rest, found := strings.Cut(loc, ":"); found {
		if strings.HasPrefix(rest, "//") {
			// A "scheme://" authority form is a URL (a SQL driver DSN,
			// http/https, or an unknown/unsupported scheme), never a local
			// file path. Excluding unknown schemes here too avoids mangling
			// a mistyped URL into a garbage path before it fails downstream.
			return "", false
		}

		if isFileDBScheme(scheme) {
			// A leading "scheme:" token naming a file-based DB driver is a
			// bare DSN, not a path: "sqlite3:f.db", "duckdb:f.db", the
			// legacy "sqlite:f.db" spelling, and malformed single-slash
			// forms like "sqlite3:/path". A single-letter Windows volume
			// ("C:\db") is not such a scheme, so it stays a path,
			// dissolving the gh #797 trap without a dedicated drive-letter
			// branch. A colon inside a filename ("./dump.sqlite3:old.db"),
			// or a leading token that only looks like a network scheme
			// ("postgres:notes.csv"), likewise stays a path (gh #859).
			return "", false
		}
	}

	fpath, err := absTemplatePath(loc)
	if err != nil {
		return "", false
	}

	return fpath, true
}

// Type is an enumeration of the various types of source location.
type Type string

const (
	TypeStdin   = "stdin"
	TypeFile    = "local_file"
	TypeSQL     = "sql"
	TypeHTTP    = "http_file"
	TypeUnknown = "unknown"
)

// TypeOf returns the type of loc, or locTypeUnknown if it
// can't be determined.
func TypeOf(loc string) Type {
	switch {
	case loc == "@stdin":
		// Convention: the "location" of stdin is always "@stdin"
		return TypeStdin
	case IsSQL(loc):
		return TypeSQL
	case strings.HasPrefix(loc, "http://"),
		strings.HasPrefix(loc, "https://"):
		return TypeHTTP
	default:
	}

	if _, err := filepath.Abs(loc); err != nil {
		return TypeUnknown
	}
	return TypeFile
}

// IsURL returns true if t is TypeHTTP or TypeSQL.
func (t Type) IsURL() bool {
	return t == TypeHTTP || t == TypeSQL
}

// isHTTP tests if s is a well-structured HTTP or HTTPS url, and
// if so, returns the url and true.
func isHTTP(s string) (u *url.URL, ok bool) {
	var err error
	u, err = url.Parse(s)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, false
	}

	return u, true
}

// Redact returns a redacted version of the source location loc, with
// the password component (if any) masked, and the values of
// secret-bearing query parameters (see IsSecretQueryParam) masked with
// the same "xxxxx" convention, e.g.
// "sqlite3:///data/app.db?_auth_pass=xxxxx". Non-secret bytes,
// including query parameter order and escaping, are preserved.
//
// Redact differs from StripSecrets, which removes secrets entirely so
// the location remains valid for reuse (e.g. as a shell-completion
// candidate); Redact output is for display and logging only.
//
// If loc contains one or more ${scheme:path} secret-resolver placeholders,
// each placeholder is temporarily replaced with a digit-only sentinel
// before standard URL/DSN redaction runs, then restored afterward.
// Placeholders survive redaction in every URL position (host, port,
// path, query) because the digit sentinel is valid in each. A
// placeholder in the password position is masked along with the
// password; the placeholder text is lost in the redacted form (use
// 'sq config keyring ls' to enumerate references). A secret query
// value composed entirely of placeholders likewise passes through
// verbatim; a mixed literal/placeholder value is masked like any
// literal secret.
func Redact(loc string) string {
	if !strings.Contains(loc, "${") {
		return redactRaw(loc, nil)
	}
	refs, err := secret.ExtractRefs(loc)
	if err != nil || len(refs) == 0 {
		return redactRaw(loc, nil)
	}

	sentinels, ok := pickSentinels(loc, len(refs))
	if !ok {
		// Couldn't find non-colliding sentinels in the salt-search
		// budget. Fall back to redactRaw without placeholder
		// preservation: placeholders get masked along with the
		// password, which is correct (no credentials leak) but loses
		// the placeholder text. Astronomically unlikely path.
		return redactRaw(loc, nil)
	}
	placeholders := make([]string, len(refs))
	sentinelled := loc
	for i, ref := range refs {
		placeholders[i] = "${" + ref.Scheme + ":" + ref.Path + "}"
		sentinelled = strings.Replace(sentinelled, placeholders[i], sentinels[i], 1)
	}
	redacted := redactRaw(sentinelled, sentinels)
	// Restore surviving sentinels. Sentinels that got eaten by redaction
	// (because they were in the password position) are not restored —
	// the placeholder text is consumed by the password mask.
	for i := range refs {
		redacted = strings.Replace(redacted, sentinels[i], placeholders[i], 1)
	}
	return redacted
}

// pickSentinels chooses n digit-only sentinel strings, none of which
// appears as a substring of loc. Digit-only so the sentinels are valid
// in every URL position (RFC 3986 §3.2.3 limits port to DIGIT). The
// search bumps a salt prefix until no candidate collides with loc —
// guarding against the rare case where loc legitimately contains the
// default sentinel pattern (e.g. a long numeric query parameter).
//
// Returns ok=false after maxSaltAttempts unsuccessful tries — that
// branch is astronomically unlikely (any salt change shifts the entire
// candidate set), but bounding the loop is the right hygiene.
func pickSentinels(loc string, n int) ([]string, bool) {
	const maxSaltAttempts = 1000
	for salt := range maxSaltAttempts {
		candidates := make([]string, n)
		clash := false
		for i := range candidates {
			candidates[i] = fmt.Sprintf("9999%03d%07d9999", salt, i)
			if strings.Contains(loc, candidates[i]) {
				clash = true
				break
			}
		}
		if !clash {
			return candidates, true
		}
	}
	return nil, false
}

// redactRaw is the underlying URL/DSN redactor. It masks the userinfo
// password and then the values of secret-bearing query parameters.
// When Redact has swapped the location's ${scheme:path} placeholders for
// sentinels, it passes them here so that a query value composed entirely
// of sentinels is recognized as a placeholder template, not a literal
// secret, and left intact. sentinels is nil when loc has no placeholders
// to preserve, and on Redact's astronomically-unlikely fallback path
// where placeholders remain in loc and are masked like any literal
// secret.
func redactRaw(loc string, sentinels []string) string {
	if loc == "" {
		return loc
	}

	var redacted string
	switch {
	case strings.HasPrefix(loc, "/"),
		strings.HasPrefix(loc, "sqlite3://"),
		strings.HasPrefix(loc, "duckdb://"):
		// File-style locations have no userinfo password to mask, but
		// may carry secret-bearing query params (e.g. sqlite3's
		// "?_auth_pass=..."), masked below.
		redacted = loc
	case strings.HasPrefix(loc, "http://"), strings.HasPrefix(loc, "https://"):
		u, err := url.ParseRequestURI(loc)
		if err != nil {
			redacted = redactBestEffort(loc)
			break
		}
		redacted = u.Redacted()
	case strings.HasPrefix(loc, "rqlite://"):
		// rqlite is a network SQL driver unknown to dburl; redact its
		// userinfo explicitly rather than relying on the best-effort
		// regex fallback below.
		u, err := url.ParseRequestURI(loc)
		if err != nil {
			redacted = redactBestEffort(loc)
			break
		}
		if _, ok := u.User.Password(); ok {
			u.User = url.UserPassword(u.User.Username(), "xxxxx")
		}
		redacted = u.String()
	default:
		// At this point, we expect it's a DSN.
		dbu, err := dburl.Parse(loc)
		if err != nil {
			redacted = redactBestEffort(loc)
			break
		}
		redacted = dbu.Redacted()
	}

	return maskSecretQueryParams(redacted, sentinels)
}

// maskSecretQueryParams masks the value of each secret-bearing query
// parameter in loc (per IsSecretQueryParam) with "xxxxx", preserving
// non-secret bytes verbatim. A URL fragment is preserved: per URL
// syntax an unencoded '#' always starts the fragment, so it is carved
// off before query processing lest it be swallowed by a masked value.
func maskSecretQueryParams(loc string, sentinels []string) string {
	head, frag, hasFrag := strings.Cut(loc, "#")
	base, query, hasQuery := strings.Cut(head, "?")
	masked := base
	if hasQuery {
		masked += "?" + replaceSecretQueryValues(query, "xxxxx", sentinels)
	}
	if hasFrag {
		masked += "#" + frag
	}
	return masked
}

// redactRawUserinfo masks the password portion of any "user:pw@host"
// pattern, whether preceded by a scheme separator ("scheme://"), a
// query separator, or appearing at the very start of the input.
// Captures the leading anchor (start-of-string or one of : / @) and
// the username separately so the replacement preserves them both.
// The username group is optional ("*"), so a password-only userinfo
// ("scheme://:pw@host") is masked too. The password character class
// explicitly excludes "/" so a greedy match can't swallow a "://"
// scheme prefix when anchored at "^".
var redactRawUserinfo = regexp.MustCompile(`(^|[:/@])([^:/?@\s]*):[^:/?@\s]+@`)

// redactRawDSNPw masks "PWD=value" / "password=value" style key/value
// pairs used in ODBC, ADO.NET, and other ;-delimited connection
// strings. Case-insensitive; stops at ; & or whitespace.
var redactRawDSNPw = regexp.MustCompile(`(?i)(\b(?:pwd|password|passwd|pw)\s*=)\s*[^;&\s]+`)

// redactBestEffort applies regex-based credential masking to inputs
// that the structured parsers (url.ParseRequestURI / dburl.Parse) could
// not handle. It catches the URL userinfo "user:pw@" pattern and the
// common DSN/ODBC "PWD=value" form. Other credential-bearing shapes
// will pass through unmasked — when that matters, fix the upstream
// parser, don't rely on this. The goal is "don't leak inline passwords
// in error messages and log lines when the loc is malformed enough
// that the structured redactors give up".
func redactBestEffort(loc string) string {
	loc = redactRawUserinfo.ReplaceAllString(loc, "${1}${2}:xxxxx@")
	loc = redactRawDSNPw.ReplaceAllString(loc, "${1}xxxxx")
	return loc
}

// IsSecretQueryParam reports whether the URL query parameter key
// conventionally carries a secret value in driver connection strings,
// e.g. "password" (sqlserver), "sslpassword" (postgres), "_auth_pass"
// (sqlite3). Matching is case-insensitive. Boolean toggles that merely
// mention passwords (e.g. mysql's "allowCleartextPasswords") do not
// match.
func IsSecretQueryParam(key string) bool {
	k := strings.ToLower(key)
	switch k {
	case "password", "passwd", "pwd", "pw", "secret", "token",
		"apikey", "api_key", "access_token", "auth_token":
		return true
	default:
		return strings.HasSuffix(k, "password") ||
			strings.HasSuffix(k, "passwd") ||
			strings.HasSuffix(k, "_pass") ||
			strings.HasSuffix(k, "_secret") ||
			strings.HasSuffix(k, "_token")
	}
}

// StripSecrets returns loc with embedded secrets removed, for use
// where the location must remain valid and completable but must not
// expose secrets, e.g. shell-completion candidates. The userinfo
// password (if any) is dropped ("scheme://alice:pw@host" becomes
// "scheme://alice@host"), and the values of secret-bearing query
// parameters (see IsSecretQueryParam) are emptied
// ("?_auth_pass=hunter2" becomes "?_auth_pass="). Stripping is purely
// textual: non-secret portions of loc, including escaping and query
// parameter order, are preserved byte-for-byte.
//
// StripSecrets differs from Redact, which masks secrets with a fixed
// "xxxxx" string for display: a masked location accepted as a
// completion candidate would silently create a source with a bogus
// password, so here the secret is removed instead.
//
// A ${scheme:path} placeholder is not itself a secret (it's the text
// stored in config), so a password or query value consisting entirely
// of placeholders passes through verbatim. Mixed literal/placeholder
// values are dropped like any literal secret.
func StripSecrets(loc string) string {
	refs, err := secret.ExtractRefs(loc)
	if err != nil || len(refs) == 0 {
		return stripSecretsRaw(loc, nil)
	}
	sentinels, ok := pickSentinels(loc, len(refs))
	if !ok {
		// No non-colliding sentinels found (astronomically unlikely).
		// Strip without placeholder preservation: placeholder-valued
		// passwords get dropped along with literal ones, which leaks
		// nothing.
		return stripSecretsRaw(loc, nil)
	}
	placeholders := make([]string, len(refs))
	sentinelled := loc
	for i, ref := range refs {
		placeholders[i] = "${" + ref.Scheme + ":" + ref.Path + "}"
		sentinelled = strings.Replace(sentinelled, placeholders[i], sentinels[i], 1)
	}
	stripped := stripSecretsRaw(sentinelled, sentinels)
	// Restore surviving sentinels. Sentinels dropped by stripping
	// (mixed literal/placeholder secrets) are simply absent.
	for i := range refs {
		stripped = strings.Replace(stripped, sentinels[i], placeholders[i], 1)
	}
	return stripped
}

// stripSecretsRaw implements StripSecrets on a location whose
// ${scheme:path} placeholders (if any) have been replaced with the
// given sentinels. A password or query value composed entirely of
// sentinels is a placeholder template, not a literal secret, and is
// left intact. A URL fragment is preserved verbatim: per URL syntax
// an unencoded '#' always starts the fragment, so it is carved off
// before query processing lest it be swallowed by a blanked value.
func stripSecretsRaw(loc string, sentinels []string) string {
	loc, frag, hasFrag := strings.Cut(loc, "#")
	base, query, hasQuery := strings.Cut(loc, "?")
	base = stripUserinfoPassword(base, sentinels)
	stripped := base
	if hasQuery {
		stripped += "?" + replaceSecretQueryValues(query, "", sentinels)
	}
	if hasFrag {
		stripped += "#" + frag
	}
	return stripped
}

// stripUserinfoPassword drops the ":password" portion of base's
// userinfo, if present. base must not contain a query string. A
// password composed entirely of sentinels is kept.
func stripUserinfoPassword(base string, sentinels []string) string {
	schemeIdx := strings.Index(base, "://")
	if schemeIdx < 0 {
		return base
	}
	authority := base[schemeIdx+3:]
	if end := strings.IndexByte(authority, '/'); end >= 0 {
		authority = authority[:end]
	}
	at := strings.LastIndexByte(authority, '@')
	if at < 0 {
		return base
	}
	userinfo := authority[:at]
	colon := strings.IndexByte(userinfo, ':')
	if colon < 0 {
		return base
	}
	if isOnlySentinels(userinfo[colon+1:], sentinels) {
		return base
	}
	i := schemeIdx + 3
	return base[:i+colon] + base[i+at:]
}

// replaceSecretQueryValues sets the value of each secret-bearing
// key=value pair (per IsSecretQueryParam) in the raw query string to
// newValue, preserving pair order and the escaping of untouched pairs.
// Values composed entirely of sentinels are kept, as are empty values.
// StripSecrets passes newValue="" (the location stays reusable);
// Redact passes the "xxxxx" display mask.
func replaceSecretQueryValues(query, newValue string, sentinels []string) string {
	pairs := strings.Split(query, "&")
	for i, pair := range pairs {
		k, v, ok := strings.Cut(pair, "=")
		if !ok || v == "" {
			continue
		}
		key, err := url.QueryUnescape(k)
		if err != nil {
			key = k
		}
		if !IsSecretQueryParam(key) {
			continue
		}
		if isOnlySentinels(v, sentinels) {
			continue
		}
		pairs[i] = k + "=" + newValue
	}
	return strings.Join(pairs, "&")
}

// isOnlySentinels reports whether s is non-empty and composed entirely
// of strings from sentinels.
func isOnlySentinels(s string, sentinels []string) bool {
	if s == "" || len(sentinels) == 0 {
		return false
	}
	for _, sentinel := range sentinels {
		s = strings.ReplaceAll(s, sentinel, "")
	}
	return s == ""
}

// MergeQuery returns loc with the given query params set, replacing
// any existing values for the same keys. Other query params are
// preserved. loc must be parseable by net/url and have a scheme:
// driver-native DSN forms that net/url cannot parse (e.g. mysql's
// "user@tcp(host)/db" shape) must not be passed here, and locations
// bearing secret placeholders are the caller's responsibility to
// exclude.
func MergeQuery(loc string, params url.Values) (string, error) {
	if len(params) == 0 {
		return loc, nil
	}
	u, err := url.Parse(loc)
	if err != nil {
		// url.Error embeds the raw input (which may carry inline
		// credentials); strip the wrapper and redact the loc.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return "", errz.Wrapf(err, "merge query: invalid location: %s",
			redactBestEffort(loc))
	}
	if u.Scheme == "" {
		// url.Parse accepts bare file paths; without a scheme there is
		// no URL to merge query params into.
		return "", errz.Errorf("merge query: location has no scheme: %s",
			redactBestEffort(loc))
	}
	q := u.Query()
	for k, vs := range params {
		q.Del(k)
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
