package sqlw_test

import (
	"bytes"
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
