package secret

import (
	"context"
	"fmt"
	"net/url"
	"strings"
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
			return "", fmt.Errorf("resolve ${%s:%s}: %w", p.scheme, p.path, err)
		}
		resolved[i] = v
	}

	userinfoIdx := userinfoPlaceholders(template, placeholders)
	return spliceWithEncoding(template, placeholders, resolved, userinfoIdx), nil
}

// userinfoPlaceholders returns the set of indices (into placeholders)
// whose substitution position lies inside the userinfo component of
// template. Detected by replacing each placeholder with a URL-safe
// sentinel and parsing the resulting string as a URL. Returns an empty
// map when template does not parse as a URL or has no userinfo.
func userinfoPlaceholders(template string, placeholders []placeholder) map[int]bool {
	out := make(map[int]bool)
	if len(placeholders) == 0 {
		return out
	}

	const sentinelPrefix = "__SQ_SECRET_REF_"
	const sentinelSuffix = "__"

	var b strings.Builder
	pos := 0
	for i, p := range placeholders {
		b.WriteString(template[pos:p.start])
		fmt.Fprintf(&b, "%s%d%s", sentinelPrefix, i, sentinelSuffix)
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
		ref := fmt.Sprintf("%s%d%s", sentinelPrefix, i, sentinelSuffix)
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
