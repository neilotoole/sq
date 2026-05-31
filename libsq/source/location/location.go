// Package location contains functionality related to source location.
package location

// NOTE: This package contains code from several eras. There's a bunch of
// overlap and duplication. It should be consolidated.

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
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
	"duckdb",
	"clickhouse",
	"oracle",
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
func WithPassword(loc, passw string) (string, error) {
	if _, ok := isFpath(loc); ok {
		return loc, nil
	}

	u, err := url.ParseRequestURI(loc)
	if err != nil {
		return "", errz.Err(err)
	}

	if passw == "" {
		// This will effectively remove any existing password in loc
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

		// It's not a http URL, so it must be a filepath
		loc = filepath.Clean(loc)
		return filepath.Base(loc)
	}

	// It's a SQL driver
	u, err := dburl.Parse(loc)
	if err != nil {
		return loc
	}

	if u.Scheme == "sqlite3" || u.Scheme == "duckdb" {
		// special handling for file-based DBs (sqlite3, duckdb)
		return path.Base(u.DSN)
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

	// Else path is empty, db name was prob part of params
	u2, err := url.ParseRequestURI(loc)
	if err != nil {
		return loc
	}
	vals, err := url.ParseQuery(u2.RawQuery)
	if err != nil {
		return loc
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
			return nil, errz.Errorf("parse location: invalid scheme: %s", loc)
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
		// It's a http or https URL
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
		return nil, errz.Err(err)
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
		return nil, errz.Errorf("parse location: invalid scheme: %s", loc)
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

// Abs returns the absolute path of loc. That is, relative
// paths etc. are resolved. If loc is not a file path or
// it cannot be processed, loc is returned unmodified.
func Abs(loc string) string {
	if fpath, ok := isFpath(loc); ok {
		return fpath
	}

	return loc
}

// isFpath returns the absolute filepath and true if loc is a file path.
func isFpath(loc string) (fpath string, ok bool) {
	// This is not exactly an industrial-strength algorithm...
	if strings.Contains(loc, "${") {
		// Excludes ${scheme:path} placeholders (e.g. ${env:DSN},
		// ${keyring:abc}). These resolve at use time and must not be
		// filepath-ified.
		return "", false
	}

	if strings.Contains(loc, ":/") {
		// Excludes "http:/" etc
		return "", false
	}

	if strings.Contains(loc, "sqlite:") {
		// Excludes "sqlite:my_file.db"
		// Be wary of windows paths, e.g. "D:\a\b\c.file"
		return "", false
	}

	if strings.Contains(loc, "duckdb:") {
		// Excludes "duckdb:my_file.duckdb" (malformed; missing the double-slash)
		return "", false
	}

	fpath, err := filepath.Abs(loc)
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
// the password component (if any) masked.
//
// If loc contains one or more ${scheme:path} secret-resolver placeholders,
// each placeholder is temporarily replaced with a digit-only sentinel
// before standard URL/DSN redaction runs, then restored afterward.
// Placeholders survive redaction in every URL position (host, port,
// path, query) because the digit sentinel is valid in each. A
// placeholder in the password position is masked along with the
// password; the placeholder text is lost in the redacted form (use
// 'sq config secrets ls' to enumerate references).
func Redact(loc string) string {
	if !strings.Contains(loc, "${") {
		return redactRaw(loc)
	}
	refs, err := secret.ExtractRefs(loc)
	if err != nil || len(refs) == 0 {
		return redactRaw(loc)
	}

	// Substitute each placeholder with a digit-only sentinel. We use
	// digits because they're the only character class valid in all
	// URL positions, including ports (RFC 3986 §3.2.3 limits port to
	// DIGIT). A sentinel like "9999<idx>9999" is unlikely to collide
	// with a real component value.
	sentinelled := loc
	sentinels := make([]string, len(refs))
	placeholders := make([]string, len(refs))
	for i, ref := range refs {
		placeholders[i] = "${" + ref.Scheme + ":" + ref.Path + "}"
		sentinels[i] = fmt.Sprintf("9999%07d9999", i)
		sentinelled = strings.Replace(sentinelled, placeholders[i], sentinels[i], 1)
	}
	redacted := redactRaw(sentinelled)
	// Restore surviving sentinels. Sentinels that got eaten by redaction
	// (because they were in the password position) are not restored —
	// the placeholder text is consumed by the password mask.
	for i := range refs {
		redacted = strings.Replace(redacted, sentinels[i], placeholders[i], 1)
	}
	return redacted
}

// redactRaw is the underlying URL/DSN redactor with no placeholder
// awareness. Redact wraps this with sentinel substitution when the input
// contains ${scheme:path} placeholders.
func redactRaw(loc string) string {
	switch {
	case loc == "",
		strings.HasPrefix(loc, "/"),
		strings.HasPrefix(loc, "sqlite3://"),
		strings.HasPrefix(loc, "duckdb://"):

		// REVISIT: If it's a sqlite/duckdb URI, could it have auth details in there?
		// e.g. "?_auth_pass=foo"
		return loc
	case strings.HasPrefix(loc, "http://"), strings.HasPrefix(loc, "https://"):
		u, err := url.ParseRequestURI(loc)
		if err != nil {
			// If we can't parse it, just return the original loc
			return loc
		}

		return u.Redacted()
	}

	// At this point, we expect it's a DSN
	dbu, err := dburl.Parse(loc)
	if err != nil {
		// Shouldn't happen, but if it does, simply return the
		// unmodified loc.
		return loc
	}

	return dbu.Redacted()
}

// WithPasswordPlaceholder returns loc with its password component
// replaced by the given placeholder string spliced verbatim (no URL
// encoding on the placeholder itself). The placeholder is expected to
// be a ${scheme:path} secret reference; the verbatim splice preserves
// the literal '{', ':', and '}' characters that URL encoding would
// otherwise mangle.
//
// Any username component is re-encoded using net/url's canonical
// userinfo encoding, so a URL like postgres://us%40er:pw@db/sakila
// (decoded username "us@er") becomes a valid postgres://us%40er:${...}@db/sakila.
//
// If loc is not a URL (e.g. a file path), it is returned unchanged.
func WithPasswordPlaceholder(loc, placeholder string) (string, error) {
	if _, ok := isFpath(loc); ok {
		return loc, nil
	}

	u, err := url.ParseRequestURI(loc)
	if err != nil {
		return "", errz.Err(err)
	}

	var username string
	if u.User != nil {
		username = u.User.Username()
	}

	// Reassemble the URL from its parsed components so we don't depend
	// on locating the userinfo-terminator '@' by string-cutting (which
	// would mis-handle URLs that legitimately contain '@' in the path
	// or query). url.User(username).String() handles userinfo encoding.
	var b strings.Builder
	b.WriteString(u.Scheme)
	b.WriteString("://")
	if username != "" {
		b.WriteString(url.User(username).String())
	}
	b.WriteString(":")
	b.WriteString(placeholder)
	b.WriteString("@")
	b.WriteString(u.Host)
	b.WriteString(u.EscapedPath())
	if u.RawQuery != "" {
		b.WriteString("?")
		b.WriteString(u.RawQuery)
	}
	if u.Fragment != "" {
		b.WriteString("#")
		b.WriteString(u.EscapedFragment())
	}
	return b.String(), nil
}
