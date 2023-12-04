package libsq_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// TestQuerySQL_Smoke is a smoke test of testh.QuerySQL.
func TestQuerySQL_Smoke(t *testing.T) {
	t.Parallel()

	wantActorFieldTypes := []reflect.Type{
		sqlz.RTypeInt64,
		sqlz.RTypeString,
		sqlz.RTypeString,
		sqlz.RTypeTime,
	}

	testCases := []struct {
		handle     string
		fieldTypes []reflect.Type
	}{
		{
			handle:     sakila.SL3,
			fieldTypes: wantActorFieldTypes,
		},
		{
			handle:     sakila.My,
			fieldTypes: wantActorFieldTypes,
		},
		{
			handle:     sakila.Pg,
			fieldTypes: wantActorFieldTypes,
		},
		{
			handle:     sakila.MS,
			fieldTypes: wantActorFieldTypes,
		},
		{
			handle:     sakila.CSVActor,
			fieldTypes: wantActorFieldTypes,
		},
		{
			handle:     sakila.XLSX,
			fieldTypes: wantActorFieldTypes,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.handle, func(t *testing.T) {
			tu.SkipShort(t, tc.handle == sakila.XLSX)
			t.Parallel()

			th := testh.New(t, testh.OptLongOpen())
			src := th.Source(tc.handle)

			tblName := sakila.TblActor
			if th.IsMonotable(src) {
				tblName = source.MonotableName
			}

			sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+tblName)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
			require.Equal(t, len(tc.fieldTypes), len(sink.Recs[0]))
			for i := range sink.Recs[0] {
				require.Equal(t,
					tc.fieldTypes[i].String(),
					reflect.TypeOf(sink.Recs[0][i]).String(),
					"expected field[%d] {%s} to have type %s but got %s",
					i,
					sink.RecMeta[i].Name(),
					tc.fieldTypes[i].String(),
					reflect.TypeOf(sink.Recs[0][i]).String(),
				)
			}
		})
	}
}

func TestQuerySQL_Count(t *testing.T) { //nolint:tparallel
	testCases := sakila.SQLAll()
	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)

			sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+sakila.TblActor)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))

			sink, err = th.QuerySQL(src, nil, "SELECT COUNT(*) FROM "+sakila.TblActor)
			require.NoError(t, err)
			count, ok := sink.Recs[0][0].(int64)
			require.True(t, ok)
			require.Equal(t, int64(sakila.TblActorCount), count)
		})
	}
}

// TestJoinDuplicateColNamesAreRenamed tests handling of multiple occurrences
// of the same result column name. The expected behavior is that the duplicate
// column is renamed.
func TestJoinDuplicateColNamesAreRenamed(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	const query = "SELECT * FROM actor INNER JOIN film_actor ON actor.actor_id = film_actor.actor_id LIMIT 1"

	sink, err := th.QuerySQL(src, nil, query)
	require.NoError(t, err)
	colNames := sink.RecMeta.MungedNames()
	// Without intervention, the returned column names would contain duplicates.
	//  [actor_id, first_name, last_name, last_update, actor_id, film_id, last_update]

	t.Logf("Cols: [%s]", strings.Join(colNames, ", "))

	colCounts := lo.CountValues(colNames)
	for col, count := range colCounts {
		assert.True(t, count == 1, "col name {%s} is not unique (occurs %d times)",
			col, count)
	}
}
