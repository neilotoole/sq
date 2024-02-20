package stringz

import (
	"strings"
)

// DoubleQuote double-quotes (and escapes) s.
//
//	hello "world"  -->  "hello ""world"""
func DoubleQuote(s string) string {
	const q = '"'
	sb := strings.Builder{}
	sb.WriteRune(q)
	for _, r := range s {
		if r == q {
			sb.WriteRune(q)
		}
		sb.WriteRune(r)
	}
	sb.WriteRune(q)
	return sb.String()
}

// StripDoubleQuote strips double quotes from s,
// or returns s unchanged if it is not correctly double-quoted.
func StripDoubleQuote(s string) string {
	if len(s) < 2 {
		return s
	}

	if s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// BacktickQuote backtick-quotes (and escapes) s.
//
//	hello `world`  --> `hello ``world```
func BacktickQuote(s string) string {
	const q = '`'
	sb := strings.Builder{}
	sb.WriteRune(q)
	for _, r := range s {
		if r == q {
			sb.WriteRune(q)
		}
		sb.WriteRune(r)
	}
	sb.WriteRune(q)
	return sb.String()
}

// SingleQuote single-quotes (and escapes) s.
//
//	jessie's girl  -->  'jessie''s girl'
func SingleQuote(s string) string {
	const q = '\''
	sb := strings.Builder{}
	sb.WriteRune(q)
	for _, r := range s {
		if r == q {
			sb.WriteRune(q)
		}
		sb.WriteRune(r)
	}
	sb.WriteRune(q)
	return sb.String()
}
