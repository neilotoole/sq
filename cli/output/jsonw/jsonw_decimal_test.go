package jsonw_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/testh"
)

func TestRecordWriter_DecimalAsNumber(t *testing.T) {
	testCases := []struct {
		name            string
		decimalAsNumber bool
		want            string
	}{
		{name: "string_mode_default", decimalAsNumber: false, want: `[{"amount":"100.5"}]` + "\n"},
		{name: "number_mode", decimalAsNumber: true, want: `[{"amount":100.5}]` + "\n"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			recMeta := testh.NewRecordMeta([]string{"amount"}, []kind.Kind{kind.Decimal})
			recs := []record.Record{{decimal.RequireFromString("100.5")}}

			buf := &bytes.Buffer{}
			pr := output.NewPrinting()
			pr.EnableColor(false)
			pr.Compact = true
			pr.DecimalAsNumber = tc.decimalAsNumber

			w := jsonw.NewStdRecordWriter(buf, pr)
			require.NoError(t, w.Open(ctx, recMeta))
			require.NoError(t, w.WriteRecords(ctx, recs))
			require.NoError(t, w.Close(ctx))
			require.Equal(t, tc.want, buf.String())
		})
	}
}
