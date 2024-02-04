// Package location contains functionality related to source location.
package location

// NOTE: This package contains code from several eras. There's a bunch of
// overlap and duplication. It should be consolidated.

import (
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xo/dburl"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// dbSchemes is a list of known SQL driver schemes.
var dbSchemes = []string{
	"mysql",
	"sqlserver",
	"postgres",
	"sqlite3",
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
func Short(loc string) string {
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

	if u.Scheme == "sqlite3" {
		// special handling for sqlite
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
		return loc
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

	// sqlite3 is a special case, handle it now
	const sqlitePrefix = "sqlite3://"
	if strings.HasPrefix(loc, sqlitePrefix) {
		fpath := strings.TrimPrefix(loc, sqlitePrefix)

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
	if strings.Contains(loc, ":/") {
		// Excludes "http:/" etc
		return "", false
	}

	if strings.Contains(loc, "sqlite:") {
		// Excludes "sqlite:my_file.db"
		// Be wary of windows paths, e.g. "D:\a\b\c.file"
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
	if err != nil || u.Host == "" || !(u.Scheme == "http" || u.Scheme == "https") {
		return nil, false
	}

	return u, true
}

// Redact returns a redacted version of the source
// location loc, with the password component (if any) of
// the location masked.
func Redact(loc string) string {
	switch {
	case loc == "",
		strings.HasPrefix(loc, "/"),
		strings.HasPrefix(loc, "sqlite3://"):

		// REVISIT: If it's a sqlite URI, could it have auth details in there?
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
