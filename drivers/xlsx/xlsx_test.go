package xlsx_test

import (
	"testing"

	options2 "github.com/neilotoole/sq/cli/config/options"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/xlsx"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
)

func Test_Smoke_Subset(t *testing.T) {
	th := testh.New(t)
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
		Options:  options2.Options{xlsx.OptImportHeader.Key(): true},
	}

	require.True(t, src.Options.IsSet(xlsx.OptImportHeader))

	hasHeader := xlsx.OptImportHeader.Get(src.Options)
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
