//nolint:lll
package jsonw_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"regexp"
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
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
)

// reANSI matches any ANSI color/reset escape sequence.
var reANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)

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
		factoryFn func(io.Writer, *output.Printing) output.RecordWriter
		name      string
		want      string
		pretty    bool
		color     bool
		multiline bool
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
		want   string
		pretty bool
		color  bool
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

// TestWriteJSON_Color verifies that WriteJSON emits ANSI escape sequences when
// color is enabled, and emits none when monochrome. In both cases the
// underlying bytes, after stripping ANSI, must be valid JSON.
func TestWriteJSON_Color(t *testing.T) {
	val := map[string]any{
		"k":          "v",
		"n":          1,
		"b":          true,
		"null_field": nil,
	}

	t.Run("on", func(t *testing.T) {
		pr := output.NewPrinting()
		pr.EnableColor(true)
		pr.Compact = true
		require.False(t, pr.IsMonochrome())

		var buf bytes.Buffer
		err := jsonw.WriteJSON(&buf, pr, val)
		require.NoError(t, err)

		out := buf.String()
		require.Contains(t, out, "\x1b[", "expected ANSI escape in color output")
		require.Contains(t, out, "\x1b[0m", "expected ANSI reset in color output")

		plain := reANSI.ReplaceAllString(out, "")
		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(plain), &got),
			"stripped output must be valid JSON")
	})

	t.Run("off", func(t *testing.T) {
		pr := output.NewPrinting()
		pr.EnableColor(false)
		pr.Compact = true
		require.True(t, pr.IsMonochrome())

		var buf bytes.Buffer
		err := jsonw.WriteJSON(&buf, pr, val)
		require.NoError(t, err)

		out := buf.String()
		require.NotContains(t, out, "\x1b[", "expected no ANSI escape in monochrome output")

		var got map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &got),
			"monochrome output must be valid JSON")
	})
}

// TestPingWriter_Result verifies that pingWriter.Result emits correct JSON
// for both the success and failure cases.
func TestPingWriter_Result(t *testing.T) {
	src := &source.Source{
		Handle:   "@test",
		Type:     drivertype.SQLite,
		Location: "test://",
	}

	t.Run("success", func(t *testing.T) {
		var buf bytes.Buffer
		pr := output.NewPrinting()
		pr.EnableColor(false)

		pw := jsonw.NewPingWriter(&buf, pr)
		err := pw.Result(src, 100*time.Millisecond, nil)
		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
		require.Equal(t, true, got["pong"], "pong must be true on success")
		require.NotNil(t, got["duration"], "duration must be present")
		require.NotContains(t, got, "error", "error key must be absent on success")
	})

	t.Run("failure", func(t *testing.T) {
		var buf bytes.Buffer
		pr := output.NewPrinting()
		pr.EnableColor(false)

		pw := jsonw.NewPingWriter(&buf, pr)
		pingErr := errors.New("connection refused")
		err := pw.Result(src, 100*time.Millisecond, pingErr)
		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
		require.Equal(t, false, got["pong"], "pong must be false on failure")
		require.Equal(t, "connection refused", got["error"])
	})
}

// TestPingWriter_Result_RedactsLocation verifies that pingWriter.Result
// redacts the Location field when Printing.Redact is true, and emits
// plaintext when Printing.Redact is false (i.e. --reveal).
func TestPingWriter_Result_RedactsLocation(t *testing.T) {
	const (
		plainLoc    = "postgres://alice:hunter2@db.example.com:5432/sakila"
		redactedLoc = "postgres://alice:xxxxx@db.example.com:5432/sakila"
	)

	src := &source.Source{
		Handle:   "@redact_test",
		Type:     drivertype.Pg,
		Location: plainLoc,
	}

	t.Run("redact_on", func(t *testing.T) {
		var buf bytes.Buffer
		pr := output.NewPrinting()
		pr.EnableColor(false)
		pr.Redact = true

		pw := jsonw.NewPingWriter(&buf, pr)
		require.NoError(t, pw.Result(src, 10*time.Millisecond, nil))

		out := buf.String()
		require.NotContains(t, out, "hunter2",
			"plaintext password must not appear when Redact is true")
		require.Contains(t, out, redactedLoc,
			"redacted location must appear in output")

		// The caller's src must not be mutated.
		require.Equal(t, plainLoc, src.Location,
			"caller's source Location must not be mutated")
	})

	t.Run("redact_off", func(t *testing.T) {
		var buf bytes.Buffer
		pr := output.NewPrinting()
		pr.EnableColor(false)
		pr.Redact = false

		pw := jsonw.NewPingWriter(&buf, pr)
		require.NoError(t, pw.Result(src, 10*time.Millisecond, nil))

		out := buf.String()
		require.Contains(t, out, "hunter2",
			"plaintext password must appear when Redact is false (--reveal)")
	})
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
