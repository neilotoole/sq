//nolint:lll
package jsonw_test

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/neilotoole/slogt"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
)

func TestRecordWriters(t *testing.T) {
	const (
		wantStdJSONNoPretty = `[{"col_int":64,"col_float":64.64,"col_decimal":"10000000000000000.99","col_bool":true,"col_text":"hello","col_datetime":"1970-01-01T00:00:00Z","col_date":"1970-01-01","col_time":"00:00:00","col_bytes":"aGVsbG8="},{"col_int":null,"col_float":null,"col_decimal":null,"col_bool":null,"col_text":null,"col_datetime":null,"col_date":null,"col_time":null,"col_bytes":null},{"col_int":64,"col_float":64.64,"col_decimal":"10000000000000000.99","col_bool":true,"col_text":"hello","col_datetime":"1970-01-01T00:00:00Z","col_date":"1970-01-01","col_time":"00:00:00","col_bytes":"aGVsbG8="}]
`
		wantStdJSONPretty = `[
  {
    "col_int": 64,
    "col_float": 64.64,
    "col_decimal": "10000000000000000.99",
    "col_bool": true,
    "col_text": "hello",
    "col_datetime": "1970-01-01T00:00:00Z",
    "col_date": "1970-01-01",
    "col_time": "00:00:00",
    "col_bytes": "aGVsbG8="
  },
  {
    "col_int": null,
    "col_float": null,
    "col_decimal": null,
    "col_bool": null,
    "col_text": null,
    "col_datetime": null,
    "col_date": null,
    "col_time": null,
    "col_bytes": null
  },
  {
    "col_int": 64,
    "col_float": 64.64,
    "col_decimal": "10000000000000000.99",
    "col_bool": true,
    "col_text": "hello",
    "col_datetime": "1970-01-01T00:00:00Z",
    "col_date": "1970-01-01",
    "col_time": "00:00:00",
    "col_bytes": "aGVsbG8="
  }
]
`
		wantArrayNoPretty = `[64,64.64,"10000000000000000.99",true,"hello","1970-01-01T00:00:00Z","1970-01-01","00:00:00","aGVsbG8="]
[null,null,null,null,null,null,null,null,null]
[64,64.64,"10000000000000000.99",true,"hello","1970-01-01T00:00:00Z","1970-01-01","00:00:00","aGVsbG8="]
`
		wantArrayPretty = `[64, 64.64, "10000000000000000.99", true, "hello", "1970-01-01T00:00:00Z", "1970-01-01", "00:00:00", "aGVsbG8="]
[null, null, null, null, null, null, null, null, null]
[64, 64.64, "10000000000000000.99", true, "hello", "1970-01-01T00:00:00Z", "1970-01-01", "00:00:00", "aGVsbG8="]
`
		wantObjectsNoPretty = `{"col_int":64,"col_float":64.64,"col_decimal":"10000000000000000.99","col_bool":true,"col_text":"hello","col_datetime":"1970-01-01T00:00:00Z","col_date":"1970-01-01","col_time":"00:00:00","col_bytes":"aGVsbG8="}
{"col_int":null,"col_float":null,"col_decimal":null,"col_bool":null,"col_text":null,"col_datetime":null,"col_date":null,"col_time":null,"col_bytes":null}
{"col_int":64,"col_float":64.64,"col_decimal":"10000000000000000.99","col_bool":true,"col_text":"hello","col_datetime":"1970-01-01T00:00:00Z","col_date":"1970-01-01","col_time":"00:00:00","col_bytes":"aGVsbG8="}
`
		wantObjectsPretty = `{"col_int": 64, "col_float": 64.64, "col_decimal": "10000000000000000.99", "col_bool": true, "col_text": "hello", "col_datetime": "1970-01-01T00:00:00Z", "col_date": "1970-01-01", "col_time": "00:00:00", "col_bytes": "aGVsbG8="}
{"col_int": null, "col_float": null, "col_decimal": null, "col_bool": null, "col_text": null, "col_datetime": null, "col_date": null, "col_time": null, "col_bytes": null}
{"col_int": 64, "col_float": 64.64, "col_decimal": "10000000000000000.99", "col_bool": true, "col_text": "hello", "col_datetime": "1970-01-01T00:00:00Z", "col_date": "1970-01-01", "col_time": "00:00:00", "col_bytes": "aGVsbG8="}
`
	)

	testCases := []struct {
		name      string
		pretty    bool
		color     bool
		factoryFn func(io.Writer, *output.Printing) output.RecordWriter
		multiline bool
		want      string
	}{
		{
			name:      "std_no_pretty",
			pretty:    false,
			color:     false,
			factoryFn: jsonw.NewStdRecordWriter,
			want:      wantStdJSONNoPretty,
		},
		{
			name:      "std_pretty",
			pretty:    true,
			color:     false,
			factoryFn: jsonw.NewStdRecordWriter,
			want:      wantStdJSONPretty,
		},
		{
			name:      "array_no_pretty",
			pretty:    false,
			color:     false,
			multiline: true,
			factoryFn: jsonw.NewArrayRecordWriter,
			want:      wantArrayNoPretty,
		},
		{
			name:      "array_pretty",
			pretty:    true,
			color:     false,
			multiline: true,
			factoryFn: jsonw.NewArrayRecordWriter,
			want:      wantArrayPretty,
		},
		{
			name:      "object_no_pretty",
			pretty:    false,
			color:     false,
			multiline: true,
			factoryFn: jsonw.NewObjectRecordWriter,
			want:      wantObjectsNoPretty,
		},
		{
			name:      "object_pretty",
			pretty:    true,
			color:     false,
			multiline: true,
			factoryFn: jsonw.NewObjectRecordWriter,
			want:      wantObjectsPretty,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			colNames, kinds := fixt.ColNamePerKind(false, false, false)
			recMeta := testh.NewRecordMeta(colNames, kinds)

			v0, v1, v2, v3, v4, v5, v6, v7, v8 := int64(64), float64(64.64), "10000000000000000.99", true, "hello", time.Unix(0,
				0).UTC(), time.Unix(0, 0).UTC(), time.Unix(0, 0).UTC(), []byte("hello")

			recs := []sqlz.Record{
				{&v0, &v1, &v2, &v3, &v4, &v5, &v6, &v7, &v8},
				{nil, nil, nil, nil, nil, nil, nil, nil, nil},
				{&v0, &v1, &v2, &v3, &v4, &v5, &v6, &v7, &v8},
			}

			buf := &bytes.Buffer{}
			pr := output.NewPrinting()
			pr.EnableColor(tc.color)
			pr.Compact = !tc.pretty

			w := tc.factoryFn(buf, pr)

			require.NoError(t, w.Open(recMeta))
			require.NoError(t, w.WriteRecords(recs))
			require.NoError(t, w.Close())
			require.Equal(t, tc.want, buf.String())

			if !tc.multiline {
				require.True(t, json.Valid(buf.Bytes()))
				return
			}

			lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			require.Equal(t, len(recs), len(lines))
			for _, line := range lines {
				require.True(t, json.Valid([]byte(line)))
			}
		})
	}
}

func TestErrorWriter(t *testing.T) {
	testCases := []struct {
		name   string
		pretty bool
		color  bool
		want   string
	}{
		{
			name:   "no_pretty",
			pretty: false,
			color:  false,
			want:   "{\"error\": \"err1\"}\n",
		},
		{
			name:   "pretty",
			pretty: true,
			color:  false,
			want:   "{\n  \"error\": \"err1\"\n}\n",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(t.Name(), func(t *testing.T) {
			buf := &bytes.Buffer{}
			pr := output.NewPrinting()
			pr.Compact = !tc.pretty
			pr.EnableColor(tc.color)

			errw := jsonw.NewErrorWriter(slogt.New(t), buf, pr)
			errw.Error(errz.New("err1"))
			got := buf.String()

			require.Equal(t, tc.want, got)
		})
	}
}
