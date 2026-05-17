package ast

import (
	"strings"
)

// expectedTokenLiterals returns the literal forms (quotes stripped) of
// the expected token types whose literal name is non-empty. Tokens that
// have only a symbolic name (e.g. NAME, NUMBER, STRING) are skipped
// because their literal text is not a useful suggestion target.
func expectedTokenLiterals(tokenTypes []int, literalNames []string) []string {
	out := make([]string, 0, len(tokenTypes))
	for _, ttype := range tokenTypes {
		if ttype < 0 || ttype >= len(literalNames) {
			continue
		}
		lit := literalNames[ttype]
		if lit == "" {
			continue
		}
		// ANTLR literal names look like "'sum'" — strip the surrounding quotes.
		lit = strings.Trim(lit, "'")
		if lit == "" {
			continue
		}
		out = append(out, lit)
	}
	return out
}

// collectExpectedTokenTypes flattens an ANTLR IntervalSet (represented
// here as the list of expected token ID pairs we already collected in
// SyntaxError) into a slice. Defined as a small helper so SyntaxError
// stays readable.
func collectExpectedTokenTypes(intervals [][2]int) []int {
	var out []int
	for _, iv := range intervals {
		for tt := iv[0]; tt <= iv[1]; tt++ {
			out = append(out, tt)
		}
	}
	return out
}
