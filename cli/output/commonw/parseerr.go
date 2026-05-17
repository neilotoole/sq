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

		// Emit the input line, hiliting the offending span if we have one.
		fmt.Fprint(w, "  ")
		if start >= 0 && stop >= start && stop <= len(srcLine) {
			fmt.Fprint(w, srcLine[:start])
			pr.ErrorHilite.Fprint(w, srcLine[start:stop])
			fmt.Fprint(w, srcLine[stop:])
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

// spanWithinLine returns the [start, stop) byte offsets within srcLine
// that the issue's offending span covers. Returns (-1, -1) when the
// span isn't available.
//
// StartChar/StopChar are 0-based absolute offsets into the full input;
// for a single-line SLQ query they translate directly to offsets in
// srcLine. For multi-line input we fall back to iss.Col + len(iss.Token).
func spanWithinLine(srcLine string, iss ast.ParseIssue) (start, stop int) {
	if iss.StartChar < 0 {
		// No token info — derive from Col.
		if iss.Col < 0 || iss.Col > len(srcLine) {
			return -1, -1
		}
		// Highlight one character at Col if no token text.
		end := iss.Col + len(iss.Token)
		if end == iss.Col {
			end = iss.Col + 1
		}
		if end > len(srcLine) {
			end = len(srcLine)
		}
		return iss.Col, end
	}

	// Single-line input is the common case.
	if !strings.ContainsRune(srcLine, '\n') &&
		iss.StartChar <= len(srcLine) && iss.StopChar < len(srcLine) {
		return iss.StartChar, iss.StopChar + 1
	}

	// Multi-line fallback.
	if iss.Col < 0 || iss.Col > len(srcLine) {
		return -1, -1
	}
	end := iss.Col + len(iss.Token)
	if end > len(srcLine) {
		end = len(srcLine)
	}
	return iss.Col, end
}
