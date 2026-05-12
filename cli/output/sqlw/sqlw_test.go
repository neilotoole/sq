package sqlw_test

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/sqlw"
)

func newMonochromePrinting() *output.Printing {
	pr := output.NewPrinting()
	pr.EnableColor(false)
	return pr
}

func newColorPrinting() *output.Printing {
	pr := output.NewPrinting()
	pr.EnableColor(true)
	return pr
}

func TestTextWriter_Color(t *testing.T) {
	buf := &bytes.Buffer{}
	w := sqlw.NewTextWriter(buf, newColorPrinting())

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
		Source:  "@sakila_pg",
	})
	require.NoError(t, err)
	require.Equal(t, "SELECT * FROM \"actor\"\n", buf.String())
}

// TestTextWriter_Color_TrueFalseNull verifies that TRUE/FALSE/NULL
// receive their dedicated color slots (Bool/Null) rather than the
// generic Keyword color, matching how the rest of sq colors those
// values.
func TestTextWriter_Color_TrueFalseNull(t *testing.T) {
	buf := &bytes.Buffer{}
	pr := newColorPrinting()
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
