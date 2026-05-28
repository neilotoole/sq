package secret

import (
	"errors"
	"fmt"
	"strings"
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
// placeholder. Returns an error if any placeholder is malformed.
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
			return nil, fmt.Errorf("unclosed ${...} at offset %d", i)
		}
		end += i + 2 // absolute index of '}'
		inner := s[i+2 : end]
		colon := strings.IndexByte(inner, ':')
		if colon < 0 {
			return nil, fmt.Errorf("missing ':' separator in ${%s} at offset %d", inner, i)
		}
		scheme := inner[:colon]
		path := inner[colon+1:]
		if err := validateScheme(scheme); err != nil {
			return nil, fmt.Errorf("%w in ${%s} at offset %d", err, inner, i)
		}
		if path == "" {
			return nil, fmt.Errorf("empty path in ${%s} at offset %d", inner, i)
		}
		out = append(out, placeholder{
			start: i, end: end + 1, scheme: scheme, path: path,
		})
		i = end // skip past the placeholder
	}
	return out, nil
}

// validateScheme returns nil if scheme matches [a-z][a-z0-9]*.
func validateScheme(scheme string) error {
	if scheme == "" {
		return errors.New("empty scheme")
	}
	for i, r := range scheme {
		if i == 0 {
			if r < 'a' || r > 'z' {
				return fmt.Errorf("invalid scheme %q", scheme)
			}
			continue
		}
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return fmt.Errorf("invalid scheme %q", scheme)
		}
	}
	return nil
}
