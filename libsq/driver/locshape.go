// LocationShape declares a SQL driver's URL syntax declaratively so
// that shell completion (and future location validation) can walk
// partial input against a shape without per-driver branches in the
// caller.
package driver

import (
	"context"
	"net/url"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// LocationShape declares the URL syntax for a SQL driver.
type LocationShape struct {
	// Type associates this shape with its driver.
	Type drivertype.Type

	// Schemes are the accepted scheme prefixes (without "://").
	// The first entry is canonical. Most drivers have one; rqlite
	// has {"rqlite", "rqlites"}.
	Schemes []string

	// Segments lists the ordered URL segments that follow "scheme://".
	// A driver only includes segments that exist in its URL form.
	Segments []Segment
}

// SegmentFor returns the segment with the given kind, or the zero
// value if not present. Used by the completer to look up the segment
// the walker says we are currently in.
func (s LocationShape) SegmentFor(kind SegmentKind) Segment {
	for _, seg := range s.Segments {
		if seg.Kind == kind {
			return seg
		}
	}
	return Segment{}
}

// SegmentKind enumerates the segment vocabulary. The minimal set that
// covers all current SQL drivers plus the planned file:// pseudo-
// driver (#751).
type SegmentKind int

const (
	// SegCredentials models "user[:pass]@".
	SegCredentials SegmentKind = iota + 1

	// SegAuthority models "host[:port]".
	SegAuthority

	// SegPathName models "/identifier" where identifier is a single
	// name (db name, instance name, service name).
	SegPathName

	// SegPathFile models "/path/to/file" for file-based drivers
	// (sqlite, duckdb).
	SegPathFile

	// SegConnParams models "?key=val&...".
	SegConnParams
)

// Segment configures one position in a LocationShape.
type Segment struct {
	// Suggest is an optional escape hatch that overrides the
	// completer's default candidate generation for this segment.
	// nil means "use the completer's default for this Kind".
	Suggest SuggestFunc

	// Placeholder is the noun shown as a completion hint for
	// SegPathName (e.g. "db", "instance", "service"). Ignored for
	// other kinds.
	Placeholder string

	// LeadingKey, on SegConnParams, names a key that should be
	// suggested first. Used by SQL Server's "?database=".
	LeadingKey string

	// Kind is the segment's kind.
	Kind SegmentKind

	// Optional means the user may skip this segment. The walker
	// advances past optional segments when the introducer delimiter
	// is not present.
	Optional bool
}

// MatchedLoc describes the result of Walk: which segments were
// matched, which segment the user is currently typing (if any), and
// the parsed fields of each.
type MatchedLoc struct {
	// Params holds the parsed SegConnParams values.
	Params url.Values

	// Loc is the original input verbatim.
	Loc string

	// Scheme is the matched scheme (without "://"), e.g. "rqlite"
	// or "rqlites".
	Scheme string

	// SegCredentials fields.
	User string
	Pass string

	// SegAuthority field.
	Hostname string

	// SegPathName field.
	PathName string

	// SegPathFile field.
	PathFile string

	// ParamLastKey is the rightmost (currently-typed) key, or "".
	ParamLastKey string

	// Done lists segment kinds whose terminator was matched, in order.
	Done []SegmentKind

	// Current is the segment kind the user is currently inside.
	// Zero if the cursor sits at a segment boundary with no
	// committed content.
	Current SegmentKind

	// Port is the parsed SegAuthority port.
	Port int

	// HasCreds is true if '@' was seen (credentials existed, even if empty).
	HasCreds bool

	// PassSet distinguishes "alice" from "alice:".
	PassSet bool

	// PortSet distinguishes "host" from "host:".
	PortSet bool

	// ParamAtValue is true if the cursor sits after "=" of the last element.
	ParamAtValue bool
}

// Suggestions abstracts the source of "values the user has used
// before" for completion. The default cli/ impl is backed by
// source.Collection; future impls may layer env vars, MRU lists, or
// keyring entries.
type Suggestions interface {
	// Values returns prior single-element values for the given kind:
	// usernames for SegCredentials, "host[:port]" forms for
	// SegAuthority, db/instance/service names for SegPathName, file
	// paths for SegPathFile. Returns nil for kinds with no natural
	// single-value form.
	Values(kind SegmentKind) []string

	// Tails returns prior URL tails starting from the given kind,
	// e.g. Tails(SegAuthority) yields
	// "db.example.com:5432/mydb?sslmode=require" forms.
	Tails(kind SegmentKind) []string

	// Locations returns the full prior locations verbatim.
	Locations() []string
}

// SuggestFunc returns candidate completion strings for one segment
// given the already-matched location prefix and the available
// suggestion sources.
type SuggestFunc func(ctx context.Context, m MatchedLoc, src Suggestions) []string

// Walk matches loc against shape and returns a MatchedLoc describing
// which segments were detected, which segment is currently being
// typed (if any), and the parsed fields of each. loc must begin with
// one of shape.Schemes followed by "://"; the top-level dispatcher
// handles the no-scheme case before calling Walk.
func Walk(shape LocationShape, loc string) (MatchedLoc, error) {
	m := MatchedLoc{Loc: loc}
	for _, scheme := range shape.Schemes {
		prefix := scheme + "://"
		if strings.HasPrefix(loc, prefix) {
			m.Scheme = scheme
			tail := loc[len(prefix):]
			return walkSegments(shape, m, tail)
		}
	}
	return m, errz.Errorf("scheme not matched: %q", loc)
}

// walkSegments walks the post-scheme tail against shape.Segments,
// populating m as it goes.
func walkSegments(shape LocationShape, m MatchedLoc, tail string) (MatchedLoc, error) {
	cursor := 0
	for _, seg := range shape.Segments {
		if seg.Kind == SegCredentials {
			matched, advance, current := walkCredentials(tail[cursor:], seg.Optional)
			if matched.User != "" || matched.PassSet || matched.HasCreds {
				m.User = matched.User
				m.Pass = matched.Pass
				m.PassSet = matched.PassSet
				m.HasCreds = matched.HasCreds
			}
			if current {
				m.Current = SegCredentials
				return m, nil
			}
			if matched.HasCreds {
				m.Done = append(m.Done, SegCredentials)
			}
			cursor += advance
		}
	}
	return m, nil
}

// walkCredentials parses an optional user[:pass]@ prefix from s.
// Returns the parsed fields, the number of bytes consumed (including
// the trailing '@' if matched), and current==true if the user is
// still typing inside the credentials segment.
func walkCredentials(s string, optional bool) (matched MatchedLoc, advance int, current bool) {
	atIdx := strings.IndexByte(s, '@')
	if atIdx == -1 {
		// No '@' present.
		if optional && strings.ContainsAny(s, "/?") {
			// Skip-signal: user has moved past credentials.
			return MatchedLoc{}, 0, false
		}
		// Partial credentials being typed.
		user, pass, hasColon := strings.Cut(s, ":")
		matched.User = user
		matched.PassSet = hasColon
		if hasColon {
			matched.Pass = pass
		}
		return matched, 0, true
	}
	creds := s[:atIdx]
	user, pass, hasColon := strings.Cut(creds, ":")
	matched.User = user
	matched.HasCreds = true
	matched.PassSet = hasColon
	if hasColon {
		matched.Pass = pass
	}
	return matched, atIdx + 1, false
}
