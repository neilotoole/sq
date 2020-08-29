package json_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/json"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/testsrc"
)

func TestImportJSONL(t *testing.T) {
	openFn := func() (io.ReadCloser, error) {
		return os.Open("testdata/jsonl_actor_nested.jsonl")
	}

	th, src, dbase, _ := testh.NewWith(t, testsrc.EmptyDB)
	err := json.ImportJSONL(th.Context, th.Log, src, openFn, dbase)
	require.NoError(t, err)

	sink, err := th.QuerySQL(src, "SELECT * FROM data")
	require.NoError(t, err)
	require.Equal(t, 4, len(sink.Recs))
}

func TestScanObjectsInArray(t *testing.T) {
	var (
		m1 = []map[string]interface{}{{"a": float64(1)}}
		m2 = []map[string]interface{}{{"a": float64(1)}, {"a": float64(2)}}
		m3 = []map[string]interface{}{{"a": float64(1)}, {"a": float64(2)}, {"a": float64(3)}}
		m4 = []map[string]interface{}{
			{"a": float64(1), "b": []interface{}{float64(1), float64(2), float64(3)}, "c": map[string]interface{}{"c1": float64(1)}, "d": "d1"},
			{"a": float64(2), "b": []interface{}{float64(21), float64(22), float64(23)}, "c": map[string]interface{}{"c1": float64(2)}, "d": "d2"},
		}
	)

	testCases := []struct {
		in         string
		wantObjs   []map[string]interface{}
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
		{in: "[\n  {\"a\" : 1},\n  {\"a\"  : 2 \n}\n,\n  {\"a\":   3}\n]\n\n", wantObjs: m3, wantChunks: []string{"{\"a\" : 1}", "{\"a\"  : 2 \n}", "{\"a\":   3}"}},
		{in: `[{"a":1,"b":[1,2,3],"c":{"c1":1},"d":"d1"}  ,  {"a":2,"b":[21,22,23],"c":{"c1":2},"d":"d2"}]`, wantObjs: m4, wantChunks: []string{`{"a":1,"b":[1,2,3],"c":{"c1":1},"d":"d1"}`, `{"a":2,"b":[21,22,23],"c":{"c1":2},"d":"d2"}`}},
	}

	for i, tc := range testCases {
		tc := tc

		t.Run(testh.Name(i, tc.in), func(t *testing.T) {
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

		t.Run(testh.Name(tc.fname), func(t *testing.T) {
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
