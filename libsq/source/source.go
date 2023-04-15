// Package source provides functionality for dealing with data sources.
package source

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"golang.org/x/exp/slog"

	"github.com/xo/dburl"

	"github.com/neilotoole/sq/libsq/core/options"
)

// Type is a source type, e.g. "mysql", "postgres", "csv", etc.
type Type string

// String returns a log/debug-friendly representation.
func (t Type) String() string {
	return string(t)
}

// TypeNone is the zero value of driver.Type.
const TypeNone = Type("")

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
	Type Type `yaml:"type" json:"type"`

	// Location is the source location, such as a DB connection URI,
	// or a file path.
	Location string `yaml:"location" json:"location"`

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
	)
}

// String returns a log/debug-friendly representation.
func (s *Source) String() string {
	return fmt.Sprintf("%s|%s| %s", s.Handle, s.Type, s.RedactedLocation())
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
		Options:  s.Options.Clone(),
	}
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

const (
	typeSL3  = Type("sqlite3")
	typePg   = Type("postgres")
	typeMS   = Type("sqlserver")
	typeMy   = Type("mysql")
	typeXLSX = Type("xlsx")
	typeCSV  = Type("csv")
	typeTSV  = Type("tsv")
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
func typeFromMediaType(mediatype string) (typ Type, ok bool) {
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
		return errz.New("source location is empty")
	}

	if s.Type == TypeNone {
		return errz.Errorf("source type is empty or unknown: {%s}", s.Type)
	}

	return nil
}
