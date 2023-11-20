package yamlw_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/yamlw"
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
	require.NoError(t, recw.Open(sink.RecMeta))

	err = recw.WriteRecords(sink.Recs)
	require.NoError(t, err)
	require.NoError(t, recw.Flush())
	require.NoError(t, recw.Close())

	want2 := want
	_ = want2
	got := buf.String()
	t.Log("\n" + got)
	require.Equal(t, want, got)
}
