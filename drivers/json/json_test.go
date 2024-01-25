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
		drivertype.JSON:  json.DetectJSON(sampleSize),
		drivertype.JSONA: json.DetectJSONA(sampleSize),
		drivertype.JSONL: json.DetectJSONL(sampleSize),
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
		{fn: drivertype.JSON, fname: "actor.json", want: drivertype.JSON},
		{fn: drivertype.JSON, fname: "film_actor.json", want: drivertype.JSON},
		{fn: drivertype.JSON, fname: "payment.json", want: drivertype.JSON},
		{fn: drivertype.JSON, fname: "address_1_object.json", want: drivertype.JSON},
		{fn: drivertype.JSON, fname: "1_record_on_1_line.jsonl", want: drivertype.JSON, wantScore: 0.9},
		{fn: drivertype.JSON, fname: "1_record_over_n_lines.json", want: drivertype.JSON, wantScore: 1.0},
		{fn: drivertype.JSON, fname: "jsona_bad_1.jsona"},
		{fn: drivertype.JSON, fname: "jsona_good_1.jsona"},
		{fn: drivertype.JSON, fname: "film_actor.jsona"},
		{fn: drivertype.JSON, fname: "payment.jsona"},
		{fn: drivertype.JSON, fname: "actor.jsona"},
		{fn: drivertype.JSON, fname: "actor.jsonl"},
		{fn: drivertype.JSON, fname: "film_actor.jsona"},
		{fn: drivertype.JSON, fname: "film_actor.jsonl"},
		{fn: drivertype.JSON, fname: "payment.jsona"},
		{fn: drivertype.JSON, fname: "payment.jsonl"},
		{fn: drivertype.JSON, fname: "jsonl_good_1.jsonl"},
		{fn: drivertype.JSON, fname: "jsonl_bad_1.jsonl"},

		// JSONA detector
		{fn: drivertype.JSONA, fname: "actor.jsona", want: drivertype.JSONA},
		{fn: drivertype.JSONA, fname: "1_record_on_1_line.jsonl"},
		{fn: drivertype.JSONA, fname: "1_record_over_n_lines.json"},
		{fn: drivertype.JSONA, fname: "jsona_bad_1.jsona"},
		{fn: drivertype.JSONA, fname: "jsona_good_1.jsona", want: drivertype.JSONA},
		{fn: drivertype.JSONA, fname: "film_actor.jsona", want: drivertype.JSONA},
		{fn: drivertype.JSONA, fname: "payment.jsona", want: drivertype.JSONA},
		{fn: drivertype.JSONA, fname: "actor.json"},
		{fn: drivertype.JSONA, fname: "actor.jsonl"},
		{fn: drivertype.JSONA, fname: "film_actor.json"},
		{fn: drivertype.JSONA, fname: "film_actor.jsonl"},
		{fn: drivertype.JSONA, fname: "payment.json"},
		{fn: drivertype.JSONA, fname: "payment.jsonl"},
		{fn: drivertype.JSONA, fname: "jsonl_good_1.jsonl"},
		{fn: drivertype.JSONA, fname: "jsonl_bad_1.jsonl"},
		// JSONL detector
		{fn: drivertype.JSONL, fname: "actor.jsonl", want: drivertype.JSONL},
		{fn: drivertype.JSONL, fname: "jsonl_good_1.jsonl", want: drivertype.JSONL},
		{fn: drivertype.JSONL, fname: "1_record_on_1_line.jsonl", want: drivertype.JSONL},
		{fn: drivertype.JSONL, fname: "1_record_over_n_lines.json"},
		{fn: drivertype.JSONL, fname: "jsonl_bad_1.jsonl"},
		{fn: drivertype.JSONL, fname: "actor.jsona"},
		{fn: drivertype.JSONL, fname: "actor.json"},
		{fn: drivertype.JSONL, fname: "film_actor.jsonl", want: drivertype.JSONL},
		{fn: drivertype.JSONL, fname: "film_actor.jsona"},
		{fn: drivertype.JSONL, fname: "film_actor.json"},
		{fn: drivertype.JSONL, fname: "payment.jsonl", want: drivertype.JSONL},
		{fn: drivertype.JSONL, fname: "payment.jsona"},
		{fn: drivertype.JSONL, fname: "payment.json"},
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
