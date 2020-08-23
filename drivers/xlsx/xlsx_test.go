package xlsx_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/xlsx"
	"github.com/neilotoole/sq/libsq/core/options"
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
	testh.SkipShort(t, true)

	th := testh.New(t)
	src := th.Source(sakila.XLSX)

	sink, err := th.QuerySQL(src, "SELECT * FROM actor")
	require.NoError(t, err)
	require.Equal(t, len(sakila.TblActorCols()), len(sink.RecMeta))
	require.Equal(t, sakila.TblActorCount, len(sink.Recs))
}

func Test_XLSX_BadDateRecognition(t *testing.T) {
	t.Parallel()

	// https://github.com/neilotoole/sq-preview/issues/2
	th := testh.New(t)

	src := &source.Source{
		Handle:   "@xlsx_bad_date",
		Type:     xlsx.Type,
		Location: proj.Abs("drivers/xlsx/testdata/problem_with_recognizing_date_colA.xlsx"),
		Options:  options.Options{"header": []string{"true"}},
	}

	hasHeader, ok, err := options.HasHeader(src.Options)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, hasHeader)

	sink, err := th.QuerySQL(src, "SELECT * FROM Summary")
	require.NoError(t, err)
	require.Equal(t, 21, len(sink.Recs))
}
