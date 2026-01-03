package csvw_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/csvw"
	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
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

// TestCSVRoundtrip tests writing CSV/TSV output from a query and then
// reading it back with "sq inspect". This verifies that CSV/TSV files
// created by sq can be correctly detected and read.
func TestCSVRoundtrip(t *testing.T) {
	testCases := []struct {
		name       string
		ext        string
		formatFlag string
		wantDriver string
	}{
		{
			name:       "csv",
			ext:        ".csv",
			formatFlag: "--csv",
			wantDriver: "csv",
		},
		{
			name:       "tsv",
			ext:        ".tsv",
			formatFlag: "--tsv",
			wantDriver: "tsv",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			th := testh.New(t)
			src := th.Source(sakila.SL3)

			// Create a temp file path for the CSV/TSV output
			csvPath := filepath.Join(t.TempDir(), "actor_roundtrip"+tc.ext)

			// Step 1: Query .actor table and write to CSV/TSV file
			tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
			require.NoError(t, tr.Exec(".actor", tc.formatFlag, "--output", csvPath))

			// Step 2: Add the CSV/TSV file as a source (fresh TestRun needed
			// so the CSV/TSV file becomes the only/active source)
			tr = testrun.New(th.Context, t, nil).Hush()
			require.NoError(t, tr.Exec("add", csvPath))

			// Step 3: Inspect the added source
			require.NoError(t, tr.Reset().Exec("inspect", "--json"))

			// Verify we got expected output
			require.Equal(t, tc.wantDriver, tr.JQ(".driver"))
			require.Equal(t, "data", tr.JQ(".tables[0].name"))
			require.Equal(t, float64(sakila.TblActorCount), tr.JQ(".tables[0].row_count"))
		})
	}
}
