package secret

import (
	"context"
	"fmt"
	"strings"
)

// Expand walks ${scheme:path} placeholders in template, resolves each via
// the registered Resolver, and returns the substituted string.
//
// Substitution is non-recursive: resolved values are never re-scanned.
// $$ in template is unescaped to a single $.
//
// On a resolver error or unknown scheme, Expand returns the underlying
// error wrapped with context.
//
// NOTE: URL-encoding of values that land inside URL userinfo is added
// in a later task; this implementation splices raw.
func (r *Registry) Expand(ctx context.Context, template string) (string, error) {
	placeholders, err := findPlaceholders(template)
	if err != nil {
		return "", err
	}
	if len(placeholders) == 0 {
		return unescapeDollar(template), nil
	}

	// Resolve all placeholders up front so we report errors before
	// touching the template.
	resolved := make([]string, len(placeholders))
	for i, p := range placeholders {
		v, err := r.ResolveScheme(ctx, p.scheme, p.path)
		if err != nil {
			return "", fmt.Errorf("resolve ${%s:%s}: %w", p.scheme, p.path, err)
		}
		resolved[i] = v
	}

	return spliceRaw(template, placeholders, resolved), nil
}

// spliceRaw rebuilds template with each placeholder replaced by its
// resolved value, applying $$ unescaping to the literal segments between
// placeholders.
func spliceRaw(template string, placeholders []placeholder, resolved []string) string {
	var b strings.Builder
	b.Grow(len(template))
	pos := 0
	for i, p := range placeholders {
		b.WriteString(unescapeDollar(template[pos:p.start]))
		b.WriteString(resolved[i])
		pos = p.end
	}
	b.WriteString(unescapeDollar(template[pos:]))
	return b.String()
}

// unescapeDollar replaces every $$ with $. It is only called on segments
// known to contain no placeholders.
func unescapeDollar(s string) string {
	if !strings.Contains(s, "$$") {
		return s
	}
	return strings.ReplaceAll(s, "$$", "$")
}
