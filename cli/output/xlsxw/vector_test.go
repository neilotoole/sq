package xlsxw_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	excelize "github.com/xuri/excelize/v2"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/xlsxw"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/vector"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
)

// TestRecordWriter_Vector verifies that a vector cell holds the full text form,
// or the summary when the text would exceed Excel's cell limit.
func TestRecordWriter_Vector(t *testing.T) {
	small := vector.Vector{
		Elem:   vector.F32,
		Values: []float64{float64(float32(0.9)), float64(float32(-0.12345679))},
	}
	bigValues := make([]float64, 3072)
	for i := range bigValues {
		bigValues[i] = float64(float32(-0.12345679))
	}
	big := vector.Vector{Elem: vector.F32, Values: bigValues}
	require.Greater(t, len(big.String()), 32767)

	testCases := []struct {
		name string
		v    vector.Vector
		want string
	}{
		{name: "small", v: small, want: "[0.9,-0.12345679]"},
		{name: "big", v: big, want: "[3072 × f32]"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			recMeta := testh.NewRecordMeta([]string{"id", "v"}, []kind.Kind{kind.Int, kind.Vector})
			recs := []record.Record{{int64(1), tc.v}}

			buf := &bytes.Buffer{}
			w := xlsxw.NewRecordWriter(buf, output.NewPrinting())
			require.NoError(t, w.Open(ctx, recMeta))
			require.NoError(t, w.WriteRecords(ctx, recs))
			require.NoError(t, w.Close(ctx))

			xl, err := excelize.OpenReader(buf)
			require.NoError(t, err)
			got, err := xl.GetCellValue(source.MonotableName, "B2")
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
