package json_test

import (
	"bytes"
	stdj "encoding/json"
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

func TestColumnOrderFlat(t *testing.T) {
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
		{in: `{"d": [3,[4,5,[6,7,8]]], "e":9, "f":[10,11,[12,13]]}`, want: []string{"d", "e", "f"}},
		{in: `{"a":1, "b": {"c":2}, "d": 3, "e":4}`, want: []string{"a", "b_c", "d", "e"}},
		{in: `{"b":1,"a":2}`, want: []string{"b", "a"}},
		{in: `{"a":1,"b":2,"c":{"c1":3,"c2":4,"c3":{"d1":5,"d2":6},"c5":7},"e":8}`, want: []string{"a", "b", "c_c1", "c_c2", "c_c3_d1", "c_c3_d2", "c_c5", "e"}},
	}

	for i, tc := range testCases {
		tc := tc

		t.Run(testh.Name(i, tc.in), func(t *testing.T) {
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
