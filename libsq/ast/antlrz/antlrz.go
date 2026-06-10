// Package antlrz contains utilities for working with ANTLR4.
package antlrz

import (
	"strings"
	"sync"
	"unicode/utf8"

	antlr "github.com/antlr4-go/antlr/v4"
)

// TokenExtractor extracts the raw text of a parser rule from the input.
type TokenExtractor struct {
	input string
	lines []string
	once  sync.Once
}

// NewTokenExtractor returns a new TokenExtractor.
func NewTokenExtractor(input string) *TokenExtractor {
	return &TokenExtractor{input: input}
}

// Offset returns the start and stop byte offsets of the parser rule in the
// input. start is inclusive; stop is exclusive (input[start:stop] yields the
// rule's raw text).
func (l *TokenExtractor) Offset(prc antlr.ParserRuleContext) (start, stop int) {
	l.once.Do(func() {
		l.lines = strings.Split(l.input, "\n")
	})

	startToken := prc.GetStart()
	startLine := startToken.GetLine() - 1
	startCol := runeColToByteCol(l.lines[startLine], startToken.GetColumn())

	stopToken := prc.GetStop()
	stopLine := stopToken.GetLine() - 1
	stopCol := runeColToByteCol(l.lines[stopLine], stopToken.GetColumn()) + len(stopToken.GetText())

	for i := range startLine {
		startCol += len(l.lines[i]) + 1
	}

	for i := range stopLine {
		stopCol += len(l.lines[i]) + 1
	}

	return startCol, stopCol
}

// Extract extracts the raw text of the parser rule from the input. It may panic
// if the parser rule is not found in the input.
func (l *TokenExtractor) Extract(prc antlr.ParserRuleContext) string {
	l.once.Do(func() {
		l.lines = strings.Split(l.input, "\n")
	})

	startToken := prc.GetStart()
	startLine := startToken.GetLine() - 1
	startCol := runeColToByteCol(l.lines[startLine], startToken.GetColumn())

	stopToken := prc.GetStop()
	stopLine := stopToken.GetLine() - 1
	stopCol := runeColToByteCol(l.lines[stopLine], stopToken.GetColumn()) + len(stopToken.GetText())

	if startLine == stopLine {
		return l.lines[startLine][startCol:stopCol]
	}

	// multi-line
	var sb strings.Builder
	sb.WriteString(l.lines[startLine][startCol:])
	sb.WriteString("\n")
	for i := startLine + 1; i < stopLine; i++ {
		sb.WriteString(l.lines[i])
		sb.WriteString("\n")
	}
	sb.WriteString(l.lines[stopLine][:stopCol])

	return sb.String()
}

// runeColToByteCol converts an ANTLR-reported rune-based column position on a
// line to a Go byte offset into that line's underlying string. ANTLR's
// Token.GetColumn returns the count of runes preceding the token on its line;
// directly using that count as a byte index breaks whenever multi-byte UTF-8
// characters appear earlier on the same line.
func runeColToByteCol(line string, runeCol int) int {
	byteCol := 0
	for range runeCol {
		if byteCol >= len(line) {
			return byteCol
		}
		_, size := utf8.DecodeRuneInString(line[byteCol:])
		byteCol += size
	}
	return byteCol
}
