// Package sqlw provides output.SQLWriter implementations for the
// --dry-run mode of the slq command, where the rendered SQL is printed
// instead of being executed.
package sqlw

import (
	"io"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/fatih/color"

	"github.com/neilotoole/sq/cli/output"
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
	_, err := io.WriteString(w.out, sql+"\n")
	return err
}

// highlight tokenises sql with chroma's SQL lexer and emits each token
// through the corresponding *color.Color slot from pr. Returns
// (highlighted, true) on success, or (sql, false) if the lexer or
// tokenisation fails — caller is expected to fall back to plain sql.
func highlight(sql string, pr *output.Printing) (string, bool) {
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
	for _, tok := range iter.Tokens() {
		clr := colorFor(tok.Type, pr)
		if clr == nil {
			buf.WriteString(tok.Value)
			continue
		}
		buf.WriteString(clr.Sprint(tok.Value))
	}
	return buf.String(), true
}

// colorFor maps a chroma token type to the matching color slot in pr,
// or nil if the token should be emitted without colorisation.
func colorFor(tt chroma.TokenType, pr *output.Printing) *color.Color {
	// Specific token types first (chroma puts TRUE/FALSE under
	// KeywordConstant and NULL under NameBuiltin in the SQL lexer).
	if tt == chroma.KeywordConstant {
		return pr.Bool
	}
	if tt == chroma.NameBuiltin {
		return pr.Null
	}
	// Sub-category groupings for literals (LiteralStringSingle's
	// SubCategory is LiteralString, etc.).
	if tt.SubCategory() == chroma.LiteralString {
		return pr.String
	}
	if tt.SubCategory() == chroma.LiteralNumber {
		return pr.Number
	}
	// Top-level categories for everything else.
	cat := tt.Category()
	if cat == chroma.Keyword {
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
