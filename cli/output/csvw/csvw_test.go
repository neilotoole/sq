package csvw_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output/csvw"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/testh"
)

func TestDateTimeHandling(t *testing.T) {
	var (
		colNames = []string{"col_datetime", "col_date", "col_time"}
		kinds    = []sqlz.Kind{sqlz.KindDatetime, sqlz.KindDate, sqlz.KindTime}
		when     = time.Unix(0, 0).UTC()
	)
	const want = "1970-01-01T00:00:00Z\t1970-01-01\t00:00:00\n"

	recMeta := testh.NewRecordMeta(colNames, kinds)
	buf := &bytes.Buffer{}

	w := csvw.NewRecordWriter(buf, false, csvw.Tab)
	require.NoError(t, w.Open(recMeta))

	rec := sqlz.Record{&when, &when, &when}
	require.NoError(t, w.WriteRecords([]sqlz.Record{rec}))
	require.NoError(t, w.Close())

	require.Equal(t, want, buf.String())
	println(buf.String())
}
