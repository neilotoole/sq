package secret

import (
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// placeholder is the parsed location of a ${scheme:path} occurrence within
// a template string. start is the index of the leading '$', end is the
// index just past the closing '}' (slice bounds are template[start:end]).
type placeholder struct {
	scheme     string
	path       string
	start, end int
}

// findPlaceholders returns every ${scheme:path} placeholder in s, in order.
// $$ is treated as an escape for a literal '$' and does not start a
// placeholder. Returns an error if any placeholder is malformed, or if
// the literal text following a placeholder contains an unbalanced '}'
// (see checkUnbalancedClose).
func findPlaceholders(s string) ([]placeholder, error) {
	var out []placeholder
	for i := 0; i < len(s); i++ {
		if s[i] != '$' {
			continue
		}
		if i+1 < len(s) && s[i+1] == '$' {
			// $$ escape — skip both characters.
			i++
			continue
		}
		if i+1 >= len(s) || s[i+1] != '{' {
			continue
		}
		// Find the closing '}'.
		end := strings.IndexByte(s[i+2:], '}')
		if end < 0 {
			return nil, errz.Errorf("unclosed ${...} at offset %d", i)
		}
		end += i + 2 // absolute index of '}'
		inner := s[i+2 : end]
		scheme, path, ok := strings.Cut(inner, ":")
		if !ok {
			return nil, errz.Errorf("missing ':' separator in ${%s} at offset %d", inner, i)
		}
		if err := validateScheme(scheme); err != nil {
			return nil, errz.Wrapf(err, "in ${%s} at offset %d", inner, i)
		}
		if path == "" {
			return nil, errz.Errorf("empty path in ${%s} at offset %d", inner, i)
		}
		out = append(out, placeholder{
			start: i, end: end + 1, scheme: scheme, path: path,
		})
		i = end // skip past the placeholder
	}
	if err := checkUnbalancedClose(s, out); err != nil {
		return nil, err
	}
	return out, nil
}

// checkUnbalancedClose scans the literal text of s (the regions outside
// the placeholders in phs) for a '}' that most likely marks a truncated
// placeholder path, and returns a parse error when one is found. The
// grammar terminates a placeholder path at the first '}', so a path
// containing '}' cannot be expressed: without this check, a template
// like "${file:/run/sec}rets/pw}" silently parses as path "/run/sec"
// with "rets/pw}" spliced into the location as literal text (gh #787).
//
// The rules, chosen so that well-formed existing templates keep parsing
// unchanged:
//
//   - Literal text before the first placeholder is never checked: a '}'
//     there cannot be a truncation artifact.
//   - A run of '}' immediately following a placeholder is literal, so
//     "${env:X}}" stays a placeholder followed by a literal '}'.
//   - Any other '}' in literal text after a placeholder must be
//     balanced by an earlier unmatched literal '{'; an unbalanced '}'
//     is an error naming its byte offset.
func checkUnbalancedClose(s string, phs []placeholder) error {
	if len(phs) == 0 {
		return nil
	}
	var depth int // unmatched literal '{' count
	next := 0     // index into phs of the next placeholder
	last := -1    // index into phs of the most recent placeholder
	for i := 0; i < len(s); i++ {
		if next < len(phs) && i == phs[next].start {
			last = next
			next++
			i = phs[last].end - 1 // loop increment moves past the closing '}'
			// A run of '}' immediately following the placeholder is
			// literal; skip it without affecting depth.
			for i+1 < len(s) && s[i+1] == '}' {
				i++
			}
			continue
		}
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
				continue
			}
			if last >= 0 {
				return errz.Errorf("unbalanced '}' at offset %d after placeholder ${%s:%s}",
					i, phs[last].scheme, phs[last].path)
			}
			// Before the first placeholder: literal; depth stays 0.
		}
	}
	return nil
}

// validateScheme returns nil if scheme matches [a-z][a-z0-9]*.
func validateScheme(scheme string) error {
	if scheme == "" {
		return errz.New("empty scheme")
	}
	for i, r := range scheme {
		if i == 0 {
			if r < 'a' || r > 'z' {
				return errz.Errorf("invalid scheme %q", scheme)
			}
			continue
		}
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return errz.Errorf("invalid scheme %q", scheme)
		}
	}
	return nil
}
