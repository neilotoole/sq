package json_test

import (
	"bytes"
	"context"
	stdj "encoding/json"
	"fmt"
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
	type mObj = map[string]interface{}

	var (
		m1 = []map[string]interface{}{{"a": float64(1)}}
		m2 = []map[string]interface{}{{"a": float64(1)}, {"a": float64(2)}}
		m3 = []map[string]interface{}{{"a": float64(1)}, {"a": float64(2)}, {"a": float64(3)}}
		//m3 = []map[string]interface{}{{"a": 1}, {"a": 2}, {"a": 3}}
	)

	_, _ = m1, m2

	testCases := []struct {
		in       string
		wantObjs []map[string]interface{}
		// wantChunks is optional; if nil, test len(gotChunks) == len(wantObjs)
		wantChunks []string
		wantErr    bool
	}{
		//{in: ``, wantErr: true},
		//{in: `[]`},
		{in: `[{"a":1}]`, wantObjs: m1, wantChunks: []string{`{"a":1}`}},
		//{in: `[ {"a":1} ]`, wantObjs: m1, wantChunks: []string{`{"a":1}`}},
		//{in: `[  { "a" :  1   }  ]`, wantObjs: m1, wantChunks: []string{`{ "a" :  1   }`}},
		//{in: `[{"a":1},{"a":2}]`, wantObjs: m2, wantChunks: []string{`{"a":1}`, `{"a":2}`}},
		{in: `[{"a":1},{"a":2},{"a":3}]`, wantObjs: m3, wantChunks: []string{`{"a":1}`, `{"a":2}`, `{"a":2}`}},
		//{in: `[{"a":1},{"a":2},{"a":3}]`, want: 3},
		//{in: "[\n  {\"a\": 1},\n  {\"a\": 2},\n  {\"a\": 3}\n]", want: 3},
		//{in: "[  {\"a\": 1},  {\"a\": 2},\n  {\"a\": 3}\n]", want: 3},
	}

	for i, tc := range testCases {
		tc := tc

		t.Run(testh.Name(i, tc.in), func(t *testing.T) {
			r := bytes.NewReader([]byte(tc.in))
			gotObjs, gotChunks, err := json.ParseObjectsInArray(r)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			if err != nil {
				require.NoError(t, err)
			}

			require.EqualValues(t, tc.wantObjs, gotObjs)
			require.Equal(t, len(tc.wantObjs), len(gotChunks))
			if tc.wantChunks != nil {
				require.Equal(t, len(tc.wantChunks), len(gotChunks))
				for j := range tc.wantChunks {
					require.Equal(t, tc.wantChunks[j], string(gotChunks[j]))
				}
			}
		})
	}
}

//func TestStdjDecode(t *testing.T) {
//	f, err := os.Open("testdata/actor_small.json")
//	require.NoError(t, err)
//	defer f.Close()
//
//	buf := &bytes.Buffer{}
//	r := io.TeeReader(f, buf)
//	dec := stdj.NewDecoder(r)
//
//	var tok stdj.Token
//	var startOffset, endOffset int64
//
//	tok, err = dec.Token()
//	require.Equal(t, stdj.Delim('['), tok)
//	startOffset = dec.InputOffset()
//	t.Logf("startOffset: %d", startOffset)
//
//	var m map[string]interface{}
//	var objCount int
//	more := true
//
//	for {
//		more = dec.More()
//
//		if !more {
//			break
//		}
//
//		err = dec.Decode(&m)
//		require.NoError(t, err)
//		q.Q(m)
//		var decBuf []byte
//		decBuf, err = ioutil.ReadAll(dec.Buffered())
//		require.NoError(t, err)
//
//		// If there's another object, delim should be comma,
//		// otherwise delim should be right bracket.
//		delimIndex, delim := json.NextDelim(decBuf, 0)
//		require.False(t, delimIndex == -1)
//		switch delim {
//		case ',':
//		// more objects to come
//		case ']':
//			// end of input
//			tok, err = dec.Token()
//			require.NoError(t, err)
//			require.Equal(t, stdj.Delim(']'), tok)
//			more = dec.More()
//			require.False(t, more)
//			//require.False(t, dec.More())
//			//return
//		default:
//			// bad input
//			require.FailNow(t, "invalid JSON: expected comma or right bracket but got %s", string(delim))
//		}
//
//		_ = decBuf
//
//		endOffset = dec.InputOffset()
//		objSize := endOffset - startOffset
//		t.Logf("obj[%d]: byte size: %d", objCount, objSize)
//
//		b := buf.Bytes()
//		b2 := make([]byte, objSize)
//		copy(b2, b[startOffset:endOffset])
//		startOffset = endOffset
//		objCount++
//
//		t.Logf("line[%d]:\n\n=====\n%s\n=====\n", objCount, string(b2))
//	}
//
//	//require.
//	//
//	//
//	//for {
//	//	tok, err
//	//}
//	//
//	//for dec.More() {
//	//	tok, err = dec.Token()
//	//	require.NoError(t, err)
//	//	t.Logf("%#v", tok)
//	//}
//}

func TestNextDelim(t *testing.T) {
	var data = []byte(`[
  {"actor_id": 1},
  {"actor_id": 2},
  {"actor_id": 3},
  {"actor_id": 4}
]`)

	var i int
	var delim byte

	for {
		i, delim = json.NextDelim(data, i)
		if i == -1 {
			break
		}

		t.Logf("%d -->  %v", i, string(delim))

		if i == len(data)-1 {
			break
		}
		i++
	}

}

//// firstDelim returns the index in b of the first JSON
//// delimiter (left or right bracket, left or right brace, or comma)
//// occurring from index start onwards. If no delimiter found,
//// (-1, 0) is returned.
//func NextDelim(b []byte, start int) (i int, delim byte) {
//	for i = start; i < len(b); i++ {
//		switch b[i] {
//		case ',', '{', '}', '[', ']':
//			return i, b[i]
//		}
//	}
//
//	return -1, 0
//}

func TestStdjDecoder2(t *testing.T) {
	f, err := os.Open("testdata/jsonl_actor_nested.jsonl")
	require.NoError(t, err)
	defer f.Close()

	dec := stdj.NewDecoder(f)
	dec.UseNumber()

	var m map[string]interface{}

	for {
		err = dec.Decode(&m)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		fmt.Println(m)
	}

	//var tok stdj.Token
	//
	//for dec.More() {
	//	tok, err = dec.Token()
	//	require.NoError(t, err)
	//	t.Logf("%#v", tok)
	//}

}

func TestStdjDecoder3(t *testing.T) {
	f, err := os.Open("testdata/actor.json")
	require.NoError(t, err)
	defer f.Close()

	dec := stdj.NewDecoder(f)
	dec.UseNumber()

	var rawMsgs []stdj.RawMessage

	for {
		err = dec.Decode(&rawMsgs)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
	}

	for i, raw := range rawMsgs {
		fmt.Println(i, string(raw))
	}

	//var tok stdj.Token
	//
	//for dec.More() {
	//	tok, err = dec.Token()
	//	require.NoError(t, err)
	//	t.Logf("%#v", tok)
	//}

}
