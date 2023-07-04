package xlsx_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/neilotoole/sq/cli/testrun"

	"github.com/neilotoole/sq/libsq/driver"

	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/xlsx"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
)

func Test_Smoke_Subset(t *testing.T) {
	th := testh.New(t, testh.OptLongOpen())
	src := th.Source(sakila.XLSXSubset)

	sink, err := th.QuerySQL(src, "SELECT * FROM actor")
	require.NoError(t, err)
	require.Equal(t, len(sakila.TblActorCols()), len(sink.RecMeta))
	require.Equal(t, sakila.TblActorCount, len(sink.Recs))
}

func Test_Smoke_Full(t *testing.T) {
	tutil.SkipShort(t, true)

	// This test fails (in GH workflow) on Windows without testh.OptLongOpen.
	// That's probably worth looking into further. It shouldn't be that slow,
	// even on Windows. However, we are going to rewrite the xlsx driver eventually,
	// so it can wait until then.
	// See: https://github.com/neilotoole/sq/issues/200
	th := testh.New(t, testh.OptLongOpen())
	src := th.Source(sakila.XLSX)

	sink, err := th.QuerySQL(src, "SELECT * FROM actor")
	require.NoError(t, err)
	require.Equal(t, len(sakila.TblActorCols()), len(sink.RecMeta))
	require.Equal(t, sakila.TblActorCount, len(sink.Recs))
}

func Test_XLSX_BadDateRecognition(t *testing.T) {
	t.Parallel()

	th := testh.New(t)

	src := &source.Source{
		Handle:   "@xlsx_bad_date",
		Type:     xlsx.Type,
		Location: proj.Abs("drivers/xlsx/testdata/problem_with_recognizing_date_colA.xlsx"),
		Options:  options.Options{driver.OptIngestHeader.Key(): true},
	}

	require.True(t, src.Options.IsSet(driver.OptIngestHeader))

	hasHeader := driver.OptIngestHeader.Get(src.Options)
	require.True(t, hasHeader)

	sink, err := th.QuerySQL(src, "SELECT * FROM Summary")
	require.NoError(t, err)
	require.Equal(t, 21, len(sink.Recs))
}

// TestHandleSomeEmptySheets verifies that sq can import XLSX
// when there are some empty sheets.
func TestHandleSomeEmptySheets(t *testing.T) {
	t.Parallel()

	th := testh.New(t)

	src := &source.Source{
		Handle:   "@xlsx_empty_sheets",
		Type:     xlsx.Type,
		Location: proj.Abs("drivers/xlsx/testdata/test_with_some_empty_sheets.xlsx"),
	}

	sink, err := th.QuerySQL(src, "SELECT * FROM Sheet1")
	require.NoError(t, err)
	require.Equal(t, 2, len(sink.Recs))
}

func TestIngestDuplicateColumns(t *testing.T) {
	ctx := context.Background()
	tr := testrun.New(ctx, t, nil)

	err := tr.Exec("add",
		"--handle", "@actor_dup",
		"--ingest.header=true",
		filepath.Join("testdata", "actor_duplicate_cols.xlsx"),
	)
	require.NoError(t, err)

	tr = testrun.New(ctx, t, tr).Hush()
	require.NoError(t, tr.Exec("--csv", ".actor"))
	wantHeaders := []string{"actor_id", "first_name", "last_name", "last_update", "actor_id_1"}
	data := tr.MustReadCSV()
	require.Equal(t, wantHeaders, data[0])

	// Make sure the data is correct
	require.Len(t, data, sakila.TblActorCount+1) // +1 for header row
	wantFirstDataRecord := []string{"1", "PENELOPE", "GUINESS", "2020-02-15T06:59:28Z", "1"}
	require.Equal(t, wantFirstDataRecord, data[1])

	// Verify that changing the template works
	const tpl2 = "x_{{.Name}}{{with .Recurrence}}_{{.}}{{end}}"

	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec(
		"config",
		"set",
		driver.OptIngestColRename.Key(),
		tpl2,
	))
	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("--csv", ".actor"))
	wantHeaders = []string{"x_actor_id", "x_first_name", "x_last_name", "x_last_update", "x_actor_id_1"}
	data = tr.MustReadCSV()
	require.Equal(t, wantHeaders, data[0])
}
