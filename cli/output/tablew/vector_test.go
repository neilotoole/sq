package tablew_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/tablew"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/vector"
	"github.com/neilotoole/sq/testh"
)

// TestRecordWriter_Vector verifies that a vector value renders as its text
// form (or summary), with f32 elements in shortest f32 form.
func TestRecordWriter_Vector(t *testing.T) {
	ctx := context.Background()
	v := vector.Vector{
		Elem:   vector.F32,
		Values: []float64{float64(float32(0.9)), float64(float32(-0.12345679))},
		Raw:    []byte{0x01, 0x02, 0x03},
	}
	recMeta := testh.NewRecordMeta([]string{"id", "v"}, []kind.Kind{kind.Int, kind.Vector})
	recs := []record.Record{{int64(1), v}}

	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	w := tablew.NewRecordWriter(buf, pr)
	require.NoError(t, w.Open(ctx, recMeta))
	require.NoError(t, w.WriteRecords(ctx, recs))
	require.NoError(t, w.Close(ctx))

	got := buf.String()
	require.Contains(t, got, "[2 × f32]")
	require.NotContains(t, got, "0.9")
}
