package csvw_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/csvw"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/testh"
)

func TestDateTimeHandling(t *testing.T) {
	ctx := context.Background()
	var (
		colNames = []string{"col_datetime", "col_date", "col_time"}
		kinds    = []kind.Kind{kind.Datetime, kind.Date, kind.Time}
		when     = time.Unix(0, 0).UTC()
	)
	const want = "1970-01-01T00:00:00Z\t1970-01-01\t00:00:00\n"

	recMeta := testh.NewRecordMeta(colNames, kinds)
	buf := &bytes.Buffer{}

	pr := output.NewPrinting()
	pr.ShowHeader = false
	pr.EnableColor(false)

	w := csvw.NewTabRecordWriter(buf, pr)
	require.NoError(t, w.Open(ctx, recMeta))

	rec := record.Record{when, when, when}
	require.NoError(t, w.WriteRecords(ctx, []record.Record{rec}))
	require.NoError(t, w.Close(ctx))

	require.Equal(t, want, buf.String())
	t.Log(buf.String())
}
