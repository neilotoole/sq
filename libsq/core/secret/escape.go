package secret

import "strings"

// Escape returns s with every '$' doubled to "$$", the placeholder
// grammar's escape for a literal dollar. The result always parses
// cleanly with zero refs (every '$' is part of a "$$" pair), and
// expanding it yields exactly s. Use Escape to store a literal string
// in a field that is interpreted as a placeholder template, such as
// source locations.
func Escape(s string) string {
	if !strings.Contains(s, "$") {
		return s
	}
	return strings.ReplaceAll(s, "$", "$$")
}

// Unescape replaces every "$$" with "$". It is the inverse of Escape
// for strings that contain no placeholders; callers must not pass a
// string containing well-formed ${scheme:path} refs, as the ref text
// itself would not be protected from the replacement. Registry.Expand
// handles unescaping for strings with refs.
func Unescape(s string) string {
	return unescapeDollar(s)
}
