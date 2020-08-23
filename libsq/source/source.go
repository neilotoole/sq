// Package source provides functionality for dealing with data sources.
package source

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/xo/dburl"

	"github.com/neilotoole/sq/libsq/core/options"
)

// Type is a source type, e.g. "mysql", "postgres", "csv", etc.
type Type string

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
	return []string{StdinHandle, ActiveHandle, ScratchHandle, JoinHandle}
}

// Source describes a data source.
type Source struct {
	Handle   string          `yaml:"handle" json:"handle"`
	Type     Type            `yaml:"type" json:"type"`
	Location string          `yaml:"location" json:"location"`
	Options  options.Options `yaml:"options,omitempty" json:"options,omitempty"`
}

func (s *Source) String() string {
	return fmt.Sprintf("%s | %s | %s", s.Handle, s.Type, s.RedactedLocation())
}

// RedactedLocation returns s.Location, with the password component
// of the location masked.
func (s *Source) RedactedLocation() string {
	if s == nil {
		return ""
	}
	loc := s.Location

	switch {
	case loc == "":
		return ""
	case strings.HasPrefix(loc, "/"):
		// It's a file
		return loc
	case strings.HasPrefix(loc, "http://"), strings.HasPrefix(loc, "https://"):
		// TODO: technically a HTTP url could have a user:password component that could be masked
		return loc
	}

	// At this point, we expect it's a DSN
	u, err := dburl.Parse(loc)
	if err != nil {
		// Shouldn't happen, but if it does, simply return the
		// unmodified loc.
		return loc
	}

	// We want to mask the password, but our preferred ****
	// text gets URL encoded, so we'll make this a two-step process.
	u.User = url.UserPassword(u.User.Username(), "password")
	return strings.Replace(u.String(), "password", "****", 1)
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
//  xlsx		application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
//  csv			text/csv
func typeFromMediaType(mediatype string) (driverType Type, ok bool) {
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
