//nolint:lll
package jsonw_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw"
	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
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
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			colNames, kinds := fixt.ColNamePerKind(false, false, false)
			recMeta := testh.NewRecordMeta(colNames, kinds)

			v0, v1, v2, v3, v4, v5, v6, v7, v8 := int64(64), float64(64.64), "10000000000000000.99", true, "hello", time.Unix(0,
				0).UTC(), time.Unix(0, 0).UTC(), time.Unix(0, 0).UTC(), []byte("hello")

			recs := []record.Record{
				{v0, v1, v2, v3, v4, v5, v6, v7, v8},
				{nil, nil, nil, nil, nil, nil, nil, nil, nil},
				{v0, v1, v2, v3, v4, v5, v6, v7, v8},
			}

			for _, rec := range recs {
				i, err := record.Valid(rec)
				require.NoError(t, err)
				require.Equal(t, -1, i)
			}

			buf := &bytes.Buffer{}
			pr := output.NewPrinting()
			pr.EnableColor(tc.color)
			pr.Compact = !tc.pretty

			w := tc.factoryFn(buf, pr)

			require.NoError(t, w.Open(ctx, recMeta))
			require.NoError(t, w.WriteRecords(ctx, recs))
			require.NoError(t, w.Close(ctx))
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
			want:   "{\"error\":\"err1\"}\n",
		},
		{
			name:   "pretty",
			pretty: true,
			color:  false,
			want:   "{\n  \"error\": \"err1\"\n}\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			pr := output.NewPrinting()
			pr.Compact = !tc.pretty
			pr.EnableColor(tc.color)

			errw := jsonw.NewErrorWriter(lgt.New(t), buf, pr)
			e := errz.New("err1")
			errw.Error(e, e)
			got := buf.String()

			require.Equal(t, tc.want, got)
		})
	}
}

// TestJSONRoundtrip tests writing JSON/JSONA/JSONL output from a query and then
// reading it back with "sq inspect". This verifies that JSON files
// created by sq can be correctly detected and read.
func TestJSONRoundtrip(t *testing.T) {
	testCases := []struct {
		name       string
		ext        string
		formatFlag string
		wantDriver string
	}{
		{
			name:       "json",
			ext:        ".json",
			formatFlag: "--json",
			wantDriver: "json",
		},
		{
			name:       "jsona",
			ext:        ".jsona",
			formatFlag: "--jsona",
			wantDriver: "jsona",
		},
		{
			name:       "jsonl",
			ext:        ".jsonl",
			formatFlag: "--jsonl",
			wantDriver: "jsonl",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			th := testh.New(t)
			src := th.Source(sakila.SL3)

			// Create a temp file path for the JSON output
			jsonPath := filepath.Join(t.TempDir(), "actor_roundtrip"+tc.ext)

			// Step 1: Query .actor table and write to JSON file
			tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
			require.NoError(t, tr.Exec(".actor", tc.formatFlag, "--output", jsonPath))

			// Step 2: Add the JSON file as a source (fresh TestRun needed
			// so the JSON file becomes the only/active source)
			tr = testrun.New(th.Context, t, nil).Hush()
			require.NoError(t, tr.Exec("add", jsonPath))

			// Step 3: Inspect the added source
			require.NoError(t, tr.Reset().Exec("inspect", "--json"))

			// Verify we got expected output
			require.Equal(t, tc.wantDriver, tr.JQ(".driver"))
			require.Equal(t, "data", tr.JQ(".tables[0].name"))
			require.Equal(t, float64(sakila.TblActorCount), tr.JQ(".tables[0].row_count"))
		})
	}
}
