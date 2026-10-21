// Package vecdecl parses SQLite vector declarations: sqlite-vec vec0 virtual
// table DDL, libSQL declared vector column types, and libSQL vector index
// definitions. It is pure Go (no cgo) and doesn't depend on the sqlite3
// driver, so that other drivers (e.g. rqlite) can reuse it.
package vecdecl

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/vector"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// Role is the role of a column in a vec0 virtual table.
type Role string

const (
	// RoleVector is a vector column.
	RoleVector Role = "vector"

	// RolePrimaryKey is a primary key column.
	RolePrimaryKey Role = "primary_key"

	// RolePartitionKey is a partition key column.
	RolePartitionKey Role = "partition_key"

	// RoleMetadata is a metadata column.
	RoleMetadata Role = "metadata"

	// RoleAuxiliary is an auxiliary column (declared with a "+" prefix).
	RoleAuxiliary Role = "auxiliary"
)

// Vec0Column describes one column declared in a vec0 virtual table.
type Vec0Column struct {
	// Vector is set for RoleVector, and is nil otherwise.
	Vector *vector.Type

	// Name is the column name, with any quoting removed.
	Name string

	// Role is the column's role.
	Role Role

	// DeclType is the type text as written, e.g. "float[4]" or "INTEGER".
	DeclType string

	// IndexedBy is the raw clause following "indexed by", e.g.
	// "rescore(quantizer=bit, oversample=50)". Empty if absent.
	IndexedBy string
}

const identPat = `(?:"(?:[^"]|"")*"|\[[^\]]*\]|` + "`[^`]*`" + `|'[^']*'|[^\s(.,]+)`

var (
	rxVirtualModule = regexp.MustCompile(`(?is)^\s*create\s+virtual\s+table\s+(?:if\s+not\s+exists\s+)?` +
		identPat + `(?:\s*\.\s*` + identPat + `)?\s+using\s+([A-Za-z0-9_]+)`)

	rxLibSQLIndex = regexp.MustCompile(`(?is)^\s*create\s+(?:unique\s+)?index\s+(?:if\s+not\s+exists\s+)?` +
		identPat + `(?:\s*\.\s*` + identPat + `)?\s+on\s+` + identPat +
		`\s*\(\s*libsql_vector_idx\s*\(`)

	rxLibSQLType = regexp.MustCompile(`(?i)^(f32_blob|float32|f64_blob|float64|f16_blob|float16|` +
		`fb16_blob|floatb16|f8_blob|float8|f1bit_blob|float1bit)\s*\(\s*(\d+)\s*\)$`)

	rxVecType   = regexp.MustCompile(`(?s)^(\w+)\s*\[\s*([^\]]*?)\s*\]`)
	rxIndexedBy = regexp.MustCompile(`(?is)\bindexed\s+by\s+`)
	rxMetric    = regexp.MustCompile(`(?i)\bdistance_metric\s*=\s*(\w+)`)
	rxPKey      = regexp.MustCompile(`(?i)\bprimary\s+key\b`)
	rxPartKey   = regexp.MustCompile(`(?i)\bpartition\s+key\b`)
)

// VirtualTableModule returns the lowercased module name of a CREATE VIRTUAL
// TABLE statement (e.g. "vec0", "fts5"), or "" if createSQL is not a
// virtual table declaration.
func VirtualTableModule(createSQL string) string {
	m := rxVirtualModule.FindStringSubmatch(stripComments(createSQL))
	if m == nil {
		return ""
	}
	return strings.ToLower(m[1])
}

// ParseVec0 parses a vec0 CREATE VIRTUAL TABLE statement and returns its
// columns in declaration order. Table options (e.g. "chunk_size=8") are not
// columns and are skipped.
func ParseVec0(createSQL string) ([]Vec0Column, error) {
	createSQL = stripComments(createSQL)
	if VirtualTableModule(createSQL) != "vec0" {
		return nil, errz.New("not a vec0 virtual table declaration")
	}

	loc := rxVirtualModule.FindStringIndex(createSQL)
	rest := createSQL[loc[1]:]
	open := len(rest) - len(strings.TrimLeft(rest, " \t\r\n\f"))
	if open >= len(rest) || rest[open] != '(' {
		return nil, errz.New("vec0 declaration has no argument list")
	}

	end, err := matchParen(rest, open)
	if err != nil {
		return nil, err
	}

	var cols []Vec0Column
	for _, arg := range splitTop(rest[open+1 : end]) {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		col, ok, err := parseVec0Arg(arg)
		if err != nil {
			return nil, err
		}
		if ok {
			cols = append(cols, col)
		}
	}

	return cols, nil
}

// parseVec0Arg parses a single vec0 argument. The returned bool is false
// if the argument is a table option rather than a column.
func parseVec0Arg(arg string) (col Vec0Column, ok bool, err error) {
	aux := false
	if strings.HasPrefix(arg, "+") {
		aux = true
		arg = strings.TrimSpace(arg[1:])
	}

	name, bare, rest := readIdent(arg)
	if name == "" {
		return col, false, errz.Errorf("invalid vec0 column definition: %q", arg)
	}
	rest = strings.TrimSpace(rest)
	if !aux && bare && (strings.Contains(name, "=") || strings.HasPrefix(rest, "=")) {
		return col, false, nil
	}

	col.Name = name
	if rest == "" {
		return col, false, errz.Errorf("vec0 column %q has no type", name)
	}

	if m := rxVecType.FindStringSubmatch(rest); m != nil {
		elem, isVec := vec0Elem(m[1])
		if !isVec {
			return col, false, errz.Errorf("vec0 column %q has unknown vector element type %q", name, m[1])
		}
		dim, convErr := strconv.Atoi(m[2])
		if convErr != nil || dim <= 0 {
			return col, false, errz.Errorf("vec0 column %q has invalid dimension %q", name, m[2])
		}
		col.Role = RoleVector
		col.DeclType = rest[:len(m[0])]
		col.Vector = &vector.Type{Dim: dim, Elem: elem}

		opts := rest[len(m[0]):]
		if loc := rxIndexedBy.FindStringIndex(opts); loc != nil {
			col.IndexedBy = strings.TrimSpace(opts[loc[1]:])
			opts = opts[:loc[0]]
		}
		if mm := rxMetric.FindStringSubmatch(opts); mm != nil {
			col.Vector.Metric = strings.ToLower(mm[1])
		}
		return col, true, nil
	}

	_, _, tail := readIdent(rest)
	col.DeclType = rest[:len(rest)-len(tail)]
	switch {
	case aux:
		col.Role = RoleAuxiliary
	case rxPKey.MatchString(tail):
		col.Role = RolePrimaryKey
	case rxPartKey.MatchString(tail):
		col.Role = RolePartitionKey
	default:
		col.Role = RoleMetadata
	}
	col.DeclType = strings.TrimSpace(col.DeclType)
	return col, true, nil
}

func vec0Elem(s string) (vector.Elem, bool) {
	switch strings.ToLower(s) {
	case "float", "f32":
		return vector.F32, true
	case "int8", "i8":
		return vector.I8, true
	case "bit":
		return vector.Bit, true
	default:
		return "", false
	}
}

// ParseLibSQLType parses a libSQL declared column type such as F32_BLOB(4)
// or FLOAT32(4). It returns false if declType is not a libSQL vector type
// with a positive dimension.
func ParseLibSQLType(declType string) (vector.Type, bool) {
	m := rxLibSQLType.FindStringSubmatch(strings.TrimSpace(declType))
	if m == nil {
		return vector.Type{}, false
	}
	dim, err := strconv.Atoi(m[2])
	if err != nil || dim <= 0 {
		return vector.Type{}, false
	}

	var elem vector.Elem
	switch strings.ToLower(m[1]) {
	case "f32_blob", "float32":
		elem = vector.F32
	case "f64_blob", "float64":
		elem = vector.F64
	case "f16_blob", "float16":
		elem = vector.F16
	case "fb16_blob", "floatb16":
		elem = vector.BF16
	case "f8_blob", "float8":
		elem = vector.F8
	default:
		elem = vector.Bit
	}
	return vector.Type{Dim: dim, Elem: elem}, true
}

// ParseLibSQLVectorIndex parses a libSQL vector index definition of the form
// CREATE INDEX i ON t(libsql_vector_idx(col, 'k=v', ...)). It returns the
// indexed column and the index details, or ok false if createIndexSQL is
// not a libSQL vector index.
func ParseLibSQLVectorIndex(createIndexSQL string) (col string, iv *metadata.IndexVector, ok bool) {
	s := stripComments(createIndexSQL)
	loc := rxLibSQLIndex.FindStringIndex(s)
	if loc == nil {
		return "", nil, false
	}

	open := loc[1] - 1
	end, err := matchParen(s, open)
	if err != nil {
		return "", nil, false
	}

	args := splitTop(s[open+1 : end])
	if len(args) == 0 {
		return "", nil, false
	}
	col, _, tail := readIdent(strings.TrimSpace(args[0]))
	if col == "" || strings.TrimSpace(tail) != "" {
		return "", nil, false
	}

	iv = &metadata.IndexVector{Method: "diskann"}
	for _, a := range args[1:] {
		a = strings.TrimSpace(a)
		if len(a) >= 2 && (a[0] == '\'' || a[0] == '"') && a[len(a)-1] == a[0] {
			a = a[1 : len(a)-1]
		}
		k, v, found := strings.Cut(a, "=")
		if !found {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if strings.EqualFold(k, "metric") {
			iv.Metric = strings.ToLower(v)
			continue
		}
		if iv.Params == nil {
			iv.Params = map[string]string{}
		}
		iv.Params[k] = v
	}
	return col, iv, true
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

// skipQuoted returns the index just past the quoted section that starts at
// s[i], or -1 if it's unterminated. A "[" opens a bracket-quoted section.
func skipQuoted(s string, i int) int {
	closer := s[i]
	if closer == '[' {
		closer = ']'
	}
	for j := i + 1; j < len(s); j++ {
		if s[j] != closer {
			continue
		}
		if closer != ']' && j+1 < len(s) && s[j+1] == closer {
			j++ // Doubled quote is an escape.
			continue
		}
		return j + 1
	}
	return -1
}

// stripComments replaces each SQL comment in s, a "--" comment running to
// the end of its line or a "/* */" comment, with a single space. Comment
// markers inside quotes are left alone. An unterminated comment runs to the
// end of s, as in SQLite.
func stripComments(s string) string {
	if !strings.Contains(s, "--") && !strings.Contains(s, "/*") {
		return s
	}

	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); {
		switch {
		case s[i] == '"' || s[i] == '\'' || s[i] == '`' || s[i] == '[':
			j := skipQuoted(s, i)
			if j < 0 {
				j = len(s)
			}
			sb.WriteString(s[i:j])
			i = j
		case strings.HasPrefix(s[i:], "--"):
			j := strings.IndexByte(s[i:], '\n')
			if j < 0 {
				j = len(s) - i
			}
			sb.WriteByte(' ')
			i += j
		case strings.HasPrefix(s[i:], "/*"):
			j := strings.Index(s[i+2:], "*/")
			if j < 0 {
				i = len(s)
			} else {
				i += 2 + j + 2
			}
			sb.WriteByte(' ')
		default:
			sb.WriteByte(s[i])
			i++
		}
	}
	return sb.String()
}

// matchParen returns the index of the ")" matching the "(" at s[open].
func matchParen(s string, open int) (int, error) {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '"', '\'', '`', '[':
			j := skipQuoted(s, i)
			if j < 0 {
				return 0, errz.New("unterminated quote in declaration")
			}
			i = j - 1
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, errz.New("unbalanced parentheses in declaration")
}

// splitTop splits s on commas that are not nested in parentheses or quotes.
func splitTop(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"', '\'', '`', '[':
			if j := skipQuoted(s, i); j >= 0 {
				i = j - 1
			}
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

// readIdent reads a leading identifier from s. The identifier may be quoted
// with double quotes, brackets, or backticks, which are stripped. The bare
// result reports whether it was unquoted. rest is the remainder of s.
func readIdent(s string) (name string, bare bool, rest string) {
	s = strings.TrimLeft(s, " \t\r\n\f")
	if s == "" {
		return "", false, ""
	}
	switch s[0] {
	case '"', '`', '[', '\'':
		j := skipQuoted(s, 0)
		if j < 0 {
			return "", false, s
		}
		inner := s[1 : j-1]
		if s[0] == '"' || s[0] == '`' || s[0] == '\'' {
			inner = strings.ReplaceAll(inner, s[:1]+s[:1], s[:1])
		}
		return inner, false, s[j:]
	}
	j := 0
	for j < len(s) && !isSpace(s[j]) && s[j] != '(' && s[j] != ',' {
		j++
	}
	return s[:j], true, s[j:]
}
