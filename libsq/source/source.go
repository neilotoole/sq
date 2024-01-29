// Package source provides functionality for dealing with data sources.
package source

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
)

const (
	// StdinHandle is the reserved handle for stdin pipe input.
	StdinHandle = "@stdin"

	// ActiveHandle is the reserved handle for the active source.
	//
	// TODO: it should be possible to use "@0" as the active handle, but
	// the SLQ grammar doesn't currently allow it. Possibly change this
	// value to "@0" after modifying the SLQ grammar.
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
type Source struct { //nolint:govet
	// Note: We disable the govet linter because it wants to reorder
	// the fields to save a few bytes. However, for the purposes of
	// outputting Source to config files, we prefer this order.
	// We could probably implement a customer marshaler if struct
	// alignment becomes an issue.

	// Handle is used to refer to a source, e.g. "@sakila".
	Handle string `yaml:"handle" json:"handle"`

	// Type is the driver type, e.g. postgres.Type.
	Type drivertype.Type `yaml:"driver" json:"driver"`

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

	attrs := make([]slog.Attr, 3, 5)
	attrs[0] = slog.String(lga.Handle, s.Handle)
	attrs[1] = slog.String(lga.Driver, string(s.Type))
	attrs[2] = slog.String(lga.Loc, s.RedactedLocation())
	if s.Catalog != "" {
		attrs = append(attrs, slog.String(lga.Catalog, s.Catalog))
	}
	if s.Schema != "" {
		attrs = append(attrs, slog.String(lga.Schema, s.Schema))
	}

	// For really intense debugging, we can log the options.
	// But it's too much for normal logging.
	//
	// if s.Options != nil {
	// 	attrs = append(attrs, slog.Any(lga.Opts, s.Options))
	// }

	return slog.GroupValue(attrs...)
}

// String returns a log/debug-friendly representation.
func (s *Source) String() string {
	return fmt.Sprintf("%s|%s|%s", s.Handle, s.Type, s.RedactedLocation())
}

// ShortLocation returns a short location string. For example, the
// base name (data.xlsx) for a file or for a DSN, user@host[:port]/db.
func (s *Source) ShortLocation() string {
	if s == nil {
		return ""
	}
	return location.Short(s.Location)
}

// Group returns the source's group. If s is in the root group,
// the empty string is returned.
//
// REVISIT: For root group, should "/" be returned instead of empty string?
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
	return location.Redact(s.Location)
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

	if s.Type == drivertype.None {
		return errz.Errorf("%s: driver type is empty or unknown: %s", s.Handle, s.Type)
	}

	return nil
}
