package cli

import (
	"log/slog"
	"net/url"
	"slices"
	"strings"

	"github.com/samber/lo"
	"github.com/xo/dburl"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/urlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// parseSourceLoc parses a source location string. dburl.Parse is used
// for schemes dburl knows; rqlite/rqlites fall back to net/url since
// dburl does not recognize them. The returned *dburl.URL wraps the
// parsed URL so callers can read User/Host/Path/RawQuery uniformly.
func parseSourceLoc(loc string, typ drivertype.Type) (*dburl.URL, error) {
	if typ == drivertype.Rqlite {
		u, err := url.Parse(loc)
		if err != nil {
			return nil, errz.Wrap(err, "parse rqlite location")
		}
		return &dburl.URL{URL: *u, OriginalScheme: u.Scheme}, nil
	}
	du, err := dburl.Parse(loc)
	if err != nil {
		return nil, errz.Wrap(err, "parse location")
	}
	return du, nil
}

// locSuggestions provides historical and contextual values that the
// completer offers to the user. It implements driver.Suggestions.
// Backed by source.Collection.
type locSuggestions struct {
	coll *source.Collection
	log  *slog.Logger
	typ  drivertype.Type
}

// newLocSuggestions constructs a locSuggestions for the given driver
// type.
func newLocSuggestions(coll *source.Collection, typ drivertype.Type,
	log *slog.Logger,
) *locSuggestions {
	return &locSuggestions{coll: coll, typ: typ, log: log}
}

// Values implements driver.Suggestions.
func (s *locSuggestions) Values(kind driver.SegmentKind) []string {
	switch kind {
	case driver.SegCredentials:
		return s.usernames()
	case driver.SegAuthority:
		return s.hosts()
	case driver.SegPathName:
		return s.pathNames()
	case driver.SegPathFile:
		return s.pathFiles()
	case driver.SegConnParams:
		return nil
	default:
		return nil
	}
}

// Tails implements driver.Suggestions.
func (s *locSuggestions) Tails(kind driver.SegmentKind) []string {
	switch kind {
	case driver.SegAuthority:
		return s.hostsWithPathAndQuery()
	case driver.SegConnParams:
		return s.pathsWithQueries()
	case driver.SegCredentials, driver.SegPathName, driver.SegPathFile:
		return nil
	default:
		return nil
	}
}

// Locations implements driver.Suggestions.
func (s *locSuggestions) Locations() []string {
	var locs []string
	_ = s.coll.Visit(func(src *source.Source) error {
		if src.Type != s.typ {
			return nil
		}
		locs = append(locs, src.Location)
		return nil
	})
	locs = lo.Uniq(locs)
	slices.Sort(locs)
	return locs
}

func (s *locSuggestions) usernames() []string {
	var unames []string
	_ = s.coll.Visit(func(src *source.Source) error {
		if src.Type != s.typ {
			return nil
		}
		du, err := parseSourceLoc(src.Location, s.typ)
		if err != nil {
			s.log.Warn("Parse source location", lga.Err, err)
			return nil
		}
		if du.User != nil {
			if uname := du.User.Username(); uname != "" {
				unames = append(unames, uname)
			}
		}
		return nil
	})
	unames = lo.Uniq(unames)
	slices.Sort(unames)
	return unames
}

func (s *locSuggestions) hosts() []string {
	var hosts []string
	_ = s.coll.Visit(func(src *source.Source) error {
		if src.Type != s.typ {
			return nil
		}
		du, err := parseSourceLoc(src.Location, s.typ)
		if err != nil {
			s.log.Warn("Parse source location", lga.Err, err)
			return nil
		}
		hosts = append(hosts, du.Host)
		return nil
	})
	hosts = lo.Uniq(hosts)
	slices.Sort(hosts)
	return hosts
}

func (s *locSuggestions) pathNames() []string {
	var names []string
	_ = s.coll.Visit(func(src *source.Source) error {
		if src.Type != s.typ {
			return nil
		}
		du, err := parseSourceLoc(src.Location, s.typ)
		if err != nil {
			s.log.Warn("Parse source location", lga.Err, err)
			return nil
		}
		if s.typ == drivertype.MSSQL && du.RawQuery != "" {
			vals, err := url.ParseQuery(du.RawQuery)
			if err == nil {
				if db := vals.Get("database"); db != "" {
					names = append(names, db)
				}
			}
			return nil
		}
		if du.Path != "" {
			names = append(names, strings.TrimPrefix(du.Path, "/"))
		}
		return nil
	})
	names = lo.Uniq(names)
	slices.Sort(names)
	return names
}

func (s *locSuggestions) pathFiles() []string {
	// For file-based drivers (sqlite, duckdb), the "path file"
	// portion of the location is the file path.
	var paths []string
	_ = s.coll.Visit(func(src *source.Source) error {
		if src.Type != s.typ {
			return nil
		}
		du, err := parseSourceLoc(src.Location, s.typ)
		if err != nil {
			s.log.Warn("Parse source location", lga.Err, err)
			return nil
		}
		if du.Path != "" {
			paths = append(paths, du.Path)
		}
		return nil
	})
	paths = lo.Uniq(paths)
	slices.Sort(paths)
	return paths
}

func (s *locSuggestions) hostsWithPathAndQuery() []string {
	var values []string
	_ = s.coll.Visit(func(src *source.Source) error {
		if src.Type != s.typ {
			return nil
		}
		du, err := parseSourceLoc(src.Location, s.typ)
		if err != nil {
			s.log.Warn("Parse source location", lga.Err, err)
			return nil
		}
		v := urlz.StripSchemeAndUser(du.URL)
		if v != "" {
			values = append(values, v)
		}
		return nil
	})
	values = lo.Uniq(values)
	slices.Sort(values)
	return values
}

func (s *locSuggestions) pathsWithQueries() []string {
	var values []string
	_ = s.coll.Visit(func(src *source.Source) error {
		if src.Type != s.typ {
			return nil
		}
		du, err := parseSourceLoc(src.Location, s.typ)
		if err != nil {
			s.log.Warn("Parse source location", lga.Err, err)
			return nil
		}
		v := du.Path
		if du.RawQuery != "" {
			v += "?" + du.RawQuery
		}
		values = append(values, v)
		return nil
	})
	values = lo.Uniq(values)
	slices.Sort(values)
	return values
}

// Compile-time check that locSuggestions implements driver.Suggestions.
var _ driver.Suggestions = (*locSuggestions)(nil)
