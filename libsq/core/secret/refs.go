package secret

// Ref is an externally-visible parsed ${scheme:path} reference.
type Ref struct {
	Scheme string
	Path   string
}

// ExtractRefs returns every ${scheme:path} reference found in s, in order.
// Returns an error if any placeholder is malformed.
func ExtractRefs(s string) ([]Ref, error) {
	placeholders, err := findPlaceholders(s)
	if err != nil {
		return nil, err
	}
	if len(placeholders) == 0 {
		return nil, nil
	}
	out := make([]Ref, len(placeholders))
	for i, p := range placeholders {
		out[i] = Ref{Scheme: p.scheme, Path: p.path}
	}
	return out, nil
}
