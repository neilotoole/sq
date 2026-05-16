// Package sqlw provides output.SQLWriter implementations for the
// --render-sql mode of the slq command, where the rendered SQL is
// printed instead of being executed.
package sqlw

import (
	"io"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/fatih/color"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// NewTextWriter returns an output.SQLWriter that emits plain SQL
// followed by a newline. When pr has color enabled, the SQL is
// syntax-highlighted using sq's existing terminal palette so it blends
// with the rest of sq's output. When pr is monochrome the SQL is
// written as-is.
func NewTextWriter(out io.Writer, pr *output.Printing) *TextWriter {
	return &TextWriter{out: out, pr: pr}
}

// TextWriter is the SQLWriter implementation for the text/raw formats.
type TextWriter struct {
	out io.Writer
	pr  *output.Printing
}

// Render implements output.SQLWriter.
func (w *TextWriter) Render(p output.SQLPayload) error {
	sql := p.SQL
	if !w.pr.IsMonochrome() {
		if h, ok := highlight(sql, w.pr); ok {
			sql = h
		}
	}
	if _, err := io.WriteString(w.out, sql+"\n"); err != nil {
		return errz.Err(err)
	}
	return nil
}

// highlight tokenises sql with chroma's SQL lexer and emits each token
// through the corresponding *color.Color slot from pr. Returns
// (highlighted, true) on success, or (sql, false) if the lexer is
// missing, tokenisation fails, or the iterator panics — caller is
// expected to fall back to plain sql. Chroma's docs note that iterators
// "may propagate [errors] in a panic", so the recover is required.
func highlight(sql string, pr *output.Printing) (out string, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			out, ok = sql, false
		}
	}()

	lexer := lexers.Get("sql")
	if lexer == nil {
		return sql, false
	}
	iter, err := lexer.Tokenise(nil, sql)
	if err != nil {
		return sql, false
	}

	var buf strings.Builder
	buf.Grow(len(sql) * 2) // rough overhead allowance for escape codes
	for tok := iter(); tok != chroma.EOF; tok = iter() {
		clr := colorFor(tok, pr)
		if clr == nil {
			buf.WriteString(tok.Value)
			continue
		}
		buf.WriteString(clr.Sprint(tok.Value))
	}
	return buf.String(), true
}

// colorFor maps a chroma token type (and its value) to the matching
// color slot in pr, or nil if the token should be emitted without
// colorisation. The value is needed because chroma's SQL lexer
// classifies TRUE/FALSE/NULL as plain Keyword, so we recover the
// finer-grained semantics from the token text.
func colorFor(tok chroma.Token, pr *output.Printing) *color.Color {
	tt := tok.Type

	// Literals via sub-category. chroma's SQL lexer emits the opening
	// quote, the content, and the closing quote as separate tokens
	// (all under LiteralStringSingle / LiteralStringDouble). Route
	// the content to pr.String (green) and the quote characters
	// themselves to pr.Faint so the user can visually distinguish
	// a string literal like 'TOM' from a double-quoted identifier
	// like "actor" by the dimness of the quote, while still
	// highlighting the content uniformly.
	if tt.SubCategory() == chroma.LiteralString {
		if len(tok.Value) > 0 {
			switch tok.Value[0] {
			case '"', '\'', '`':
				return pr.Faint
			}
		}
		return pr.String
	}
	if tt.SubCategory() == chroma.LiteralNumber {
		return pr.Number
	}

	cat := tt.Category()

	// Keywords: separate booleans and NULL by value, since chroma's
	// SQL lexer does not split them out into distinct token types.
	// EqualFold avoids per-token allocations from strings.ToUpper.
	if cat == chroma.Keyword {
		switch {
		case strings.EqualFold(tok.Value, "TRUE"),
			strings.EqualFold(tok.Value, "FALSE"):
			return pr.Bool
		case strings.EqualFold(tok.Value, "NULL"):
			return pr.Null
		}
		return pr.Key
	}

	if cat == chroma.Operator || cat == chroma.Punctuation {
		return pr.Punc
	}
	if cat == chroma.Comment {
		return pr.Faint
	}
	return nil
}
