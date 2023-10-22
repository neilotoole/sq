// Package source provides functionality for dealing with data sources.
package source

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/xo/dburl"

	"github.com/neilotoole/sq/libsq/core/options"
)

// DriverType is a driver type, e.g. "mysql", "postgres", "csv", etc.
type DriverType string

// String returns a log/debug-friendly representation.
func (t DriverType) String() string {
	return string(t)
}

// TypeNone is the zero value of DriverType.
const TypeNone = DriverType("")

const (
	// StdinHandle is the reserved handle for stdin pipe input.
	StdinHandle = "@stdin"

	// ActiveHandle is the reserved handle for the active source.
	// FIXME: it should be possible to use "@0" as the active handle, but
	//  the SLQ grammar doesn't currently allow it. Possibly change this
	//  value to "@0" after modifying the SLQ grammar.
	ActiveHandle = "@active"

	// ScratchHandle is the reserved handle for the scratch source.
	ScratchHandle = "@scratch"

	// JoinHandle is the reserved handle for the join db source.
	JoinHandle = "@join"

	// MonotableName is the table name used for "mono-table" drivers
	// such as CSV. Thus a source @address_csv will have its
	// data accessible via @address_csv.data.
	MonotableName = "data"
)

// ReservedHandles returns a slice of the handle names that
// are reserved for sq use.
func ReservedHandles() []string {
	return []string{
		"@in", // Possible alias for @stdin
		"@0",  // Possible alias for @stdin
		StdinHandle,
		ActiveHandle,
		ScratchHandle,
		JoinHandle,
	}
}

var _ slog.LogValuer = (*Source)(nil)

// Source describes a data source.
type Source struct {
	// Handle is used to refer to a source, e.g. "@sakila".
	Handle string `yaml:"handle" json:"handle"`

	// Type is the driver type, e.g. postgres.Type.
	Type DriverType `yaml:"driver" json:"driver"`

	// Location is the source location, such as a DB connection URI,
	// or a file path.
	Location string `yaml:"location" json:"location"`

	// Catalog, when non-empty, specifies a catalog (database) name
	// override to use when constructing table references.
	//
	// For example, if the SLQ table selector is "actor", the query
	// might be rendered as 'SELECT * FROM actor'. But if Catalog is
	// set to "sakila" and Schema to "dbo", then that same query would
	// be rendered as 'SELECT * FROM sakila.dbo.actor'.
	//
	// Note that although Source.Catalog is not exposed to the end user,
	// this field is used by the SQL renderer to construct table references,
	// especially for SQL Server. Because SQL Server doesn't permit setting the
	// default schema on a per-connection basis, we construct a fully-qualified
	// table reference using the catalog and schema, e.g. "sakila.dbo.actor".
	//
	// See also: Source.Schema.
	Catalog string `yaml:"catalog,omitempty" json:"catalog,omitempty"`

	// Schema, when non-empty, specifies a schema name
	// override to use when constructing table references.
	//
	// See also: Source.Catalog.
	Schema string `yaml:"schema,omitempty" json:"schema,omitempty"`

	// Options are additional params, typically empty.
	Options options.Options `yaml:"options,omitempty" json:"options,omitempty"`
}

// LogValue implements slog.LogValuer.
func (s *Source) LogValue() slog.Value {
	if s == nil {
		return slog.Value{}
	}

	return slog.GroupValue(
		slog.String(lga.Handle, s.Handle),
		slog.String(lga.Driver, string(s.Type)),
		slog.String(lga.Loc, s.RedactedLocation()),
		slog.Any(lga.Opts, s.Options),
	)
}

// String returns a log/debug-friendly representation.
func (s *Source) String() string {
	return fmt.Sprintf("%s|%s|%s", s.Handle, s.Type, s.RedactedLocation())
}

// Group returns the source's group. If s is in the root group,
// the empty string is returned.
//
// FIXME: For root group, should "/" be returned instead of empty string?
func (s *Source) Group() string {
	return groupFromHandle(s.Handle)
}

func groupFromHandle(h string) string {
	// Trim the leading @
	h = h[1:]
	i := strings.LastIndex(h, "/")
	if i == -1 {
		return ""
	}

	return h[0:i]
}

// RedactedLocation returns s.Location, with the password component
// of the location masked.
func (s *Source) RedactedLocation() string {
	if s == nil {
		return ""
	}
	return RedactLocation(s.Location)
}

// Clone returns a deep copy of s. If s is nil, nil is returned.
func (s *Source) Clone() *Source {
	if s == nil {
		return nil
	}

	return &Source{
		Handle:   s.Handle,
		Type:     s.Type,
		Location: s.Location,
		Catalog:  s.Catalog,
		Schema:   s.Schema,
		Options:  s.Options.Clone(),
	}
}

// RedactSources returns a new slice, where each element
// is a clone of the input *Source with its location field
// redacted. This is useful for printing.
func RedactSources(srcs ...*Source) []*Source {
	a := make([]*Source, len(srcs))
	for i := range a {
		if srcs[i] == nil {
			continue
		}

		a[i] = srcs[i].Clone()
		a[i].Location = a[i].RedactedLocation()
	}

	return a
}

// RedactLocation returns a redacted version of the source
// location loc, with the password component (if any) of
// the location masked.
func RedactLocation(loc string) string {
	switch {
	case loc == "",
		strings.HasPrefix(loc, "/"),
		strings.HasPrefix(loc, "sqlite3://"):
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

// ShortLocation returns a short location string. For example, the
// base name (data.xlsx) for a file or for a DSN, user@host[:port]/db.
func (s *Source) ShortLocation() string {
	if s == nil {
		return ""
	}
	return ShortLocation(s.Location)
}

// Redefine the DriverType values here rather than introducing
// a circular dependency on the driver impl packages.
const (
	typeSL3  = DriverType("sqlite3")
	typePg   = DriverType("postgres")
	typeMS   = DriverType("sqlserver")
	typeMy   = DriverType("mysql")
	typeXLSX = DriverType("xlsx")
	typeCSV  = DriverType("csv")
	typeTSV  = DriverType("tsv")
)

// typeFromMediaType returns the driver type corresponding to mediatype.
// For example:
//
//	xlsx		application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
//	csv			text/csv
//
// Note that we don't rely on this function for types such
// as application/json, because JSON can map to multiple
// driver types (json, jsona, jsonl).
func typeFromMediaType(mediatype string) (typ DriverType, ok bool) {
	switch {
	case strings.Contains(mediatype, `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`):
		return typeXLSX, true
	case strings.Contains(mediatype, `text/csv`):
		return typeCSV, true
	case strings.Contains(mediatype, `text/tab-separated-values`):
		return typeTSV, true
	}

	return TypeNone, false
}

// Target returns @handle.tbl. This is often used in log messages.
func Target(src *Source, tbl string) string {
	if src == nil {
		return ""
	}

	return src.Handle + "." + tbl
}

// validSource performs basic checking on source s.
func validSource(s *Source) error {
	if s == nil {
		return errz.New("source is nil")
	}

	err := ValidHandle(s.Handle)
	if err != nil {
		return err
	}

	if strings.TrimSpace(s.Location) == "" {
		return errz.Errorf("%s: location is empty", s.Handle)
	}

	if s.Type == TypeNone {
		return errz.Errorf("%s: driver type is empty or unknown: %s", s.Handle, s.Type)
	}

	return nil
}
