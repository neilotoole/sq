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

// suggestForToken returns the closest match in candidates to token, or "".
// "Closest" is judged by Levenshtein distance with a length-dependent
// threshold from maxEditDistance. Exact matches are NOT suggested (the
// token would not have been an error if it matched). Ties are broken by
// first-seen order in candidates.
func suggestForToken(token string, candidates []string) string {
	if token == "" || len(candidates) == 0 {
		return ""
	}
	threshold := maxEditDistance(len(token))
	best := ""
	bestDist := threshold + 1
	for _, c := range candidates {
		if c == token {
			return "" // exact match isn't a typo
		}
		d := levenshtein(token, c)
		if d < bestDist {
			best = c
			bestDist = d
		}
	}
	if bestDist > threshold {
		return ""
	}
	return best
}

// maxEditDistance returns the maximum acceptable edit distance for a
// suggestion against a token of length n. Short tokens tolerate fewer
// edits to avoid silly suggestions for unrelated 2-character inputs.
func maxEditDistance(n int) int {
	switch {
	case n >= 10:
		return 3
	case n >= 5:
		return 2
	default:
		return 1
	}
}

// levenshtein computes the edit distance between a and b. Iterative,
// two-row implementation — sufficient for short identifier-length
// strings.
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	if len(rb) == 0 {
		return len(ra)
	}
	prev := make([]int, len(rb)+1)
	curr := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(rb)]
}
