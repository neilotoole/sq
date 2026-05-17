package commonw

import (
	"fmt"
	"io"
	"strings"

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
// When pr's colors are enabled, the offending span in the input line
// is rendered with pr.ErrorHilite. The caret line uses the same
// underlying color.
func RenderParseError(w io.Writer, pr *output.Printing, pe *ast.ParseError) {
	if pe == nil || len(pe.Issues) == 0 {
		return
	}

	lines := strings.Split(pe.Input, "\n")

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

		// Emit the input line, hiliting the offending span if we have one.
		fmt.Fprint(w, "  ")
		if start >= 0 && stop >= start && stop <= len(srcRunes) {
			fmt.Fprint(w, string(srcRunes[:start]))
			pr.ErrorHilite.Fprint(w, string(srcRunes[start:stop]))
			fmt.Fprint(w, string(srcRunes[stop:]))
		} else {
			fmt.Fprint(w, srcLine)
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
