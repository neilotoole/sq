package cli

import (
	"context"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/samber/lo"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/driver"
)

// nextSegmentIntroducer returns the introducer character that the user
// should type next after completing the segment of the given kind.
// Looks up the segment AFTER the given kind in shape and returns its
// introducer ("/" for SegPathName/SegPathFile, "?" for SegConnParams).
// Returns "" if no following segment exists.
func nextSegmentIntroducer(shape driver.LocationShape, after driver.SegmentKind) string {
	found := false
	for _, seg := range shape.Segments {
		if found {
			switch seg.Kind {
			case driver.SegPathName, driver.SegPathFile:
				return "/"
			case driver.SegConnParams:
				return "?"
			case driver.SegCredentials, driver.SegAuthority:
				return ""
			default:
				return ""
			}
		}
		if seg.Kind == after {
			found = true
		}
	}
	return ""
}

// suggestCreds generates candidates when MatchedLoc.Current is
// SegCredentials. Offers username placeholders, history usernames,
// and both "@" and ":" continuations for ambiguous early input.
func suggestCreds(m driver.MatchedLoc, src driver.Suggestions) []string {
	cs := candidateSet{prefix: m.Loc}
	base := m.Scheme + "://"
	unames := src.Values(driver.SegCredentials)

	if m.User == "" {
		// Empty: offer history usernames and the "username" placeholder.
		cs.addPrefixed(base, unames...)
		cs.add(base + "username")
		return cs.build()
	}

	// Partial username: offer "@" and ":" to push past credentials,
	// plus history usernames as continuations.
	cs.add(m.Loc+"@", m.Loc+":")
	for _, u := range unames {
		v := base + u
		cs.add(v+"@", v+":")
	}
	return cs.build()
}

// suggestAuthority generates candidates when MatchedLoc.Current is
// SegAuthority. Offers "localhost", default port, and history hosts.
func suggestAuthority(m driver.MatchedLoc, src driver.Suggestions,
	defaultPort int, shape driver.LocationShape,
) []string {
	cs := candidateSet{prefix: m.Loc}
	const localhost = "localhost"
	afterHost := nextSegmentIntroducer(shape, driver.SegAuthority)
	if afterHost == "" {
		afterHost = "/" // sensible fallback.
	}

	// Determine the base prefix the authority sits on top of.
	base, _, hasAt := strings.Cut(m.Loc, "@")
	if hasAt {
		base += "@"
	} else {
		base = m.Scheme + "://"
	}

	hosts := src.Values(driver.SegAuthority)
	tails := src.Tails(driver.SegAuthority)

	if m.Hostname == "" {
		// Empty host: offer localhost + history hosts/tails.
		cs.add(base + localhost + afterHost)
		if defaultPort > 0 {
			cs.add(base + localhost + ":" + strconv.Itoa(defaultPort) + afterHost)
		}
		cs.addPrefixed(base, tails...)
		for _, h := range hosts {
			cs.add(base + h + afterHost)
		}
		return cs.build()
	}

	if !m.PortSet {
		// Hostname but no port: offer the default port, the
		// "afterHost" continuation, and history.
		cs.add(m.Loc + afterHost)
		if defaultPort > 0 {
			cs.add(m.Loc + ":" + strconv.Itoa(defaultPort) + afterHost)
		}
		cs.add(base + localhost + afterHost)
		if defaultPort > 0 {
			cs.add(base + localhost + ":" + strconv.Itoa(defaultPort) + afterHost)
		}
		cs.addPrefixed(base, tails...)
		for _, h := range hosts {
			cs.add(base + h + afterHost)
		}
		return cs.build()
	}

	// Hostname + port: offer afterHost.
	cs.add(m.Loc + afterHost)
	if defaultPort > 0 {
		cs.add(base + localhost + ":" + strconv.Itoa(defaultPort) + afterHost)
	}
	cs.add(base + localhost + afterHost)
	cs.addPrefixed(base, tails...)
	for _, h := range hosts {
		cs.add(base + h + afterHost)
	}
	return cs.build()
}

// suggestPathName generates candidates when MatchedLoc.Current is
// SegPathName. Offers the placeholder name and history db names.
func suggestPathName(m driver.MatchedLoc, src driver.Suggestions, placeholder string) []string {
	cs := candidateSet{prefix: m.Loc}
	names := src.Values(driver.SegPathName)
	if m.PathName == "" {
		cs.add(m.Loc + placeholder)
		for _, n := range names {
			cs.add(m.Loc + n)
		}
		return cs.build()
	}
	// Partial name: offer "?" to move on, plus history dbnames against
	// the base up to and including "/".
	cs.add(m.Loc + "?")
	if idx := strings.LastIndex(m.Loc, "/"); idx >= 0 {
		base := m.Loc[:idx+1]
		for _, n := range names {
			cs.add(base + n)
		}
	}
	return cs.build()
}

// suggestPathFile generates candidates when MatchedLoc.Current is
// SegPathFile. Offers filesystem listings and "?" once a file is
// fully matched.
func suggestPathFile(ctx context.Context, m driver.MatchedLoc, src driver.Suggestions) []string {
	cs := candidateSet{prefix: m.Loc}
	base := m.Scheme + "://"
	typed := m.PathFile

	paths := locCompListFiles(ctx, typed)
	for i := range paths {
		if ioz.IsPathToRegularFile(paths[i]) && paths[i] == typed {
			paths[i] += "?"
		}
		cs.add(base + paths[i])
	}

	// Also offer full prior locations (sqlite/duckdb-style).
	cs.add(src.Locations()...)
	return cs.build()
}

// suggestConnParams generates candidates when MatchedLoc.Current is
// SegConnParams. Honors leadingKey by suggesting that key first.
func suggestConnParams(m driver.MatchedLoc, src driver.Suggestions,
	drvr driver.SQLDriver, leadingKey string,
) []string {
	_ = src // Tails could feed history-driven param strings in the future.
	cs := candidateSet{prefix: m.Loc}
	keys, values := connParamKeysAndValues(drvr, leadingKey)

	// Locate the "stump" (everything up to and incl. last '&' or '?').
	stump := m.Loc
	if idx := strings.LastIndexAny(stump, "&?"); idx >= 0 {
		stump = stump[:idx+1]
	}

	if !m.ParamAtValue {
		// We are typing a key.
		current := m.ParamLastKey
		for _, k := range keys {
			if !strings.HasPrefix(k, current) {
				continue
			}
			// Dedup: skip if this key already has a non-empty value.
			if existing, ok := m.Params[k]; ok {
				skip := false
				for _, v := range existing {
					if v != "" {
						skip = true
						break
					}
				}
				if skip {
					continue
				}
			}
			cs.add(stump + k + "=")
		}
		return cs.build()
	}

	// We are typing a value for ParamLastKey.
	vs := values[m.ParamLastKey]
	for _, v := range vs {
		cs.add(stump + m.ParamLastKey + "=" + v)
	}
	if len(vs) == 0 {
		// Unknown values: offer "&" to move to next param.
		last := m.Loc[len(m.Loc)-1]
		if last != '&' && last != '?' && last != '=' {
			cs.add(m.Loc + "&")
		}
	}
	out := cs.build()
	if len(out) == 0 {
		// No matches: push to "&" for next param.
		return []string{m.Loc + "&"}
	}
	return out
}

// connParamKeysAndValues returns the driver's ConnParams keys (with
// leadingKey hoisted if set) and a key->[]value map. Keys are
// URL-query-escaped so shell completion can safely emit them (some
// drivers, e.g. sqlserver, declare keys containing spaces such as
// "Workstation ID"). The values map is keyed by the escaped form so
// later lookups by m.ParamLastKey (also escaped, since it came from
// the typed URL) hit correctly.
func connParamKeysAndValues(drvr driver.SQLDriver, leadingKey string) (
	keys []string, values map[string][]string,
) {
	og := drvr.ConnParams()
	ogKeys := lo.Keys(og)
	slices.Sort(ogKeys)

	if leadingKey != "" {
		ogKeys = lo.Without(ogKeys, leadingKey)
		ogKeys = append([]string{leadingKey}, ogKeys...)
	}

	keys = make([]string, len(ogKeys))
	values = make(map[string][]string, len(og))
	for i, k := range ogKeys {
		escaped := url.QueryEscape(k)
		keys[i] = escaped
		values[escaped] = og[k]
	}
	return keys, values
}

// generateCandidates dispatches to the per-segment-kind helper
// indicated by m.Current. Honors any custom Segment.Suggest hook
// before falling back to defaults.
func generateCandidates(ctx context.Context, shape driver.LocationShape,
	m driver.MatchedLoc, src driver.Suggestions, drvr driver.SQLDriver,
) []string {
	seg := shape.SegmentFor(m.Current)

	if seg.Suggest != nil {
		return seg.Suggest(ctx, m, src)
	}

	switch m.Current {
	case driver.SegCredentials:
		return suggestCreds(m, src)
	case driver.SegAuthority:
		return suggestAuthority(m, src, drvr.DriverMetadata().DefaultPort, shape)
	case driver.SegPathName:
		return suggestPathName(m, src, seg.Placeholder)
	case driver.SegPathFile:
		return suggestPathFile(ctx, m, src)
	case driver.SegConnParams:
		return suggestConnParams(m, src, drvr, seg.LeadingKey)
	}
	return nil
}
