package ast

import (
	"fmt"
	"strings"
	"unicode"

	antlr "github.com/antlr4-go/antlr/v4"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
)

// TokenKind is a high-level classification of a lexed SLQ token, used by
// renderers to apply syntax-aware colorization without depending on
// ANTLR's raw token IDs.
type TokenKind int

// TokenKind values.
const (
	// TokenUnknown is the zero value; used when a token type doesn't fit
	// any of the other kinds.
	TokenUnknown TokenKind = iota

	// TokenHandle is a source handle like @sakila/local/sl3.
	TokenHandle

	// TokenName is a dotted identifier like .actor or .first_name.
	TokenName

	// TokenIdentifier is a bare identifier (no leading dot or @).
	TokenIdentifier

	// TokenKeyword is a reserved word (sum, avg, max, WHERE, GROUP_BY, etc.).
	TokenKeyword

	// TokenNumber is a numeric literal.
	TokenNumber

	// TokenString is a quoted string literal.
	TokenString

	// TokenBool is a boolean literal (true/false).
	TokenBool

	// TokenNull is the null literal.
	TokenNull

	// TokenPunc is punctuation or an operator (| ( ) [ ] : , . + - etc.).
	TokenPunc
)

// String returns a human-readable name for the kind, suitable for
// logs and test failure messages. The strings are not part of any
// public API contract.
func (k TokenKind) String() string {
	switch k {
	case TokenHandle:
		return "handle"
	case TokenName:
		return "name"
	case TokenIdentifier:
		return "identifier"
	case TokenKeyword:
		return "keyword"
	case TokenNumber:
		return "number"
	case TokenString:
		return "string"
	case TokenBool:
		return "bool"
	case TokenNull:
		return "null"
	case TokenPunc:
		return "punc"
	case TokenUnknown:
		return "unknown"
	}
	return fmt.Sprintf("TokenKind(%d)", int(k))
}

// Token is a single lexed token from an SLQ input.
type Token struct {
	// Text is the raw token text from the input.
	Text string

	// Kind is the high-level classification.
	Kind TokenKind

	// Start is the 0-based rune offset of the token's first character
	// in the original input.
	Start int

	// Stop is the 0-based rune offset of the token's last character
	// (inclusive). For a one-character token, Stop == Start.
	Stop int

	// Line is the 1-based line number where the token begins.
	Line int
}

// Tokenize runs the SLQ lexer over input and returns visible (non-skipped)
// tokens. Whitespace and line comments are not returned; the lexer discards
// them before this function sees the token stream. Error reporting is
// suppressed by removing the lexer's error listeners: on a lex error (e.g.,
// an unrecognized character) the lexer still performs its default recovery
// (skipping the offending input) and Tokenize returns whatever tokens it
// produced, with no error indication. Tokenize is for presentation only
// (syntax-aware colorization); authoritative error reporting is done by
// parseSLQ.
func Tokenize(input string) []Token {
	if input == "" {
		return nil
	}
	lex := slq.NewSLQLexer(antlr.NewInputStream(input))
	lex.RemoveErrorListeners() // we don't surface lex errors here
	literals := lex.LiteralNames

	var out []Token
	for {
		t := lex.NextToken()
		if t == nil {
			break
		}
		ttype := t.GetTokenType()
		if ttype == antlr.TokenEOF {
			break
		}
		var lit string
		if ttype >= 0 && ttype < len(literals) {
			lit = literals[ttype]
		}
		out = append(out, Token{
			Text:  t.GetText(),
			Kind:  tokenKindOf(ttype, lit),
			Start: t.GetStart(),
			Stop:  t.GetStop(),
			Line:  t.GetLine(),
		})
	}
	return out
}

// tokenKindOf classifies an ANTLR token type into a high-level TokenKind.
// For anonymous keyword/punctuation tokens (T__N), the lexer's literal
// name is inspected: alphabetic literals become keywords, non-alphabetic
// ones become punctuation.
func tokenKindOf(ttype int, literal string) TokenKind {
	switch ttype {
	case slq.SLQLexerHANDLE:
		return TokenHandle
	case slq.SLQLexerNAME:
		return TokenName
	case slq.SLQLexerID, slq.SLQLexerARG:
		return TokenIdentifier
	case slq.SLQLexerSTRING:
		return TokenString
	case slq.SLQLexerNUMBER, slq.SLQLexerNN, slq.SLQLexerDIGITS, slq.SLQLexerIDNUM:
		return TokenNumber
	case slq.SLQLexerBOOL:
		return TokenBool
	case slq.SLQLexerNULL:
		return TokenNull
	case slq.SLQLexerPROPRIETARY_FUNC_NAME,
		slq.SLQLexerJOIN_TYPE,
		slq.SLQLexerWHERE,
		slq.SLQLexerGROUP_BY,
		slq.SLQLexerHAVING,
		slq.SLQLexerORDER_BY,
		slq.SLQLexerALIAS_RESERVED:
		return TokenKeyword
	case slq.SLQLexerLPAR,
		slq.SLQLexerRPAR,
		slq.SLQLexerLBRA,
		slq.SLQLexerRBRA,
		slq.SLQLexerCOMMA,
		slq.SLQLexerPIPE,
		slq.SLQLexerCOLON,
		slq.SLQLexerLT,
		slq.SLQLexerLT_EQ,
		slq.SLQLexerGT,
		slq.SLQLexerGT_EQ,
		slq.SLQLexerNEQ,
		slq.SLQLexerEQ:
		return TokenPunc
	}

	// Anonymous literal tokens (T__N): use the literal text to decide.
	s := strings.Trim(literal, "'")
	if s == "" {
		return TokenUnknown
	}
	if isAlphaWord(s) {
		return TokenKeyword
	}
	return TokenPunc
}

// isAlphaWord reports whether s consists entirely of letters and
// underscores (i.e., looks like a keyword rather than punctuation).
func isAlphaWord(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && r != '_' {
			return false
		}
	}
	return true
}
