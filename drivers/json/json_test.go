package json_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/neilotoole/sq/libsq/files"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/json"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/tu"
)

func TestDriverDetectorFuncs(t *testing.T) {
	const sampleSize = 1000

	detectFns := map[drivertype.Type]files.DriverDetectFunc{ //nolint:exhaustive
		drivertype.TypeJSON:  json.DetectJSON(sampleSize),
		drivertype.TypeJSONA: json.DetectJSONA(sampleSize),
		drivertype.TypeJSONL: json.DetectJSONL(sampleSize),
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
		{fn: drivertype.TypeJSON, fname: "actor.json", want: drivertype.TypeJSON},
		{fn: drivertype.TypeJSON, fname: "film_actor.json", want: drivertype.TypeJSON},
		{fn: drivertype.TypeJSON, fname: "payment.json", want: drivertype.TypeJSON},
		{fn: drivertype.TypeJSON, fname: "address_1_object.json", want: drivertype.TypeJSON},
		{fn: drivertype.TypeJSON, fname: "1_record_on_1_line.jsonl", want: drivertype.TypeJSON, wantScore: 0.9},
		{fn: drivertype.TypeJSON, fname: "1_record_over_n_lines.json", want: drivertype.TypeJSON, wantScore: 1.0},
		{fn: drivertype.TypeJSON, fname: "jsona_bad_1.jsona"},
		{fn: drivertype.TypeJSON, fname: "jsona_good_1.jsona"},
		{fn: drivertype.TypeJSON, fname: "film_actor.jsona"},
		{fn: drivertype.TypeJSON, fname: "payment.jsona"},
		{fn: drivertype.TypeJSON, fname: "actor.jsona"},
		{fn: drivertype.TypeJSON, fname: "actor.jsonl"},
		{fn: drivertype.TypeJSON, fname: "film_actor.jsona"},
		{fn: drivertype.TypeJSON, fname: "film_actor.jsonl"},
		{fn: drivertype.TypeJSON, fname: "payment.jsona"},
		{fn: drivertype.TypeJSON, fname: "payment.jsonl"},
		{fn: drivertype.TypeJSON, fname: "jsonl_good_1.jsonl"},
		{fn: drivertype.TypeJSON, fname: "jsonl_bad_1.jsonl"},

		// JSONA detector
		{fn: drivertype.TypeJSONA, fname: "actor.jsona", want: drivertype.TypeJSONA},
		{fn: drivertype.TypeJSONA, fname: "1_record_on_1_line.jsonl"},
		{fn: drivertype.TypeJSONA, fname: "1_record_over_n_lines.json"},
		{fn: drivertype.TypeJSONA, fname: "jsona_bad_1.jsona"},
		{fn: drivertype.TypeJSONA, fname: "jsona_good_1.jsona", want: drivertype.TypeJSONA},
		{fn: drivertype.TypeJSONA, fname: "film_actor.jsona", want: drivertype.TypeJSONA},
		{fn: drivertype.TypeJSONA, fname: "payment.jsona", want: drivertype.TypeJSONA},
		{fn: drivertype.TypeJSONA, fname: "actor.json"},
		{fn: drivertype.TypeJSONA, fname: "actor.jsonl"},
		{fn: drivertype.TypeJSONA, fname: "film_actor.json"},
		{fn: drivertype.TypeJSONA, fname: "film_actor.jsonl"},
		{fn: drivertype.TypeJSONA, fname: "payment.json"},
		{fn: drivertype.TypeJSONA, fname: "payment.jsonl"},
		{fn: drivertype.TypeJSONA, fname: "jsonl_good_1.jsonl"},
		{fn: drivertype.TypeJSONA, fname: "jsonl_bad_1.jsonl"},
		// JSONL detector
		{fn: drivertype.TypeJSONL, fname: "actor.jsonl", want: drivertype.TypeJSONL},
		{fn: drivertype.TypeJSONL, fname: "jsonl_good_1.jsonl", want: drivertype.TypeJSONL},
		{fn: drivertype.TypeJSONL, fname: "1_record_on_1_line.jsonl", want: drivertype.TypeJSONL},
		{fn: drivertype.TypeJSONL, fname: "1_record_over_n_lines.json"},
		{fn: drivertype.TypeJSONL, fname: "jsonl_bad_1.jsonl"},
		{fn: drivertype.TypeJSONL, fname: "actor.jsona"},
		{fn: drivertype.TypeJSONL, fname: "actor.json"},
		{fn: drivertype.TypeJSONL, fname: "film_actor.jsonl", want: drivertype.TypeJSONL},
		{fn: drivertype.TypeJSONL, fname: "film_actor.jsona"},
		{fn: drivertype.TypeJSONL, fname: "film_actor.json"},
		{fn: drivertype.TypeJSONL, fname: "payment.jsonl", want: drivertype.TypeJSONL},
		{fn: drivertype.TypeJSONL, fname: "payment.jsona"},
		{fn: drivertype.TypeJSONL, fname: "payment.json"},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tu.Name(tc.fn, tc.fname), func(t *testing.T) {
			newRdrFn := func(ctx context.Context) (io.ReadCloser, error) { return os.Open(filepath.Join("testdata", tc.fname)) }
			detectFn := detectFns[tc.fn]

			ctx := lg.NewContext(context.Background(), lgt.New(t))

			gotType, gotScore, gotErr := detectFn(ctx, newRdrFn)
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
