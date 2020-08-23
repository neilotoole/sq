package json_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/neilotoole/lg/testlg"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/json"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
)

func TestTypeDetectorFuncs(t *testing.T) {
	detectFns := map[source.Type]source.TypeDetectFunc{
		json.TypeJSON:  json.DetectJSON,
		json.TypeJSONA: json.DetectJSONA,
		json.TypeJSONL: json.DetectJSONL,
	}

	testCases := []struct {
		fn      source.Type
		f       string
		want    source.Type // Note: that the zero value is source.TypeNone
		wantErr bool
	}{
		{fn: json.TypeJSONA, f: "actor.jsona", want: json.TypeJSONA},
		{fn: json.TypeJSONA, f: "jsona_bad_1.jsona"},
		{fn: json.TypeJSONA, f: "jsona_good_1.jsona", want: json.TypeJSONA},
		{fn: json.TypeJSONA, f: "actor.json"},
		{fn: json.TypeJSONA, f: "actor.jsonl"},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(testh.TName(tc.fn, tc.f), func(t *testing.T) {
			openFn := func() (io.ReadCloser, error) { return os.Open(filepath.Join("testdata", tc.f)) }
			detectFn := detectFns[tc.fn]

			gotType, gotScore, gotErr := detectFn(context.Background(), testlg.New(t), openFn)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tc.want, gotType)
			if tc.want == source.TypeNone {
				require.Equal(t, float32(0), gotScore)
			} else {
				require.Equal(t, float32(1.0), gotScore)
			}
		})
	}
}

//func TestDecode(t *testing.T) {
//	const jsona = `[1,2,3,4,"hello",true,null,7.7, {"id": 3}]`
//	const jsono = `{"a": 1, "b": 2}`
//
//	var s []interface{}
//
//	err := stdj.Unmarshal([]byte(jsono), &s)
//	require.NoError(t, err)
//
//	fmt.Printf("%#v\n", s)
//
//}
