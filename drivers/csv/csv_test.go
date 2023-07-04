package csv_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/neilotoole/sq/libsq/driver"

	"github.com/neilotoole/sq/cli/testrun"

	"github.com/neilotoole/sq/libsq/core/timez"

	"github.com/neilotoole/sq/libsq/core/stringz"

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
			require.EqualValues(t, int64(sakila.TblActorCount), sink.Result())
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

	ts, err := timez.ParseTimestampUTC("2020-02-15T06:59:28Z")
	require.NoError(t, err)

	rec0 := sink.Recs[0]
	want := []any{
		int64(1),
		"47 MySakila Drive",
		nil,
		nil,
		int64(300),
		nil,
		nil,
		ts,
	}

	for i := range want {
		require.EqualValues(t, want[i], rec0[i], "field [%d]", i)
	}
}

func TestIngestDuplicateColumns(t *testing.T) {
	ctx := context.Background()
	tr := testrun.New(ctx, t, nil)

	err := tr.Exec("add", filepath.Join("testdata", "actor_duplicate_cols.csv"), "--handle", "@actor_dup")
	require.NoError(t, err)

	tr = testrun.New(ctx, t, tr).Hush()
	require.NoError(t, tr.Exec("--csv", ".data"))
	wantHeaders := []string{"actor_id", "first_name", "last_name", "last_update", "actor_id_1"}
	data := tr.MustReadCSV()
	require.Equal(t, wantHeaders, data[0])

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
	require.NoError(t, tr.Exec("--csv", ".data"))
	wantHeaders = []string{"x_actor_id", "x_first_name", "x_last_name", "x_last_update", "x_actor_id_1"}
	data = tr.MustReadCSV()
	require.Equal(t, wantHeaders, data[0])
}
