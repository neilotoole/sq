package driver

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// LocationShape declares a SQL driver's URL syntax declaratively, so
// that shell completion (and future location validation) can walk
// partial input against the shape without per-driver branches in the
// caller.
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

// Validate checks the LocationShape for structural problems that a
// driver hand-rolled literal could realistically introduce: missing
// type, no schemes, no segments, an unset segment kind, duplicate
// segment kinds, or a LeadingKey/Placeholder set on a segment whose
// kind ignores it. Callers (typically a driver-package test) should
// invoke this once at startup, not on every completion.
func (s LocationShape) Validate() error {
	if s.Type == "" {
		return errz.New("LocationShape: Type is empty")
	}
	if len(s.Schemes) == 0 {
		return errz.New("LocationShape: Schemes is empty")
	}
	for i, sc := range s.Schemes {
		if sc == "" {
			return errz.Errorf("LocationShape: Schemes[%d] is empty", i)
		}
	}
	if len(s.Segments) == 0 {
		return errz.New("LocationShape: Segments is empty")
	}
	seen := make(map[SegmentKind]bool, len(s.Segments))
	for i, seg := range s.Segments {
		if seg.Kind == 0 {
			return errz.Errorf("LocationShape: Segments[%d].Kind is unset", i)
		}
		if seen[seg.Kind] {
			return errz.Errorf("LocationShape: Segments[%d] duplicates kind %v",
				i, seg.Kind)
		}
		seen[seg.Kind] = true
		if seg.LeadingKey != "" && seg.Kind != SegConnParams {
			return errz.Errorf("LocationShape: Segments[%d] has LeadingKey on kind %v "+
				"(only SegConnParams uses LeadingKey)", i, seg.Kind)
		}
		if seg.Placeholder != "" && seg.Kind != SegPathName {
			return errz.Errorf("LocationShape: Segments[%d] has Placeholder on kind %v "+
				"(only SegPathName uses Placeholder)", i, seg.Kind)
		}
	}
	return nil
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
	return m, errz.New("scheme not matched")
}

// walkSegments walks the post-scheme tail against shape.Segments,
// populating m as it goes.
func walkSegments(shape LocationShape, m MatchedLoc, tail string) (MatchedLoc, error) {
	cursor := 0
	for _, seg := range shape.Segments {
		switch seg.Kind {
		case SegCredentials:
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
		case SegAuthority:
			authEnd := strings.IndexAny(tail[cursor:], "/?")
			if authEnd == -1 {
				// Whole remainder is (partial) authority.
				parseAuthority(tail[cursor:], &m)
				m.Current = SegAuthority
				return m, nil
			}
			parseAuthority(tail[cursor:cursor+authEnd], &m)
			m.Done = append(m.Done, SegAuthority)
			cursor += authEnd
			// NOTE: cursor stops AT '/' or '?'; the delimiter
			// belongs to the next segment.
		case SegPathName:
			if cursor >= len(tail) || tail[cursor] != '/' {
				// No '/' to introduce path.
				if seg.Optional {
					continue
				}
				return m, nil
			}
			cursor++ // consume '/'
			pathEnd := strings.IndexByte(tail[cursor:], '?')
			if pathEnd == -1 {
				m.PathName = tail[cursor:]
				m.Current = SegPathName
				return m, nil
			}
			m.PathName = tail[cursor : cursor+pathEnd]
			m.Done = append(m.Done, SegPathName)
			cursor += pathEnd
		case SegPathFile:
			// PathFile has no introducer; it starts at the cursor
			// right after "scheme://". Terminator is '?'.
			pathEnd := strings.IndexByte(tail[cursor:], '?')
			if pathEnd == -1 {
				m.PathFile = tail[cursor:]
				m.Current = SegPathFile
				return m, nil
			}
			m.PathFile = tail[cursor : cursor+pathEnd]
			m.Done = append(m.Done, SegPathFile)
			cursor += pathEnd
		case SegConnParams:
			if cursor >= len(tail) || tail[cursor] != '?' {
				if seg.Optional {
					continue
				}
				return m, nil
			}
			cursor++ // consume '?'
			paramText := tail[cursor:]
			parseConnParams(paramText, &m)
			m.Current = SegConnParams
			return m, nil
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

// parseAuthority parses "host[:port]" into m.Hostname, m.Port,
// m.PortSet. Uses net/url for IPv6-bracket and port handling.
func parseAuthority(authStr string, m *MatchedLoc) {
	// net/url needs a scheme to parse an authority. Wrap with a
	// dummy scheme.
	u, err := url.Parse("dummy://" + authStr)
	if err != nil {
		// Best-effort fallback: take the whole string as hostname.
		m.Hostname = authStr
		return
	}
	m.Hostname = u.Hostname()
	if port := u.Port(); port != "" {
		m.PortSet = true
		if p, err := strconv.Atoi(port); err == nil {
			m.Port = p
		} else {
			m.Port = -1
		}
	} else if strings.HasSuffix(u.Host, ":") {
		// "host:" with empty port.
		m.PortSet = true
	}
}

// parseConnParams parses "key=val&key=val..." and populates m.Params,
// m.ParamLastKey, m.ParamAtValue.
func parseConnParams(s string, m *MatchedLoc) {
	if s == "" {
		m.Params = url.Values{}
		return
	}

	// Split on '&', parse all but the last as complete key=val pairs;
	// the last is the "currently typing" element.
	elements := strings.Split(s, "&")
	last := elements[len(elements)-1]
	completed := elements[:len(elements)-1]

	m.Params = url.Values{}
	for _, el := range completed {
		k, v, _ := strings.Cut(el, "=")
		m.Params.Add(k, v)
	}

	key, val, hasEq := strings.Cut(last, "=")
	m.ParamLastKey = key
	if hasEq {
		m.ParamAtValue = true
		// The value-in-progress is also recorded into Params so that
		// the completer can see existing values (incl. partial) when
		// dedup-suggesting keys.
		m.Params.Add(key, val)
	}
}
