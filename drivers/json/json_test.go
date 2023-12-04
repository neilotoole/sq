package json_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/slogt"

	"github.com/neilotoole/sq/drivers/json"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/tu"
)

func TestDriverDetectorFuncs(t *testing.T) {
	const sampleSize = 1000

	detectFns := map[drivertype.Type]source.DriverDetectFunc{ //nolint:exhaustive
		json.TypeJSON:  json.DetectJSON(sampleSize),
		json.TypeJSONA: json.DetectJSONA(sampleSize),
		json.TypeJSONL: json.DetectJSONL(sampleSize),
	}

	testCases := []struct {
		fn    drivertype.Type
		fname string
		// Note that the zero value is drivertype.None.
		want drivertype.Type
		// If wantScore is zero, it's not inspected. If non-zero,
		// gotScore is tested against wantScore
		wantScore float32
		wantErr   bool
	}{
		// JSON detector
		{fn: json.TypeJSON, fname: "actor.json", want: json.TypeJSON},
		{fn: json.TypeJSON, fname: "film_actor.json", want: json.TypeJSON},
		{fn: json.TypeJSON, fname: "payment.json", want: json.TypeJSON},
		{fn: json.TypeJSON, fname: "address_1_object.json", want: json.TypeJSON},
		{fn: json.TypeJSON, fname: "1_record_on_1_line.jsonl", want: json.TypeJSON, wantScore: 0.9},
		{fn: json.TypeJSON, fname: "1_record_over_n_lines.json", want: json.TypeJSON, wantScore: 1.0},
		{fn: json.TypeJSON, fname: "jsona_bad_1.jsona"},
		{fn: json.TypeJSON, fname: "jsona_good_1.jsona"},
		{fn: json.TypeJSON, fname: "film_actor.jsona"},
		{fn: json.TypeJSON, fname: "payment.jsona"},
		{fn: json.TypeJSON, fname: "actor.jsona"},
		{fn: json.TypeJSON, fname: "actor.jsonl"},
		{fn: json.TypeJSON, fname: "film_actor.jsona"},
		{fn: json.TypeJSON, fname: "film_actor.jsonl"},
		{fn: json.TypeJSON, fname: "payment.jsona"},
		{fn: json.TypeJSON, fname: "payment.jsonl"},
		{fn: json.TypeJSON, fname: "jsonl_good_1.jsonl"},
		{fn: json.TypeJSON, fname: "jsonl_bad_1.jsonl"},

		// JSONA detector
		{fn: json.TypeJSONA, fname: "actor.jsona", want: json.TypeJSONA},
		{fn: json.TypeJSONA, fname: "1_record_on_1_line.jsonl"},
		{fn: json.TypeJSONA, fname: "1_record_over_n_lines.json"},
		{fn: json.TypeJSONA, fname: "jsona_bad_1.jsona"},
		{fn: json.TypeJSONA, fname: "jsona_good_1.jsona", want: json.TypeJSONA},
		{fn: json.TypeJSONA, fname: "film_actor.jsona", want: json.TypeJSONA},
		{fn: json.TypeJSONA, fname: "payment.jsona", want: json.TypeJSONA},
		{fn: json.TypeJSONA, fname: "actor.json"},
		{fn: json.TypeJSONA, fname: "actor.jsonl"},
		{fn: json.TypeJSONA, fname: "film_actor.json"},
		{fn: json.TypeJSONA, fname: "film_actor.jsonl"},
		{fn: json.TypeJSONA, fname: "payment.json"},
		{fn: json.TypeJSONA, fname: "payment.jsonl"},
		{fn: json.TypeJSONA, fname: "jsonl_good_1.jsonl"},
		{fn: json.TypeJSONA, fname: "jsonl_bad_1.jsonl"},
		// JSONL detector
		{fn: json.TypeJSONL, fname: "actor.jsonl", want: json.TypeJSONL},
		{fn: json.TypeJSONL, fname: "jsonl_good_1.jsonl", want: json.TypeJSONL},
		{fn: json.TypeJSONL, fname: "1_record_on_1_line.jsonl", want: json.TypeJSONL},
		{fn: json.TypeJSONL, fname: "1_record_over_n_lines.json"},
		{fn: json.TypeJSONL, fname: "jsonl_bad_1.jsonl"},
		{fn: json.TypeJSONL, fname: "actor.jsona"},
		{fn: json.TypeJSONL, fname: "actor.json"},
		{fn: json.TypeJSONL, fname: "film_actor.jsonl", want: json.TypeJSONL},
		{fn: json.TypeJSONL, fname: "film_actor.jsona"},
		{fn: json.TypeJSONL, fname: "film_actor.json"},
		{fn: json.TypeJSONL, fname: "payment.jsonl", want: json.TypeJSONL},
		{fn: json.TypeJSONL, fname: "payment.jsona"},
		{fn: json.TypeJSONL, fname: "payment.json"},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tu.Name(tc.fn, tc.fname), func(t *testing.T) {
			openFn := func(ctx context.Context) (io.ReadCloser, error) { return os.Open(filepath.Join("testdata", tc.fname)) }
			detectFn := detectFns[tc.fn]

			ctx := lg.NewContext(context.Background(), slogt.New(t))

			gotType, gotScore, gotErr := detectFn(ctx, openFn)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tc.want, gotType)
			if tc.want == drivertype.None {
				require.Equal(t, float32(0), gotScore)
				return
			}

			if tc.wantScore != 0 {
				require.Equal(t, tc.wantScore, gotScore)
			} else {
				require.Equal(t, float32(1.0), gotScore)
			}
		})
	}
}
