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

// nextSegmentAfter returns information about the segment that follows
// `after` in shape. introducer is the delimiter that introduces the
// next segment ("/" or "?"). optional reports whether that next
// segment may be skipped. hasNext is false if no segment follows.
func nextSegmentAfter(shape driver.LocationShape, after driver.SegmentKind) (
	introducer string, optional, hasNext bool,
) {
	found := false
	for _, seg := range shape.Segments {
		if found {
			switch seg.Kind {
			case driver.SegPathName, driver.SegPathFile:
				return "/", seg.Optional, true
			case driver.SegConnParams:
				return "?", seg.Optional, true
			case driver.SegCredentials, driver.SegAuthority:
				return "", false, false
			}
			return "", false, false
		}
		if seg.Kind == after {
			found = true
		}
	}
	return "", false, false
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

	if m.PassSet {
		// Password phase: "@" closes credentials. If no characters
		// typed yet, also offer a "password" placeholder so the next
		// TAB jumps to the host. Don't offer ":" (would insert a
		// literal colon at the start of the password) and don't
		// replay history usernames here.
		cs.add(m.Loc + "@")
		if m.Pass == "" {
			cs.add(m.Loc + "password@")
		}
		return cs.build()
	}

	// Username phase: "@" closes creds with no password; ":" starts
	// the password segment; history usernames are alternate
	// continuations.
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
	afterHost, nextOptional, _ := nextSegmentAfter(shape, driver.SegAuthority)
	if afterHost == "" {
		afterHost = "/" // sensible fallback.
	}
	// If the next segment is an optional path, the user may also skip
	// it and go straight to ConnParams. Offer "?" as an alternate
	// continuation in that case.
	offerQuery := nextOptional && afterHost == "/"

	// addContinuation adds the prefix with afterHost, plus (when
	// offerQuery) the prefix with "?" to support skipping the optional
	// next segment.
	addContinuation := func(prefix string) {
		cs.add(prefix + afterHost)
		if offerQuery {
			cs.add(prefix + "?")
		}
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
		addContinuation(base + localhost)
		if defaultPort > 0 {
			addContinuation(base + localhost + ":" + strconv.Itoa(defaultPort))
		}
		cs.addPrefixed(base, tails...)
		for _, h := range hosts {
			addContinuation(base + h)
		}
		return cs.build()
	}

	if !m.PortSet {
		// Hostname but no port: offer the default port, the
		// "afterHost" continuation, and history.
		addContinuation(m.Loc)
		if defaultPort > 0 {
			addContinuation(m.Loc + ":" + strconv.Itoa(defaultPort))
		}
		addContinuation(base + localhost)
		if defaultPort > 0 {
			addContinuation(base + localhost + ":" + strconv.Itoa(defaultPort))
		}
		cs.addPrefixed(base, tails...)
		for _, h := range hosts {
			addContinuation(base + h)
		}
		return cs.build()
	}

	// Hostname + port. If the user typed a trailing colon with no
	// port digits, offer the default port for the typed host;
	// otherwise offer the next-segment continuation directly.
	if strings.HasSuffix(m.Loc, ":") && defaultPort > 0 {
		addContinuation(m.Loc + strconv.Itoa(defaultPort))
	} else {
		addContinuation(m.Loc)
	}
	if defaultPort > 0 {
		addContinuation(base + localhost + ":" + strconv.Itoa(defaultPort))
	}
	addContinuation(base + localhost)
	cs.addPrefixed(base, tails...)
	for _, h := range hosts {
		addContinuation(base + h)
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
// SegPathFile. Offers filesystem listings, "?" once a file is fully
// matched, and (when the segment is Optional) "?" with empty path so
// drivers like duckdb that allow scheme://?key=val for stdin can be
// completed.
func suggestPathFile(ctx context.Context, m driver.MatchedLoc, src driver.Suggestions, optional bool) []string {
	cs := candidateSet{prefix: m.Loc}
	base := m.Scheme + "://"
	typed := m.PathFile

	// Optional + empty: offer the skip-to-conn-params variant.
	if optional && typed == "" {
		cs.add(base + "?")
	}

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

	// Locate the "stump" (everything up to and including the
	// delimiter that introduces the currently-typed key=value pair).
	// The query starts at the FIRST '?'; params inside the query are
	// '&'-separated. Searching for the last '?' in the full URL would
	// mis-handle values that happen to contain '?'.
	stump := m.Loc
	if qIdx := strings.IndexByte(stump, '?'); qIdx >= 0 {
		if ampIdx := strings.LastIndexByte(stump[qIdx+1:], '&'); ampIdx >= 0 {
			stump = stump[:qIdx+1+ampIdx+1]
		} else {
			stump = stump[:qIdx+1]
		}
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
		return suggestPathFile(ctx, m, src, seg.Optional)
	case driver.SegConnParams:
		return suggestConnParams(m, src, drvr, seg.LeadingKey)
	}
	return nil
}
