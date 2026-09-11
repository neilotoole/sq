package jsonw_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw"
	"github.com/neilotoole/sq/cli/output/jsonw/internal"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/testh"
)

// TestRecordWriter_BytesColor verifies the JSON record writer encodes a []byte
// value as a base64 string in both the colored and monochrome paths. The
// monochrome path is also covered by TestRecordWriters; this test asserts the
// colored path explicitly (the Bytes color wraps the base64 string), mirroring
// the YAML writer's coverage.
func TestRecordWriter_BytesColor(t *testing.T) {
	ctx := context.Background()
	hi := []byte("hi")
	wantB64 := `"` + base64.StdEncoding.EncodeToString(hi) + `"` // "aGk="

	render := func(t *testing.T, enableColor bool) string {
		t.Helper()
		recMeta := testh.NewRecordMeta([]string{"c"}, []kind.Kind{kind.Bytes})
		recs := []record.Record{{hi}}

		buf := &bytes.Buffer{}
		pr := output.NewPrinting()
		pr.EnableColor(enableColor)
		pr.Compact = true

		w := jsonw.NewStdRecordWriter(buf, pr)
		require.NoError(t, w.Open(ctx, recMeta))
		require.NoError(t, w.WriteRecords(ctx, recs))
		require.NoError(t, w.Close(ctx))
		return buf.String()
	}

	t.Run("color", func(t *testing.T) {
		pr := output.NewPrinting()
		pr.EnableColor(true)
		clrs := internal.NewColors(pr)
		wantColored := string(clrs.Bytes.Prefix) + wantB64 + string(clrs.Bytes.Suffix)

		got := render(t, true)
		require.Contains(t, got, wantColored)
	})

	t.Run("monochrome", func(t *testing.T) {
		got := render(t, false)
		require.Contains(t, got, `"c":`+wantB64)
		require.NotContains(t, got, "\x1b")
		require.True(t, json.Valid([]byte(got)))
	})
}
