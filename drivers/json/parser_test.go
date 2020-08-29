package json_test

import (
	"bytes"
	"context"
	stdj "encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/json"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestStdjDecoder(t *testing.T) {
	f, err := os.Open("testdata/jsonl_actor_nested.jsonl")
	require.NoError(t, err)
	defer f.Close()

	dec := stdj.NewDecoder(f)
	dec.UseNumber()

	var tok stdj.Token

	for dec.More() {
		tok, err = dec.Token()
		require.NoError(t, err)
		t.Logf("%#v", tok)
	}
}

// objectScanner scans for JSON objects, returning the raw
// bytes
type objectScanner struct {
	ctx context.Context
	dec *stdj.Decoder
}

func newObjectScanner(ctx context.Context, r io.Reader) *objectScanner {
	return &objectScanner{
		ctx: ctx,
		dec: stdj.NewDecoder(r),
	}
}

func (sc *objectScanner) Next() (hasMore bool, raw []byte, err error) {
	var t stdj.Token

	if sc.dec.InputOffset() == 0 {
		t, err = sc.dec.Token()
		if err != nil {
			return false, nil, errz.Err(err)
		}

		if t != stdj.Delim('[') {
			return false, nil, errz.Errorf("expected [ delim")
		}
	}

	return false, nil, nil

	//for {
	//	t, err =
	//}
}

func TestParseObjects(t *testing.T) {
	f, err := os.Open("testdata/actor_small.json")
	require.NoError(t, err)
	defer f.Close()

	objs, raw, err := json.ParseObjectsInArray(f)
	require.NoError(t, err)
	require.Equal(t, 3, len(objs))
	require.Equal(t, 3, len(raw))
}

func TestParseObjects2(t *testing.T) {
	f, err := os.Open("testdata/payment.json")
	require.NoError(t, err)
	defer f.Close()

	objs, raw, err := json.ParseObjectsInArray(f)
	require.NoError(t, err)
	require.Equal(t, sakila.TblPaymentCount, len(objs))
	require.Equal(t, sakila.TblPaymentCount, len(raw))
}

func TestParseObjects3(t *testing.T) {
	var (
		m1 = []map[string]interface{}{{"a": float64(1)}}
		m2 = []map[string]interface{}{{"a": float64(1)}, {"a": float64(2)}}
		m3 = []map[string]interface{}{{"a": float64(1)}, {"a": float64(2)}, {"a": float64(3)}}
		m4 = []map[string]interface{}{
			{"a": float64(1), "b": []interface{}{float64(1), float64(2), float64(3)}, "c": map[string]interface{}{"c1": float64(1)}, "d": "d1"},
			{"a": float64(2), "b": []interface{}{float64(21), float64(22), float64(23)}, "c": map[string]interface{}{"c1": float64(2)}, "d": "d2"},
		}
	)

	_, _, _, _ = m1, m2, m3, m4

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
			println("input:>>>" + tc.in + "<<<")

			r := bytes.NewReader([]byte(tc.in))
			gotObjs, gotChunks, err := json.ParseObjectsInArray(r)
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
