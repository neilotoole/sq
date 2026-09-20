package jsonw_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/testh"
)

// TestRecordWriter_ControlCharEscapes verifies that the JSON record writer
// spells backspace and form feed using the two-character short escapes, the
// same way encoding/json and jsoncolor do. Previously both were emitted as
// six-character numeric escapes, so the record writer disagreed with the
// jsoncolor path used by inspect, error and config output.
func TestRecordWriter_ControlCharEscapes(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name string
		in   string
		want string
	}{
		{name: "backspace", in: "a\bb", want: "\"a\\bb\""},
		{name: "form_feed", in: "a\fb", want: "\"a\\fb\""},
		{name: "tab", in: "a\tb", want: "\"a\\tb\""},
		{name: "newline", in: "a\nb", want: "\"a\\nb\""},
		{name: "carriage_return", in: "a\rb", want: "\"a\\rb\""},
		{name: "other_control_stays_numeric", in: "a\x01b", want: "\"a\\u0001b\""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, enableColor := range []bool{true, false} {
				recMeta := testh.NewRecordMeta([]string{"c"}, []kind.Kind{kind.Text})
				recs := []record.Record{{tc.in}}

				buf := &bytes.Buffer{}
				pr := output.NewPrinting()
				pr.EnableColor(enableColor)
				pr.Compact = true

				w := jsonw.NewStdRecordWriter(buf, pr)
				require.NoError(t, w.Open(ctx, recMeta))
				require.NoError(t, w.WriteRecords(ctx, recs))
				require.NoError(t, w.Close(ctx))

				got := buf.String()
				require.Contains(t, got, tc.want,
					"color=%v: expected %s in output, got %s", enableColor, tc.want, got)

				if !enableColor {
					require.True(t, json.Valid([]byte(got)), "output must be valid JSON")

					var decoded []map[string]string
					require.NoError(t, json.Unmarshal([]byte(got), &decoded))
					require.Equal(t, tc.in, decoded[0]["c"], "output must round-trip")
				}
			}
		})
	}
}
