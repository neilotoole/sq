package commonw

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/fatih/color"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/ast"
)

const (
	// parseErrIndent is the two-space prefix used before input and caret lines.
	parseErrIndent = "  "
	// parseErrCaret is the repeated character that marks the offending span.
	parseErrCaret = "~"
)

// writeIssueSummary writes the one-line "sq: syntax error ..." summary for a
// single issue. It is shared by [RenderParseError] and
// [RenderParseErrorSummary] so the two renderings can't drift.
func writeIssueSummary(w io.Writer, pr *output.Printing, iss ast.ParseIssue) {
	pr.Error.Fprintf(w, "sq: syntax error at line %d, col %d: %s\n",
		iss.Line, iss.DisplayCol(), iss.Msg)
}

// RenderParseErrorSummary writes only the one-line summary for each issue in
// pe, e.g.:
//
//	sq: syntax error at line 1, col 10: unexpected 'mx'
//
// It omits the highlighted input line, caret, and "did you mean" suggestion
// that [RenderParseError] emits. This is the rendering used when the
// error.format.text.verbose option is false.
func RenderParseErrorSummary(w io.Writer, pr *output.Printing, pe *ast.ParseError) {
	if pe == nil || len(pe.Issues) == 0 {
		return
	}
	for _, iss := range pe.Issues {
		writeIssueSummary(w, pr, iss)
	}
}

// RenderParseError writes a multi-line, position-highlighted error
// rendering of pe to w. For each issue:
//
//	sq: syntax error at line L, col C: <msg>
//
//	  <input line>
//	  <caret line>
//
//	did you mean '<suggestion>'?  // only if Suggestion is set
//
// When pr's colors are enabled, the input line is colorized per-token
// using sq's standard palette (handle, key, keyword, number, string,
// punc, etc.), with pr.ErrorHilite overlaid on the offending span.
// The caret line uses pr.Error.
//
// Multi-line input falls back to plain text with hilite overlay only.
func RenderParseError(w io.Writer, pr *output.Printing, pe *ast.ParseError) {
	if pe == nil || len(pe.Issues) == 0 {
		return
	}

	lines := strings.Split(pe.Input, "\n")

	// Precompute each line's global rune start offset (lines are '\n'-
	// delimited; +1 per newline) so spanWithinLine can map an issue's absolute
	// span offsets to line-local without re-scanning preceding lines per issue.
	lineStarts := make([]int, len(lines))
	runeOff := 0
	for li, ln := range lines {
		lineStarts[li] = runeOff
		runeOff += utf8.RuneCountInString(ln) + 1
	}

	// Tokenize once for syntax-aware coloring of the input line.
	// Multi-line input uses the plain text + hilite fallback because
	// per-line slicing of tokens (which carry global rune offsets) is
	// not implemented.
	multiLine := strings.Contains(pe.Input, "\n")
	var tokens []ast.Token
	if !multiLine {
		tokens = ast.Tokenize(pe.Input)
	}

	for i, iss := range pe.Issues {
		if i > 0 {
			fmt.Fprintln(w)
		}
		writeIssueSummary(w, pr, iss)
		fmt.Fprintln(w)

		// Pick the source line. Line is 1-based.
		lineIdx := iss.Line - 1
		if lineIdx < 0 || lineIdx >= len(lines) {
			continue
		}
		srcLine := lines[lineIdx]

		// Compute span within srcLine, mapping the issue's absolute span
		// offsets onto this line via the precomputed line start offset.
		start, stop := spanWithinLine(srcLine, lineStarts[lineIdx], iss)

		srcRunes := []rune(srcLine)

		// Emit the input line. For single-line input we use token-driven
		// colorization (handle/keyword/number/etc.) with pr.ErrorHilite
		// overlaid on the offending span. For multi-line input we fall
		// back to plain text plus hilite; per-line slicing of tokens
		// (which carry global rune offsets) is not implemented.
		fmt.Fprint(w, parseErrIndent)
		switch {
		case multiLine:
			if start >= 0 && stop >= start && stop <= len(srcRunes) {
				fmt.Fprint(w, string(srcRunes[:start]))
				pr.ErrorHilite.Fprint(w, string(srcRunes[start:stop]))
				fmt.Fprint(w, string(srcRunes[stop:]))
			} else {
				fmt.Fprint(w, srcLine)
			}
		default:
			renderColorizedLine(w, pr, srcRunes, tokens, start, stop)
		}
		fmt.Fprintln(w)

		// Caret line.
		if start >= 0 && stop > start {
			fmt.Fprint(w, parseErrIndent)
			fmt.Fprint(w, strings.Repeat(" ", start))
			pr.Error.Fprint(w, strings.Repeat(parseErrCaret, stop-start))
			fmt.Fprintln(w)
		}

		// "did you mean 'max'?" directly follows the caret line (no blank
		// line). The quotes are faint and the suggestion itself takes the
		// string color, so the candidate reads as the focus.
		if iss.Suggestion != "" {
			fmt.Fprint(w, "did you mean ")
			pr.Faint.Fprint(w, "'")
			pr.String.Fprint(w, iss.Suggestion)
			pr.Faint.Fprint(w, "'")
			fmt.Fprintln(w, "?")
		}
	}
}

// colorForKind returns the *color.Color to use when rendering a token
// of the given kind, or nil if the token should render in default color.
func colorForKind(pr *output.Printing, kind ast.TokenKind) *color.Color {
	switch kind {
	case ast.TokenHandle:
		return pr.Handle
	case ast.TokenName:
		return pr.Key
	case ast.TokenKeyword:
		return pr.Bold
	case ast.TokenNumber:
		return pr.Number
	case ast.TokenString:
		return pr.String
	case ast.TokenBool:
		return pr.Bool
	case ast.TokenNull:
		return pr.Null
	case ast.TokenPunc:
		return pr.Punc
	case ast.TokenIdentifier, ast.TokenUnknown:
		// Render in default color (no ANSI codes).
		return nil
	}
	return nil
}

// renderColorizedLine writes srcRunes to w with per-token coloring from
// pr's palette, overlaying pr.ErrorHilite on the [hiliteStart, hiliteStop)
// span. tokens are the result of ast.Tokenize on the FULL input; the
// caller must ensure positions in tokens index directly into srcRunes
// (i.e., the input is single-line, or the line being rendered starts at
// rune offset 0).
func renderColorizedLine(
	w io.Writer,
	pr *output.Printing,
	srcRunes []rune,
	tokens []ast.Token,
	hiliteStart, hiliteStop int,
) {
	// Build per-rune color map.
	colors := make([]*color.Color, len(srcRunes))
	for _, tok := range tokens {
		c := colorForKind(pr, tok.Kind)
		if c == nil {
			continue
		}
		// Token positions are rune offsets in the full input; for
		// single-line use they index directly into srcRunes.
		hi := tok.Stop
		if hi >= len(srcRunes) {
			hi = len(srcRunes) - 1
		}
		for i := tok.Start; i <= hi; i++ {
			if i >= 0 {
				colors[i] = c
			}
		}
		// For string tokens (e.g. the SLQ literal "bob", whose double
		// quotes are part of the token), mute the surrounding quote
		// characters so the content reads as the focus.
		if tok.Kind == ast.TokenString {
			muteStringQuotes(colors, pr.Faint, tok.Start, hi)
		}
	}
	// Overlay ErrorHilite for the offending span.
	if hiliteStart >= 0 && hiliteStop > hiliteStart {
		end := min(hiliteStop, len(srcRunes))
		for i := hiliteStart; i < end; i++ {
			colors[i] = pr.ErrorHilite
		}
	}
	// Walk runs of same color.
	i := 0
	for i < len(srcRunes) {
		j := i + 1
		for j < len(srcRunes) && colors[j] == colors[i] {
			j++
		}
		segment := string(srcRunes[i:j])
		if colors[i] != nil {
			colors[i].Fprint(w, segment)
		} else {
			fmt.Fprint(w, segment)
		}
		i = j
	}
}

// muteStringQuotes re-paints the first and last positions of a string token
// in colors with faint, so the surrounding quote characters are visually
// muted while the inner content keeps the string color. The guard hi >
// start avoids muting a single-rune token (which can't be a valid quoted
// string, but is cheap defensive coding).
func muteStringQuotes(colors []*color.Color, faint *color.Color, start, hi int) {
	if hi <= start {
		return
	}
	if start >= 0 && start < len(colors) {
		colors[start] = faint
	}
	if hi >= 0 && hi < len(colors) {
		colors[hi] = faint
	}
}

// spanWithinLine returns the [start, stop) rune offsets within srcLine that
// the issue's offending span covers, or (-1, -1) when no span is available.
// lineStart is the global rune offset of srcLine's first rune within the
// full input; it maps the issue's absolute Span offsets (per ParseIssue's
// contract) onto offsets local to srcLine.
func spanWithinLine(srcLine string, lineStart int, iss ast.ParseIssue) (start, stop int) {
	srcRunes := []rune(srcLine)

	// Prefer the precise span when available, converted to line-local
	// offsets. Span.Stop is inclusive, so the exclusive stop is Stop+1.
	// Clamp to the line length: Span.Stop can sit at end-of-line for
	// EOF-synthesized tokens.
	if iss.Span != nil {
		ls := iss.Span.Start - lineStart
		if ls >= 0 && ls <= len(srcRunes) {
			end := min(iss.Span.Stop-lineStart+1, len(srcRunes))
			if end <= ls {
				// Zero-width span (e.g. the synthetic <EOF> token, whose Stop
				// is Start-1): emit a single-rune caret at the position so the
				// error still gets a visible marker.
				end = ls + 1
			}
			return ls, end
		}
	}

	// Fall back to the line-local Col (+ token rune width). Col is
	// line-relative, so it needs no lineStart adjustment.
	if iss.Col < 0 || iss.Col > len(srcRunes) {
		return -1, -1
	}
	end := iss.Col + len([]rune(iss.Token))
	if end == iss.Col {
		end = iss.Col + 1 // no token text: highlight a single rune
	}
	return iss.Col, min(end, len(srcRunes))
}
