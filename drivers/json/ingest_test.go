package json_test

import (
	"bytes"
	"context"
	stdj "encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/json"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/testsrc"
	"github.com/neilotoole/sq/testh/tu"
)

func BenchmarkIngestJSONL_Flat(b *testing.B) {
	// $ go test -count=10 -benchtime=5s -bench BenchmarkIngestJSONL_Flat > old.bench.txt
	// # Make changes
	// $ go test -count=10 -benchtime=5s -bench BenchmarkIngestJSONL_Flat > new.bench.txt
	// $ benchstat old.bench.text new.bench.txt

	// Either fpath (testdata file path) or input should be provided.
	testCases := []struct {
		name      string
		fpath     string
		input     string
		wantRows  int
		wantCols  []string
		wantKinds []kind.Kind
		wantErr   bool
	}{
		{
			name:      "actor",
			fpath:     "actor.jsonl",
			wantRows:  sakila.TblActorCount,
			wantCols:  sakila.TblActorCols(),
			wantKinds: sakila.TblActorColKinds(),
		},
		{
			name:      "film_actor",
			fpath:     "film_actor.jsonl",
			wantRows:  sakila.TblFilmActorCount,
			wantCols:  sakila.TblFilmActorCols(),
			wantKinds: sakila.TblFilmActorColKinds(),
		},
		{
			name:      "actor_nested",
			fpath:     "jsonl_actor_nested.jsonl",
			wantRows:  4,
			wantCols:  []string{"actor_id", "name_first_name", "name_last_name", "last_update"},
			wantKinds: []kind.Kind{kind.Int, kind.Text, kind.Text, kind.Datetime},
		},
		{
			name: "recs_medium",
			input: `{"a": 1, "b": 1, "c": true, "d": "2020-06-11", "e": 2.0}
{"a": 1.0, "b": 1, "c": false, "d": "2020-06-12", "e":2.01}`,
			wantRows:  2,
			wantCols:  []string{"a", "b", "c", "d", "e"},
			wantKinds: []kind.Kind{kind.Int, kind.Int, kind.Bool, kind.Date, kind.Float},
		},
		{
			name: "recs_small",
			input: `{"b": 1}
{"a": 1.1, "b": 2}`,
			wantRows:  2,
			wantCols:  []string{"b", "a"},
			wantKinds: []kind.Kind{kind.Int, kind.Float},
		},
		{
			name: "recs_null",
			input: `{"a": null, "b": null}
{"a": 1.1, "b": 2.0000}`,
			wantRows:  2,
			wantCols:  []string{"a", "b"},
			wantKinds: []kind.Kind{kind.Float, kind.Int},
		},
	}

	for _, tc := range testCases {
		tc := tc

		newRdrFn := func(_ context.Context) (io.ReadCloser, error) { //nolint:unparam
			return io.NopCloser(strings.NewReader(tc.input)), nil
		}

		if tc.fpath != "" {
			newRdrFn = func(_ context.Context) (io.ReadCloser, error) {
				return os.Open(filepath.Join("testdata", tc.fpath))
			}
		}

		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				th := testh.New(b, testh.OptNoLog())
				src := th.Source(testsrc.EmptyDB)
				grip := th.Open(src)

				job := json.NewIngestJob(src, newRdrFn, grip, 0, true)

				b.StartTimer()
				err := json.IngestJSONL(th.Context, job)
				if tc.wantErr {
					require.Error(b, err)
					return
				}
			}
		})
	}
}

func TestIngestJSONL_Flat(t *testing.T) {
	t.Parallel()

	// Either fpath (testdata file path) or input should be provided.
	testCases := []struct {
		name      string
		fpath     string
		input     string
		wantRows  int
		wantCols  []string
		wantKinds []kind.Kind
		wantErr   bool
	}{
		{
			name:      "actor",
			fpath:     "actor.jsonl",
			wantRows:  sakila.TblActorCount,
			wantCols:  sakila.TblActorCols(),
			wantKinds: sakila.TblActorColKinds(),
		},
		{
			name:      "film_actor",
			fpath:     "film_actor.jsonl",
			wantRows:  sakila.TblFilmActorCount,
			wantCols:  sakila.TblFilmActorCols(),
			wantKinds: sakila.TblFilmActorColKinds(),
		},
		{
			name:      "actor_nested",
			fpath:     "jsonl_actor_nested.jsonl",
			wantRows:  4,
			wantCols:  []string{"actor_id", "name_first_name", "name_last_name", "last_update"},
			wantKinds: []kind.Kind{kind.Int, kind.Text, kind.Text, kind.Datetime},
		},
		{
			name: "recs_medium",
			input: `{"a": 1, "b": 1, "c": true, "d": "2020-06-11", "e": 2.0}
{"a": 1.0, "b": 1, "c": false, "d": "2020-06-12", "e":2.01}`,
			wantRows:  2,
			wantCols:  []string{"a", "b", "c", "d", "e"},
			wantKinds: []kind.Kind{kind.Int, kind.Int, kind.Bool, kind.Date, kind.Float},
		},
		{
			name: "recs_small",
			input: `{"b": 1}
{"a": 1.1, "b": 2}`,
			wantRows:  2,
			wantCols:  []string{"b", "a"},
			wantKinds: []kind.Kind{kind.Int, kind.Float},
		},
		{
			name: "recs_null",
			input: `{"a": null, "b": null}
{"a": 1.1, "b": 2.0000}`,
			wantRows:  2,
			wantCols:  []string{"a", "b"},
			wantKinds: []kind.Kind{kind.Float, kind.Int},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			newRdrFn := func(_ context.Context) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(tc.input)), nil
			}

			if tc.fpath != "" {
				newRdrFn = func(_ context.Context) (io.ReadCloser, error) {
					return os.Open(filepath.Join("testdata", tc.fpath))
				}
			}

			th, src, _, grip, _ := testh.NewWith(t, testsrc.EmptyDB)
			job := json.NewIngestJob(src, newRdrFn, grip, 0, true)

			err := json.IngestJSONL(th.Context, job)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			sink, err := th.QuerySQL(src, nil, "SELECT * FROM data")
			require.NoError(t, err)
			require.Equal(t, tc.wantRows, len(sink.Recs))
			require.Equal(t, tc.wantCols, sink.RecMeta.Names())
			require.Equal(t, tc.wantCols, sink.RecMeta.MungedNames())
			require.Equal(t, tc.wantKinds, sink.RecMeta.Kinds())
		})
	}
}

func TestIngestJSON_Flat(t *testing.T) {
	t.Parallel()

	newRdrFn := func(context.Context) (io.ReadCloser, error) {
		return os.Open("testdata/actor.json")
	}

	th, src, _, grip, _ := testh.NewWith(t, testsrc.EmptyDB)
	job := json.NewIngestJob(src, newRdrFn, grip, 0, true)

	err := json.IngestJSON(th.Context, job)
	require.NoError(t, err)

	sink, err := th.QuerySQL(src, nil, "SELECT * FROM data")
	require.NoError(t, err)
	require.Equal(t, sakila.TblActorCount, len(sink.Recs))
}

func TestScanObjectsInArray(t *testing.T) {
	t.Parallel()

	var (
		m1 = []map[string]any{{"a": float64(1)}}
		m2 = []map[string]any{{"a": float64(1)}, {"a": float64(2)}}
		m3 = []map[string]any{{"a": float64(1)}, {"a": float64(2)}, {"a": float64(3)}}
		m4 = []map[string]any{
			{
				"a": float64(1), "b": []any{float64(1), float64(2), float64(3)}, "c": map[string]any{"c1": float64(1)},
				"d": "d1",
			},
			{
				"a": float64(2), "b": []any{float64(21), float64(22), float64(23)}, "c": map[string]any{"c1": float64(2)},
				"d": "d2",
			},
		}
	)

	testCases := []struct {
		in         string
		wantObjs   []map[string]any
		wantChunks []string
		wantErr    bool
	}{
		{in: ``, wantErr: true},
		{in: `[,]`, wantErr: true},
		{in: `[],`, wantErr: true},
		{in: `[]  ,`, wantErr: true},
		{in: `,[]`, wantErr: true},
		{in: `[]`},
		{in: ` []`},
		{in: ` [  ] `},
		{in: `[{"a":1}]`, wantObjs: m1, wantChunks: []string{`{"a":1}`}},
		{in: `{[{"a":1}]}`, wantErr: true},
		{in: `[,{"a":1}]`, wantErr: true},
		{in: `[{"a":1},]`, wantErr: true},
		{in: ` [{"a":1}]`, wantObjs: m1, wantChunks: []string{`{"a":1}`}},
		{in: `[ {"a":1} ]`, wantObjs: m1, wantChunks: []string{`{"a":1}`}},
		{in: `[  { "a" :  1   }  ]`, wantObjs: m1, wantChunks: []string{`{ "a" :  1   }`}},
		{in: `[{"a":1},{"a":2}]`, wantObjs: m2, wantChunks: []string{`{"a":1}`, `{"a":2}`}},
		{in: `[,{"a":1},{"a":2}]`, wantErr: true},
		{in: `[{"a":1},,{"a":2}]`, wantErr: true},
		{in: `  [{"a":1},{"a":2}]`, wantObjs: m2, wantChunks: []string{`{"a":1}`, `{"a":2}`}},
		{in: `[{"a":1}, {"a":2}]`, wantObjs: m2, wantChunks: []string{`{"a":1}`, `{"a":2}`}},
		{in: `[{"a":1} ,{"a":2}]`, wantObjs: m2, wantChunks: []string{`{"a":1}`, `{"a":2}`}},
		{in: `[{"a":1} , {"a":2}]`, wantObjs: m2, wantChunks: []string{`{"a":1}`, `{"a":2}`}},
		{in: `[{"a":1} , {"a":2} ]`, wantObjs: m2, wantChunks: []string{`{"a":1}`, `{"a":2}`}},
		{in: `[ {"a":1} , {"a":2} ]`, wantObjs: m2, wantChunks: []string{`{"a":1}`, `{"a":2}`}},
		{in: `[  { "a"  : 1} ,   {"a":  2 }  ]`, wantObjs: m2, wantChunks: []string{`{ "a"  : 1}`, `{"a":  2 }`}},
		{in: `[{"a":1},{"a":2},{"a":3}]`, wantObjs: m3, wantChunks: []string{`{"a":1}`, `{"a":2}`, `{"a":3}`}},
		{in: `[{"a":1} ,{"a":2},{"a":3}]`, wantObjs: m3, wantChunks: []string{`{"a":1}`, `{"a":2}`, `{"a":3}`}},
		{
			in: "[\n  {\"a\" : 1},\n  {\"a\"  : 2 \n}\n,\n  {\"a\":   3}\n]\n\n", wantObjs: m3,
			wantChunks: []string{"{\"a\" : 1}", "{\"a\"  : 2 \n}", "{\"a\":   3}"},
		},
		{
			in:       `[{"a":1,"b":[1,2,3],"c":{"c1":1},"d":"d1"}  ,  {"a":2,"b":[21,22,23],"c":{"c1":2},"d":"d2"}]`,
			wantObjs: m4, wantChunks: []string{
				`{"a":1,"b":[1,2,3],"c":{"c1":1},"d":"d1"}`,
				`{"a":2,"b":[21,22,23],"c":{"c1":2},"d":"d2"}`,
			},
		},
	}

	for i, tc := range testCases {
		tc := tc

		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			t.Parallel()

			r := bytes.NewReader([]byte(tc.in))
			gotObjs, gotChunks, err := json.ScanObjectsInArray(r)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.EqualValues(t, tc.wantObjs, gotObjs)
			require.Equal(t, len(tc.wantObjs), len(gotChunks))
			require.Equal(t, len(tc.wantChunks), len(gotChunks))
			for j := range tc.wantChunks {
				require.Equal(t, tc.wantChunks[j], string(gotChunks[j]))
			}
		})
	}
}

func TestScanObjectsInArray_Files(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		fname     string
		wantCount int
	}{
		{fname: "testdata/actor.json", wantCount: sakila.TblActorCount},
		{fname: "testdata/film_actor.json", wantCount: sakila.TblFilmActorCount},
		{fname: "testdata/payment.json", wantCount: sakila.TblPaymentCount},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tu.Name(tc.fname), func(t *testing.T) {
			t.Parallel()

			f, err := os.Open(tc.fname)
			require.NoError(t, err)
			defer f.Close()

			gotObjs, gotChunks, err := json.ScanObjectsInArray(f)
			require.NoError(t, err)
			require.Equal(t, tc.wantCount, len(gotObjs))
			require.Equal(t, tc.wantCount, len(gotChunks))
		})
	}
}

func TestColumnOrderFlat(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		in      string
		want    []string
		wantErr bool
	}{
		{in: `{}`, want: nil},
		{in: `{"a":1}`, want: []string{"a"}},
		{in: `{"a":1, "b": {"c":2}}`, want: []string{"a", "b_c"}},
		{in: `{"a":1, "b": {"c":2}, "d":3}`, want: []string{"a", "b_c", "d"}},
		{in: `{"a":1, "b": {"c":2, "d":3}}`, want: []string{"a", "b_c", "b_d"}},
		{in: `{"a":1, "b": {"c":2}, "d":3, "e":4}`, want: []string{"a", "b_c", "d", "e"}},
		{in: `{"a":1, "b": {"c":2}, "d": [3,4], "e":5}`, want: []string{"a", "b_c", "d", "e"}},
		{in: `{"d": [3,4], "e":5}`, want: []string{"d", "e"}},
		{in: `{"d": [3], "e":5}`, want: []string{"d", "e"}},
		{in: `{"d": [3,[4,5]], "e":6}`, want: []string{"d", "e"}},
		{in: `{"d": [3,[4,5,[6,7,8]]], "e":9, "fname":[10,11,[12,13]]}`, want: []string{"d", "e", "fname"}},
		{in: `{"a":1, "b": {"c":2}, "d": 3, "e":4}`, want: []string{"a", "b_c", "d", "e"}},
		{in: `{"b":1,"a":2}`, want: []string{"b", "a"}},
		{
			in:   `{"a":1,"b":2,"c":{"c1":3,"c2":4,"c3":{"d1":5,"d2":6},"c5":7},"e":8}`,
			want: []string{"a", "b", "c_c1", "c_c2", "c_c3_d1", "c_c3_d2", "c_c5", "e"},
		},
	}

	for i, tc := range testCases {
		tc := tc

		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			t.Parallel()

			require.True(t, stdj.Valid([]byte(tc.in)))

			gotCols, err := json.ColumnOrderFlat([]byte(tc.in))
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, gotCols)
		})
	}
}
