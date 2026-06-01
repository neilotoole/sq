package secret

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// Expand walks ${scheme:path} placeholders in template, resolves each via
// the registered Resolver, and returns the substituted string.
//
// Substitution is non-recursive: resolved values are never re-scanned.
// $$ in template is unescaped to a single $.
//
// When template parses as a URL, resolved values that land inside the URL
// userinfo are URL-encoded so characters like '@', ':', '/', '?' and '#'
// don't break URL parsing in downstream consumers. Resolved values in
// other positions (host, path, opaque whole-DSN) are spliced raw.
//
// On a resolver error or unknown scheme, Expand returns the underlying
// error wrapped with context.
func (r *Registry) Expand(ctx context.Context, template string) (string, error) {
	placeholders, err := findPlaceholders(template)
	if err != nil {
		return "", err
	}
	if len(placeholders) == 0 {
		return unescapeDollar(template), nil
	}

	resolved := make([]string, len(placeholders))
	for i, p := range placeholders {
		v, err := r.ResolveScheme(ctx, p.scheme, p.path)
		if err != nil {
			// For the sq-owned keyring scheme, the fix is always to
			// (re-)set the value at that path. Surface the exact
			// command so the user doesn't have to guess.
			if errors.Is(err, ErrNotFound) && p.scheme == "keyring" {
				return "", errz.Wrapf(err,
					"resolve ${%s:%s} (run: sq config keyring create %s)",
					p.scheme, p.path, p.path)
			}
			return "", errz.Wrapf(err, "resolve ${%s:%s}", p.scheme, p.path)
		}
		resolved[i] = v
	}

	userinfoIdx := userinfoPlaceholders(template, placeholders)
	return spliceWithEncoding(template, placeholders, resolved, userinfoIdx), nil
}

// userinfoSentinelFmt is the fmt template for the digit-only sentinel
// substituted in for each placeholder during userinfo detection. It
// must be all digits so it parses correctly even when a placeholder
// occupies the URL port position (e.g. "postgres://host:${env:PORT}/db"
// — Go's url.Parse rejects non-digit ports, which would otherwise
// short-circuit detection and leak userinfo placeholders past the
// URL-encoding step). The fixed leading/trailing "9999000" /
// "9999" plus a 7-digit index yields an 18-digit string that is
// extremely unlikely to collide with real URL data.
const userinfoSentinelFmt = "9999000%07d9999"

// userinfoPlaceholders returns the set of indices (into placeholders)
// whose substitution position lies inside the userinfo component of
// template. Detected by replacing each placeholder with a digit-only
// sentinel and parsing the resulting string as a URL. Returns an empty
// map when template does not parse as a URL or has no userinfo.
func userinfoPlaceholders(template string, placeholders []placeholder) map[int]bool {
	out := make(map[int]bool)
	if len(placeholders) == 0 {
		return out
	}

	var b strings.Builder
	pos := 0
	for i, p := range placeholders {
		b.WriteString(template[pos:p.start])
		fmt.Fprintf(&b, userinfoSentinelFmt, i)
		pos = p.end
	}
	b.WriteString(template[pos:])
	sentinelled := b.String()

	u, err := url.Parse(sentinelled)
	if err != nil || u.User == nil {
		return out
	}
	user := u.User.Username()
	pass, _ := u.User.Password()
	for i := range placeholders {
		ref := fmt.Sprintf(userinfoSentinelFmt, i)
		if strings.Contains(user, ref) || strings.Contains(pass, ref) {
			out[i] = true
		}
	}
	return out
}

// spliceWithEncoding rebuilds template, URL-encoding placeholders flagged
// in userinfoIdx and splicing others raw.
func spliceWithEncoding(template string, placeholders []placeholder,
	resolved []string, userinfoIdx map[int]bool,
) string {
	var b strings.Builder
	b.Grow(len(template))
	pos := 0
	for i, p := range placeholders {
		b.WriteString(unescapeDollar(template[pos:p.start]))
		if userinfoIdx[i] {
			b.WriteString(urlUserinfoEncode(resolved[i]))
		} else {
			b.WriteString(resolved[i])
		}
		pos = p.end
	}
	b.WriteString(unescapeDollar(template[pos:]))
	return b.String()
}

// urlUserinfoEncode percent-encodes s for use inside URL userinfo. It
// routes via url.UserPassword which performs the correct encoding for
// the password component, then trims the wrapping that url.URL.String
// adds.
func urlUserinfoEncode(s string) string {
	u := &url.URL{User: url.UserPassword("", s)}
	out := u.String()
	out = strings.TrimPrefix(out, "//:")
	out = strings.TrimSuffix(out, "@")
	return out
}

// unescapeDollar replaces every $$ with $. It is only called on segments
// known to contain no placeholders.
func unescapeDollar(s string) string {
	if !strings.Contains(s, "$$") {
		return s
	}
	return strings.ReplaceAll(s, "$$", "$")
}
