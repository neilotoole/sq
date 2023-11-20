package source

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

var dbSchemes = []string{
	"mysql",
	"sqlserver",
	"postgres",
	"sqlite3",
}

// LocationFileName returns the final component of the file/URL path.
func LocationFileName(src *Source) (string, error) {
	if IsSQLLocation(src.Location) {
		return "", errz.Errorf("location is not a file: %s", src.Location)
	}

	ploc, err := parseLoc(src.Location)
	if err != nil {
		return "", err
	}

	return ploc.name + ploc.ext, nil
}

// IsSQLLocation returns true if source location loc seems to be
// a DSN for a SQL driver.
func IsSQLLocation(loc string) bool {
	for _, dbScheme := range dbSchemes {
		if strings.HasPrefix(loc, dbScheme+"://") {
			return true
		}
	}

	return false
}

// LocationWithPassword returns the location string with the password
// value set, overriding any existing password. If loc is not a URL
// (e.g. it's a file path), it is returned unmodified.
func LocationWithPassword(loc, passw string) (string, error) {
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

// ShortLocation returns a short location string. For example, the
// base name (data.xlsx) for a file, or for a DSN, user@host[:port]/db.
func ShortLocation(loc string) string {
	if !IsSQLLocation(loc) {
		// NOT a SQL location, must be a document (local filepath or URL).

		// Let's check if it's http
		u, ok := httpURL(loc)
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

// parsedLoc is a parsed representation of a source location.
type parsedLoc struct {
	// loc is the original unparsed location value.
	loc string

	// typ is the associated source driver type, which may
	// be empty until later determination.
	typ drivertype.Type

	// scheme is the original location scheme
	scheme string

	// user is the username, if applicable.
	user string

	// pass is the password, if applicable.
	pass string

	// hostname is the hostname, if applicable.
	hostname string

	// port is the port number or 0 if not applicable.
	port int

	// name is the "source name", e.g. "sakila". Typically this
	// is the database name, but for a file location such
	// as "/path/to/things.xlsx" it would be "things".
	name string

	// ext is the file extension, if applicable.
	ext string

	// dsn is the connection "data source name" that can be used in a
	// call to sql/Open. Empty for non-SQL locations.
	dsn string
}

// parseLoc parses a location string. On return
// the typ field may not be set: further processing
// may be required.
func parseLoc(loc string) (*parsedLoc, error) {
	ploc := &parsedLoc{loc: loc}

	if !strings.Contains(loc, "://") {
		if strings.Contains(loc, ":/") {
			// malformed location, such as "sqlite3:/path/to/file"
			return nil, errz.Errorf("parse location: invalid scheme: %s", loc)
		}

		// no scheme: it's just a regular file path for a document such as an Excel file
		name := filepath.Base(loc)
		ploc.ext = filepath.Ext(name)
		if ploc.ext != "" {
			name = name[:len(name)-len(ploc.ext)]
		}

		ploc.name = name
		return ploc, nil
	}

	if u, ok := httpURL(loc); ok {
		// It's a http or https URL
		ploc.scheme = u.Scheme
		ploc.hostname = u.Hostname()
		if u.Port() != "" {
			var err error
			ploc.port, err = strconv.Atoi(u.Port())
			if err != nil {
				return nil, errz.Wrapf(err, "parse location: invalid port {%s}: {%s}", u.Port(), loc)
			}
		}

		name := path.Base(u.Path)
		ploc.ext = path.Ext(name)
		if ploc.ext != "" {
			name = name[:len(name)-len(ploc.ext)]
		}

		ploc.name = name
		return ploc, nil
	}

	// sqlite3 is a special case, handle it now
	const sqlitePrefix = "sqlite3://"
	if strings.HasPrefix(loc, sqlitePrefix) {
		fpath := strings.TrimPrefix(loc, sqlitePrefix)

		ploc.scheme = "sqlite3"
		ploc.typ = typeSL3
		ploc.dsn = fpath

		// fpath could include params, e.g. "sqlite3://C:\sakila.db?param=val"
		if i := strings.IndexRune(fpath, '?'); i >= 0 {
			// Snip off the params
			fpath = fpath[:i]
		}

		name := filepath.Base(fpath)
		ploc.ext = filepath.Ext(name)
		if ploc.ext != "" {
			name = name[:len(name)-len(ploc.ext)]
		}

		ploc.name = name
		return ploc, nil
	}

	u, err := dburl.Parse(loc)
	if err != nil {
		return nil, errz.Err(err)
	}

	ploc.scheme = u.OriginalScheme
	ploc.dsn = u.DSN
	ploc.user = u.User.Username()
	ploc.pass, _ = u.User.Password()
	ploc.hostname = u.Hostname()
	if u.Port() != "" {
		ploc.port, err = strconv.Atoi(u.Port())
		if err != nil {
			return nil, errz.Wrapf(err, "parse location: invalid port {%s}: %s", u.Port(), loc)
		}
	}

	switch ploc.scheme {
	default:
		return nil, errz.Errorf("parse location: invalid scheme: %s", loc)
	case "sqlserver":
		ploc.typ = typeMS

		u2, err := url.ParseRequestURI(loc)
		if err != nil {
			return nil, errz.Wrapf(err, "parse location: %s", loc)
		}

		vals, err := url.ParseQuery(u2.RawQuery)
		if err != nil {
			return nil,
				errz.Wrapf(err, "parse location: %s", loc)
		}
		ploc.name = vals.Get("database")
	case "postgres":
		ploc.typ = typePg
		ploc.name = strings.TrimPrefix(u.Path, "/")
	case "mysql":
		ploc.typ = typeMy
		ploc.name = strings.TrimPrefix(u.Path, "/")
	}

	return ploc, nil
}

// AbsLocation returns the absolute path of loc. That is, relative
// paths etc. are resolved. If loc is not a file path or
// it cannot be processed, loc is returned unmodified.
func AbsLocation(loc string) string {
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
