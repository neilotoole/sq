package mermaidw

import (
	"regexp"
	"strings"

	"github.com/fatih/color"

	"github.com/neilotoole/sq/cli/output"
)

// colorize applies ANSI color to a rendered Mermaid erDiagram source, wrapping
// each token with its mapped output.Printing color. It returns src unchanged
// when pr is nil or in monochrome mode, so non-TTY sinks (files, pipes,
// --no-color, NO_COLOR, --monochrome) stay byte-identical to the plain diagram
// — ANSI escapes would corrupt a redirected .mmd. The per-line grammar mirrors
// the docs-site tokenizer added in #691
// (site/layouts/_default/_markup/render-codeblock-mermaid.html).
func colorize(src string, pr *output.Printing) string {
	if pr == nil || pr.IsMonochrome() {
		return src
	}

	lines := strings.Split(src, "\n")
	for i, line := range lines {
		lines[i] = colorizeLine(line, pr)
	}
	return strings.Join(lines, "\n")
}

// Per-line token regexes, each capturing every character (including
// whitespace) so reassembly is lossless: stripping the emitted ANSI escapes
// reproduces the input exactly. Entity operands may be quoted (sq quotes any
// name that isn't a bare identifier), so they match "..." or a bare token.
//
// The relationship operator is an ER cardinality token — glyphs (| o } {)
// around a --/.. connector — rather than any non-space run. Requiring a real
// cardinality token here is what distinguishes a relationship from a 3-token
// attribute line ("int id PK"), and stops a quoted entity name that merely
// contains a "--" substring from being misread as a relationship.
const cardOp = `[|}o{]+(?:--|\.\.)[|}o{]+`

var (
	reComment     = regexp.MustCompile(`^(\s*)(%%.*)$`)
	reKeyword     = regexp.MustCompile(`^(\s*)(erDiagram)(\s*)$`)
	reRelLabel    = regexp.MustCompile(`^(\s*)("[^"]*"|\S+)(\s+)(` + cardOp + `)(\s+)("[^"]*"|\S+)(\s+:\s+)(".*")(\s*)$`)
	reRel         = regexp.MustCompile(`^(\s*)("[^"]*"|\S+)(\s+)(` + cardOp + `)(\s+)("[^"]*"|\S+)(\s*)$`)
	reEntityOpen  = regexp.MustCompile(`^(\s*)("[^"]*"|\S+)(\s*)(\{)(\s*)$`)
	reEntityClose = regexp.MustCompile(`^(\s*)(\})(\s*)$`)
	// reAttr matches "type name [keys]" where keys is PK/FK/UK, comma- or
	// space-joined (sq joins with a comma, e.g. "PK,FK"). The optional keys
	// group includes its own leading whitespace. Unlike #691's attribute rule,
	// there is no trailing-comment group: sq's renderer sanitizes attribute
	// names/types to bare words and never emits an attribute-level comment, so
	// the simpler shape suffices and an unexpected line falls through to plain.
	reAttr = regexp.MustCompile(`^(\s*)("[^"]*"|\S+)(\s+)("[^"]*"|\S+)(\s+(?:PK|FK|UK)(?:[ ,](?:PK|FK|UK))*)?(\s*)$`)
)

// colorizeLine colorizes a single erDiagram line. Lines that match no known
// token shape are returned unchanged, so an unexpected line is emitted plain
// rather than corrupted.
func colorizeLine(line string, pr *output.Printing) string {
	trimmed := strings.TrimSpace(line)
	switch {
	case trimmed == "":
		return line
	case strings.HasPrefix(trimmed, "%%"):
		return assemble(reComment, line, nil, pr.Faint)
	case trimmed == "erDiagram":
		return assemble(reKeyword, line, nil, pr.Key, nil)
	case trimmed == "}":
		return assemble(reEntityClose, line, nil, pr.Punc, nil)
	case strings.HasSuffix(trimmed, "{"):
		// NAME {: name -> Header, brace -> Punc. Checked before the
		// relationship branch so a quoted name containing a cardinality-like
		// substring (e.g. "weird o--table") is still colorized as an entity.
		return assemble(reEntityOpen, line, nil, pr.Header, nil, pr.Punc, nil)
	default:
		// Relationship: LEFT op RIGHT [: "label"]; entities -> Header,
		// operator -> Punc, label -> String, the " : " separator stays default.
		// The strict cardinality operator (see the regex block) keeps an
		// attribute line ("int id PK") from matching here.
		if s, ok := assembleOK(reRelLabel, line,
			nil, pr.Header, nil, pr.Punc, nil, pr.Header, nil, pr.String, nil); ok {
			return s
		}
		if s, ok := assembleOK(reRel, line,
			nil, pr.Header, nil, pr.Punc, nil, pr.Header, nil); ok {
			return s
		}
		// Attribute: type -> Number, name -> default, keys -> Key.
		return assemble(reAttr, line, nil, pr.Number, nil, nil, pr.Key, nil)
	}
}

// assemble matches re against line and rebuilds it, wrapping each capture
// group with the color at the same position in clrs (a nil entry leaves that
// group uncolored). It returns line unchanged when re doesn't match.
func assemble(re *regexp.Regexp, line string, clrs ...*color.Color) string {
	s, _ := assembleOK(re, line, clrs...)
	return s
}

// assembleOK is assemble, additionally reporting whether re matched.
func assembleOK(re *regexp.Regexp, line string, clrs ...*color.Color) (string, bool) {
	m := re.FindStringSubmatch(line)
	if m == nil {
		return line, false
	}

	var b strings.Builder
	// m[0] is the whole match; capture groups start at m[1], aligned with clrs.
	for i, group := range m[1:] {
		if i < len(clrs) && clrs[i] != nil {
			b.WriteString(span(clrs[i], group))
			continue
		}
		b.WriteString(group)
	}
	return b.String(), true
}

// span wraps s with clr, leaving an empty string untouched so an absent
// optional capture group contributes no stray escape sequence.
func span(clr *color.Color, s string) string {
	if s == "" {
		return s
	}
	return clr.Sprint(s)
}
