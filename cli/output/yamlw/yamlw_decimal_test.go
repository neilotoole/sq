package yamlw_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/yamlw"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/testh"
)

func TestRecordWriter_DecimalAsNumber(t *testing.T) {
	t.Run("string_mode_default", func(t *testing.T) {
		got := renderDecimalYAML(t, false)
		require.Contains(t, got, `amount: "100.5"`)
	})

	t.Run("number_mode", func(t *testing.T) {
		got := renderDecimalYAML(t, true)
		require.Contains(t, got, "amount: 100.5")
		require.NotContains(t, got, `"100.5"`)
	})
}

func renderDecimalYAML(t *testing.T, asNumber bool) string {
	t.Helper()
	ctx := context.Background()
	recMeta := testh.NewRecordMeta([]string{"amount"}, []kind.Kind{kind.Decimal})
	recs := []record.Record{{decimal.RequireFromString("100.5")}}

	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	pr.DecimalAsNumber = asNumber

	w := yamlw.NewRecordWriter(buf, pr)
	require.NoError(t, w.Open(ctx, recMeta))
	require.NoError(t, w.WriteRecords(ctx, recs))
	require.NoError(t, w.Flush(ctx))
	require.NoError(t, w.Close(ctx))
	return buf.String()
}
