package yamlw_test

import (
	"bytes"
	"io"
	"testing"
	"time"

	goccy "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/yamlw"
	"github.com/neilotoole/sq/libsq/core/timez"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestRecordWriter(t *testing.T) {
	const want = `- last_update: 2020-06-11T02:50:54Z
  actor_id: 1
  last_name: GUINESS
  first_name: PENELOPE
- last_update: 2020-06-11T02:50:54Z
  actor_id: 2
  last_name: WAHLBERG
  first_name: NICK
`

	th, src, _, _, _ := testh.NewWith(t, sakila.SL3)
	query := src.Handle + ".actor | .last_update, .actor_id, .last_name, .first_name | .[0:2]"

	sink, err := th.QuerySLQ(query, nil)
	require.NoError(t, err)
	t.Log(len(sink.Recs))

	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	recw := yamlw.NewRecordWriter(buf, pr)
	require.NoError(t, recw.Open(th.Context, sink.RecMeta))

	err = recw.WriteRecords(th.Context, sink.Recs)
	require.NoError(t, err)
	require.NoError(t, recw.Flush(th.Context))
	require.NoError(t, recw.Close(th.Context))

	want2 := want
	_ = want2
	got := buf.String()
	t.Log("\n" + got)
	require.Equal(t, want, got)
}

func TestGoccyYAMLTimestampEncode(t *testing.T) {
	// The goccy YAML encoder, as of v0.15.9, treats a time.Time and certain
	// string representations of that time differently. The time.Time is rendered
	// without quotes, while the string representation is rendered with quotes.
	// This test verifies that behavior.

	const val = "2020-06-11T02:50:54Z"

	enc := goccy.NewEncoder(io.Discard)

	ts := timez.MustParse(time.RFC3339Nano, val)

	node, err := enc.EncodeToNode(ts)
	require.NoError(t, err)
	str := node.String()
	require.Equal(t, val, str)

	node, err = enc.EncodeToNode(val)
	require.NoError(t, err)
	str = node.String()
	require.Equal(t, `"`+val+`"`, str, "expected quotes around string representation of time")
}
