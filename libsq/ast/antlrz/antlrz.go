// Package antlrz contains utilities for working with ANTLR4.
package antlrz

import (
	"strings"
	"sync"

	"github.com/antlr4-go/antlr/v4"
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

// Offset returns the start and stop (inclusive) offsets of the parser rule in
// the input.
func (l *TokenExtractor) Offset(prc antlr.ParserRuleContext) (start, stop int) {
	l.once.Do(func() {
		l.lines = strings.Split(l.input, "\n")
	})

	startToken := prc.GetStart()
	startLine := startToken.GetLine() - 1
	startCol := startToken.GetColumn()

	stopToken := prc.GetStop()
	stopLine := stopToken.GetLine() - 1
	stopCol := stopToken.GetColumn() + len(stopToken.GetText())

	for i := 0; i < startLine; i++ {
		startCol += len(l.lines[i]) + 1
	}

	for i := 0; i < stopLine; i++ {
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
	startCol := startToken.GetColumn()

	stopToken := prc.GetStop()
	stopLine := stopToken.GetLine() - 1
	stopCol := stopToken.GetColumn() + len(stopToken.GetText())

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
