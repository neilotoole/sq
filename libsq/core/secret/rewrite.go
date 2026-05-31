package secret

import (
	"context"
	"fmt"
	"strings"
)

// RewritePlaceholders walks every ${scheme:path} occurrence in
// template and rebuilds the string with each placeholder's path
// replaced by the value returned from fn. Scheme is passed through
// unchanged; only the path portion is rewritten. Surrounding literal
// text (including escaped $$) is preserved verbatim.
//
// fn is called once per placeholder in the order they appear. If fn
// returns an error, RewritePlaceholders stops and returns it.
// Returning a path that itself contains placeholder syntax is
// undefined — callers must produce inert literal paths.
//
// Use this to transform placeholder bodies that the user typed
// (e.g. expand a relative file path to absolute at sq-add time)
// without resolving the placeholder against the secret store.
// Resolution is Expand's job; rewriting is this function's job.
func RewritePlaceholders(
	ctx context.Context, template string,
	fn func(ctx context.Context, scheme, path string) (string, error),
) (string, error) {
	placeholders, err := findPlaceholders(template)
	if err != nil {
		return "", err
	}
	if len(placeholders) == 0 {
		return template, nil
	}
	var b strings.Builder
	b.Grow(len(template))
	pos := 0
	for _, p := range placeholders {
		b.WriteString(template[pos:p.start])
		newPath, err := fn(ctx, p.scheme, p.path)
		if err != nil {
			return "", fmt.Errorf("rewrite ${%s:%s}: %w", p.scheme, p.path, err)
		}
		fmt.Fprintf(&b, "${%s:%s}", p.scheme, newPath)
		pos = p.end
	}
	b.WriteString(template[pos:])
	return b.String(), nil
}
