package csv_test

import (
	"testing"

	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/samber/lo"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestSmoke(t *testing.T) {
	t.Parallel()

	testCases := []string{sakila.CSVActor, sakila.TSVActor, sakila.CSVActorHTTP}

	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(sakila.CSVActor)

			sink, err := th.QuerySQL(src, "SELECT * FROM data")
			require.NoError(t, err)
			require.Equal(t, len(sakila.TblActorCols()), len(sink.RecMeta))
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
		})
	}
}

func TestQuerySQL_Count(t *testing.T) {
	t.Parallel()

	testCases := []string{sakila.CSVActor, sakila.TSVActor}
	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)

			sink, err := th.QuerySQL(src, "SELECT * FROM data")
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))

			sink, err = th.QuerySQL(src, "SELECT COUNT(*) FROM data")
			require.NoError(t, err)
			require.EqualValues(t, lo.ToPtr[int64](sakila.TblActorCount), sink.Result())
		})
	}
}

func TestEmptyAsNull(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	sink, err := th.QuerySLQ(sakila.CSVAddress+`| .data | .[0:1]`, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))

	require.Equal(t, stringz.Strings(sakila.TblAddressColKinds()), stringz.Strings(sink.RecMeta.Kinds()))

	ts, err := stringz.ParseTimestampUTC("2020-02-15T06:59:28Z")
	require.NoError(t, err)

	rec0 := sink.Recs[0]
	want := []any{
		lo.ToPtr[int64](1),
		lo.ToPtr("47 MySakila Drive"),
		nil,
		nil,
		lo.ToPtr[int64](300),
		nil,
		nil,
		lo.ToPtr(ts),
	}

	for i := range want {
		require.EqualValues(t, want[i], rec0[i], "field [%d]", i)
	}
}
