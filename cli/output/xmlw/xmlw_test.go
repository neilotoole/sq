//nolint:lll
package xmlw_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/neilotoole/sq/testh/testsrc"

	"github.com/neilotoole/sq/cli/output"

	"github.com/neilotoole/sq/cli/output/xmlw"

	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/stretchr/testify/require"
)

func TestRecordWriter_Actor(t *testing.T) {
	const (
		want0 = `<?xml version="1.0"?>
<records />
`
		want3Pretty = `<?xml version="1.0"?>
<records>
  <record>
    <actor_id>1</actor_id>
    <first_name>PENELOPE</first_name>
    <last_name>GUINESS</last_name>
    <last_update>2020-06-11T02:50:54Z</last_update>
  </record>
  <record>
    <actor_id>2</actor_id>
    <first_name>NICK</first_name>
    <last_name>WAHLBERG</last_name>
    <last_update>2020-06-11T02:50:54Z</last_update>
  </record>
  <record>
    <actor_id>3</actor_id>
    <first_name>ED</first_name>
    <last_name>CHASE</last_name>
    <last_update>2020-06-11T02:50:54Z</last_update>
  </record>
</records>
`

		want3NoPretty = `<?xml version="1.0"?>
<records>
<record><actor_id>1</actor_id><first_name>PENELOPE</first_name><last_name>GUINESS</last_name><last_update>2020-06-11T02:50:54Z</last_update></record>
<record><actor_id>2</actor_id><first_name>NICK</first_name><last_name>WAHLBERG</last_name><last_update>2020-06-11T02:50:54Z</last_update></record>
<record><actor_id>3</actor_id><first_name>ED</first_name><last_name>CHASE</last_name><last_update>2020-06-11T02:50:54Z</last_update></record>
</records>
`
	)

	testCases := []struct {
		name    string
		color   bool
		pretty  bool
		numRecs int
		want    string
	}{
		{name: "actor_0_pretty", color: false, pretty: true, numRecs: 0, want: want0},
		{name: "actor_0_no_pretty", color: false, pretty: false, numRecs: 0, want: want0},
		{name: "actor_3_pretty", color: false, pretty: true, numRecs: 3, want: want3Pretty},
		{name: "actor_3_no_pretty", color: false, pretty: false, numRecs: 3, want: want3NoPretty},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			fm := output.NewFormatting()
			fm.EnableColor(tc.color)
			fm.Pretty = tc.pretty

			recMeta, recs := testh.RecordsFromTbl(t, sakila.SL3, sakila.TblActor)
			recs = recs[0:tc.numRecs]

			buf := &bytes.Buffer{}

			w := xmlw.NewRecordWriter(buf, fm)
			require.NoError(t, w.Open(recMeta))
			require.NoError(t, w.WriteRecords(recs))
			require.NoError(t, w.Close())

			require.Equal(t, tc.want, buf.String())
		})
	}
}

func TestRecordWriter_TblTypes(t *testing.T) {
	fm := output.NewFormatting()
	fm.EnableColor(false)

	recMeta, recs := testh.RecordsFromTbl(t, testsrc.MiscDB, testsrc.TblTypes)
	buf := &bytes.Buffer{}

	w := xmlw.NewRecordWriter(buf, fm)
	require.NoError(t, w.Open(recMeta))
	require.NoError(t, w.WriteRecords(recs))
	require.NoError(t, w.Close())

	want, err := os.ReadFile("testdata/tbl_types.xml")
	require.NoError(t, err)
	require.Equal(t, string(want), buf.String())
}
