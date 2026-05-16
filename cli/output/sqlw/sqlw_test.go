package sqlw_test

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/fatih/color"
	goccy "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/sqlw"
)

func newMonochromePrinting() *output.Printing {
	pr := output.NewPrinting()
	pr.EnableColor(false)
	return pr
}

// newColorPrinting returns a *output.Printing with color enabled, and
// pins fatih/color's package-level NoColor to false for the duration
// of the test. Without this, NO_COLOR env vars or parallel tests
// touching the global can suppress ANSI output and make the
// "should contain ANSI escapes" assertions flaky.
//
// CAUTION: tests that call this MUST NOT call t.Parallel(), because
// the cleanup restores color.NoColor and a parallel sibling could
// observe either value. If parallelism is needed, refactor to avoid
// mutating color.NoColor (e.g. wire color decisions exclusively
// through output.Printing).
func newColorPrinting(t *testing.T) *output.Printing {
	t.Helper()
	prev := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = prev })

	pr := output.NewPrinting()
	pr.EnableColor(true)
	return pr
}

func TestTextWriter_Color(t *testing.T) {
	buf := &bytes.Buffer{}
	w := sqlw.NewTextWriter(buf, newColorPrinting(t))

	const sql = `SELECT * FROM "actor" WHERE "first_name" = 'TOM'`

	err := w.Render(output.SQLPayload{SQL: sql})
	require.NoError(t, err)

	got := buf.String()
	// Color output should contain ANSI escapes.
	require.Contains(t, got, "\x1b[", "expected ANSI escape codes in colored output")
	// Stripping ANSI should yield the original SQL + newline.
	require.Equal(t, sql+"\n", stripANSI(got))
}

// stripANSI removes ANSI color escape sequences.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func TestTextWriter_NoColor(t *testing.T) {
	buf := &bytes.Buffer{}
	w := sqlw.NewTextWriter(buf, newMonochromePrinting())

	err := w.Render(output.SQLPayload{
		SLQ:     `.actor`,
		SQL:     `SELECT * FROM "actor"`,
		Dialect: "postgres",
		Sources: output.SQLSources{Target: "@sakila_pg", Inputs: []string{"@sakila_pg"}},
	})
	require.NoError(t, err)
	require.Equal(t, "SELECT * FROM \"actor\"\n", buf.String())
}

// TestTextWriter_Color_StringQuoteSlot verifies the quote-vs-content
// slot routing for chroma's LiteralString family: content (between
// quotes) is colored with pr.String, while the quote characters
// themselves (single, double, or backtick) are dimmed with pr.Faint
// so the user can distinguish a string literal like 'TOM' from a
// double-quoted identifier like "actor" by the dimness of the quote.
func TestTextWriter_Color_StringQuoteSlot(t *testing.T) {
	buf := &bytes.Buffer{}
	pr := newColorPrinting(t)
	w := sqlw.NewTextWriter(buf, pr)

	const sql = `SELECT "first_name" FROM "actor" WHERE "first_name" = 'TOM'`
	require.NoError(t, w.Render(output.SQLPayload{SQL: sql}))

	got := buf.String()
	require.Equal(t, sql+"\n", stripANSI(got), "round-trip text must match")

	// Content tokens use pr.String.
	require.Contains(t, got, pr.String.Sprint("actor"),
		"identifier content should be wrapped in pr.String")
	require.Contains(t, got, pr.String.Sprint("first_name"),
		"identifier content should be wrapped in pr.String")
	require.Contains(t, got, pr.String.Sprint("TOM"),
		"string literal content should be wrapped in pr.String")

	// Quote characters use pr.Faint.
	require.Contains(t, got, pr.Faint.Sprint(`"`),
		"double-quote character should be wrapped in pr.Faint")
	require.Contains(t, got, pr.Faint.Sprint(`'`),
		"single-quote character should be wrapped in pr.Faint")
}

// TestTextWriter_Color_TrueFalseNull verifies that TRUE/FALSE/NULL
// receive their dedicated color slots (Bool/Null) rather than the
// generic Keyword color, matching how the rest of sq colors those
// values.
func TestTextWriter_Color_TrueFalseNull(t *testing.T) {
	buf := &bytes.Buffer{}
	pr := newColorPrinting(t)
	w := sqlw.NewTextWriter(buf, pr)

	const sql = `SELECT TRUE, FALSE, NULL FROM "t"`
	require.NoError(t, w.Render(output.SQLPayload{SQL: sql}))

	got := buf.String()
	// Each value should appear wrapped in its slot's escape sequence.
	for _, val := range []string{"TRUE", "FALSE"} {
		require.Contains(t, got, pr.Bool.Sprint(val),
			"expected %q rendered with pr.Bool", val)
	}
	require.Contains(t, got, pr.Null.Sprint("NULL"),
		"expected NULL rendered with pr.Null")
	// SELECT keyword should NOT use the Bool color.
	require.NotContains(t, got, pr.Bool.Sprint("SELECT"))
}

func samplePayload() output.SQLPayload {
	return output.SQLPayload{
		SLQ:     `.actor | .first_name == "TOM"`,
		SQL:     `SELECT * FROM "actor" WHERE "first_name" = 'TOM'`,
		Dialect: "postgres",
		Sources: output.SQLSources{
			Target: "@sakila_pg",
			Inputs: []string{"@sakila_pg"},
		},
		Args: map[string]string{"name": "TOM"},
	}
}

func TestJSONWriter_Pretty(t *testing.T) {
	buf := &bytes.Buffer{}
	w := sqlw.NewJSONWriter(buf, newMonochromePrinting())
	require.NoError(t, w.Render(samplePayload()))

	// Pretty output spans multiple lines.
	require.Greater(t, strings.Count(buf.String(), "\n"), 3)

	var got output.SQLPayload
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Equal(t, samplePayload(), got)
}

func TestJSONLWriter_SingleLine(t *testing.T) {
	buf := &bytes.Buffer{}
	w := sqlw.NewJSONLWriter(buf, newMonochromePrinting())
	require.NoError(t, w.Render(samplePayload()))

	out := buf.String()
	// JSONL has exactly one trailing newline and no internal newlines.
	require.Equal(t, 1, strings.Count(out, "\n"))
	require.True(t, strings.HasSuffix(out, "\n"))

	var got output.SQLPayload
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &got))
	require.Equal(t, samplePayload(), got)
}

func TestJSONWriter_OmitsEmptyArgs(t *testing.T) {
	buf := &bytes.Buffer{}
	w := sqlw.NewJSONWriter(buf, newMonochromePrinting())

	p := samplePayload()
	p.Args = nil
	require.NoError(t, w.Render(p))

	require.NotContains(t, buf.String(), "args")
}

func TestYAMLWriter_RoundTrip(t *testing.T) {
	buf := &bytes.Buffer{}
	w := sqlw.NewYAMLWriter(buf, newMonochromePrinting())
	require.NoError(t, w.Render(samplePayload()))

	require.True(t, strings.HasSuffix(buf.String(), "\n"))

	var got output.SQLPayload
	require.NoError(t, goccy.Unmarshal(buf.Bytes(), &got))
	require.Equal(t, samplePayload(), got)
}

func TestJSONWriter_Color(t *testing.T) {
	buf := &bytes.Buffer{}
	w := sqlw.NewJSONWriter(buf, newColorPrinting(t))
	require.NoError(t, w.Render(samplePayload()))

	got := buf.String()
	require.Contains(t, got, "\x1b[", "expected ANSI escape codes in colored JSON output")
	// Stripping ANSI should leave valid JSON that round-trips.
	var p output.SQLPayload
	require.NoError(t, json.Unmarshal([]byte(stripANSI(got)), &p))
	require.Equal(t, samplePayload(), p)
}

func TestJSONLWriter_Color(t *testing.T) {
	buf := &bytes.Buffer{}
	w := sqlw.NewJSONLWriter(buf, newColorPrinting(t))
	require.NoError(t, w.Render(samplePayload()))

	got := buf.String()
	require.Contains(t, got, "\x1b[", "expected ANSI escape codes in colored JSONL output")
	// JSONL stays single-line (one trailing newline, no internal newlines)
	// even after colorizing.
	stripped := stripANSI(got)
	require.Equal(t, 1, strings.Count(stripped, "\n"))
	require.True(t, strings.HasSuffix(stripped, "\n"))

	var p output.SQLPayload
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stripped)), &p))
	require.Equal(t, samplePayload(), p)
}

func TestYAMLWriter_Color(t *testing.T) {
	buf := &bytes.Buffer{}
	w := sqlw.NewYAMLWriter(buf, newColorPrinting(t))
	require.NoError(t, w.Render(samplePayload()))

	got := buf.String()
	require.Contains(t, got, "\x1b[", "expected ANSI escape codes in colored YAML output")
	var p output.SQLPayload
	require.NoError(t, goccy.Unmarshal([]byte(stripANSI(got)), &p))
	require.Equal(t, samplePayload(), p)
}

func TestYAMLWriter_OmitsEmptyArgs(t *testing.T) {
	buf := &bytes.Buffer{}
	w := sqlw.NewYAMLWriter(buf, newMonochromePrinting())

	p := samplePayload()
	p.Args = nil
	require.NoError(t, w.Render(p))

	require.NotContains(t, buf.String(), "args")
}
