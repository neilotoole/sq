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
