package commonw

import (
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/ast"
)

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
// punc, etc.), with pr.ErrorHilite overlayed on the offending span.
// The caret line uses pr.Error.
//
// Multi-line input falls back to plain text with hilite overlay only.
func RenderParseError(w io.Writer, pr *output.Printing, pe *ast.ParseError) {
	if pe == nil || len(pe.Issues) == 0 {
		return
	}

	lines := strings.Split(pe.Input, "\n")

	// Tokenize once for syntax-aware coloring of the input line.
	// Skipped for multi-line input (we fall back to plain rendering there
	// because token positions are global rune offsets and per-line slicing
	// isn't implemented yet).
	multiLine := strings.Contains(pe.Input, "\n")
	var tokens []ast.Token
	if !multiLine {
		tokens = ast.Tokenize(pe.Input)
	}

	for i, iss := range pe.Issues {
		if i > 0 {
			fmt.Fprintln(w)
		}
		pr.Error.Fprintf(w, "sq: syntax error at line %d, col %d: %s\n",
			iss.Line, iss.Col+1, iss.Msg)
		fmt.Fprintln(w)

		// Pick the source line. Line is 1-based.
		lineIdx := iss.Line - 1
		if lineIdx < 0 || lineIdx >= len(lines) {
			continue
		}
		srcLine := lines[lineIdx]

		// Compute span within srcLine.
		start, stop := spanWithinLine(srcLine, iss)

		// srcLine is sliced as runes because StartChar/StopChar are rune
		// indices (ANTLR-go InputStream uses []rune internally).
		srcRunes := []rune(srcLine)

		// Emit the input line. For single-line input we use token-driven
		// colorization (handle/keyword/number/etc.) with pr.ErrorHilite
		// overlayed on the offending span. For multi-line input we fall
		// back to plain text plus hilite, since token positions are global
		// rune offsets and the line-extraction work for multi-line isn't
		// implemented yet.
		fmt.Fprint(w, "  ")
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
			fmt.Fprint(w, "  ")
			fmt.Fprint(w, strings.Repeat(" ", start))
			pr.Error.Fprint(w, strings.Repeat("~", stop-start))
			fmt.Fprintln(w)
		}

		if iss.Suggestion != "" {
			fmt.Fprintln(w)
			fmt.Fprintf(w, "did you mean '%s'?\n", iss.Suggestion)
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
// span. tokens is the result of ast.Tokenize on the full input; only
// tokens that fall inside [0, len(srcRunes)) are used. Suitable only for
// single-line input (lineStartOffset == 0).
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

// spanWithinLine returns the [start, stop) rune offsets within srcLine
// that the issue's offending span covers. Returns (-1, -1) when the
// span isn't available.
//
// StartChar/StopChar are 0-based absolute rune offsets into the full
// input (ANTLR-go InputStream stores input as []rune). For a single-line
// SLQ query they translate directly to offsets in srcLine. For multi-line
// input we fall back to iss.Col + rune-length of iss.Token.
func spanWithinLine(srcLine string, iss ast.ParseIssue) (start, stop int) {
	srcRunes := []rune(srcLine)
	if iss.StartChar < 0 {
		// No token info — derive from Col.
		if iss.Col < 0 || iss.Col > len(srcRunes) {
			return -1, -1
		}
		// Highlight one character at Col if no token text.
		tokenRunes := len([]rune(iss.Token))
		end := iss.Col + tokenRunes
		if end == iss.Col {
			end = iss.Col + 1
		}
		end = min(end, len(srcRunes))
		return iss.Col, end
	}

	// Single-line common case: absolute offsets fall within this line.
	if iss.StartChar <= len(srcRunes) && iss.StopChar < len(srcRunes) {
		return iss.StartChar, iss.StopChar + 1
	}

	// Multi-line fallback.
	if iss.Col < 0 || iss.Col > len(srcRunes) {
		return -1, -1
	}
	end := iss.Col + len([]rune(iss.Token))
	end = min(end, len(srcRunes))
	return iss.Col, end
}
