package libsq_test

import (
	"reflect"
	"testing"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// TestQuerySQL_Smoke is a smoke test of testh.QuerySQL.
func TestQuerySQL_Smoke(t *testing.T) {
	t.Parallel()

	wantActorFieldTypes := []reflect.Type{
		sqlz.RTypeInt64P,
		sqlz.RTypeStringP,
		sqlz.RTypeStringP,
		sqlz.RTypeTimeP,
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
			tutil.SkipShort(t, tc.handle == sakila.XLSX)
			t.Parallel()

			th := testh.New(t)
			src := th.Source(tc.handle)

			tblName := sakila.TblActor
			if th.IsMonotable(src) {
				tblName = source.MonotableName
			}

			sink, err := th.QuerySQL(src, "SELECT * FROM "+tblName)
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

func TestQuerySQL_Count(t *testing.T) {
	t.Parallel()

	testCases := sakila.SQLAll()
	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)

			sink, err := th.QuerySQL(src, "SELECT * FROM "+sakila.TblActor)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))

			sink, err = th.QuerySQL(src, "SELECT COUNT(*) FROM "+sakila.TblActor)
			require.NoError(t, err)
			count, ok := sink.Recs[0][0].(*int64)
			require.True(t, ok)
			require.Equal(t, int64(sakila.TblActorCount), *count)
		})
	}
}
