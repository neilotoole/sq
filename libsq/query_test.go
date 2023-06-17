package libsq_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/neilotoole/sq/libsq"

	"golang.org/x/exp/slices"

	"github.com/neilotoole/sq/libsq/source"

	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// queryTestCase is used to test libsq's rendering of SLQ into SQL.
// It is probably the most important test struct in the codebase.
type queryTestCase struct {
	// name is the test name
	name string

	// skip indicates the test should be skipped. Useful for test cases
	// that we wantSQL to implement in the future.
	skip bool

	// in is the SLQ input. The "@sakila" handle is replaced
	// with the source's actual handle before an individual
	// test cases is executed.
	in string

	// args contains args for the "--args a b" mechanism. Typically empty.
	args map[string]string

	// wantErr indicates that an error is expected
	wantErr bool

	// wantSQL is the wanted SQL
	wantSQL string

	// override allows an alternative "wantSQL" for a specific driver type.
	// For example, MySQL uses backtick as the quote char, so it needs
	// a separate wantSQL string.
	override map[source.DriverType]string

	// onlyFor indicates that this test should only run on sources of
	// the specified types. When empty, the test is executed on all types.
	onlyFor []source.DriverType

	// skipExec indicates that the resulting query should not be executed.
	// Some SLQ inputs we wantSQL to test don't actually have corresponding
	// data in the Sakila datasets.
	skipExec bool

	// wantRecCount is the number of expected records from actually executing
	// the query. This is N/A if skipExec is true.
	wantRecCount int

	// sinkTest, if non-nil, is executed against the sink returned
	// from the query execution.
	sinkFns []SinkTestFunc
}

// SinkTestFunc is a function that tests a sink.
type SinkTestFunc func(t testing.TB, sink *testh.RecordSink)

func execQueryTestCase(t *testing.T, tc queryTestCase) {
	if tc.skip {
		t.Skip()
	}

	t.Helper()

	coll := testh.New(t).NewCollection(sakila.SQLLatest()...)
	// coll := testh.New(t).NewCollection(sakila.Pg)

	for _, src := range coll.Sources() {
		src := src

		t.Run(string(src.Type), func(t *testing.T) {
			t.Helper()

			if len(tc.onlyFor) > 0 {
				if !slices.Contains(tc.onlyFor, src.Type) {
					t.Skip()
				}
			}

			in := strings.Replace(tc.in, "@sakila", src.Handle, 1)
			t.Log(in)
			want := tc.wantSQL
			if overrideWant, ok := tc.override[src.Type]; ok {
				want = overrideWant
			}

			_, err := coll.SetActive(src.Handle, false)
			require.NoError(t, err)

			th := testh.New(t)
			dbases := th.Databases()

			qc := &libsq.QueryContext{
				Collection:      coll,
				DBOpener:        dbases,
				JoinDBOpener:    dbases,
				ScratchDBOpener: dbases,
				Args:            tc.args,
			}

			gotSQL, gotErr := libsq.SLQ2SQL(th.Context, qc, in)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, want, gotSQL)
			t.Log(gotSQL)

			if tc.skipExec {
				return
			}

			sink, err := th.QuerySLQ(in, tc.args)
			require.NoError(t, err)
			require.Equal(t, tc.wantRecCount, len(sink.Recs))

			for i := range tc.sinkFns {
				tc.sinkFns[i](t, sink)
			}
		})
	}
}

// assertSinkColValue returns a SinkTestFunc that asserts that
// the column colIndex of each record matches val.
func assertSinkColValue(colIndex int, val any) SinkTestFunc {
	return func(t testing.TB, sink *testh.RecordSink) {
		for rowi, rec := range sink.Recs {
			assert.Equal(t, val, rec[colIndex], "record[%d:%d] (%s)", rowi, colIndex, sink.RecMeta[colIndex].Name())
		}
	}
}

// assertSinkColValue returns a SinkTestFunc that asserts that
// the name of column colIndex matches name.
func assertSinkColName(colIndex int, name string) SinkTestFunc {
	return func(t testing.TB, sink *testh.RecordSink) {
		assert.Equal(t, name, sink.RecMeta[colIndex].Name(), "column %d", colIndex)
	}
}
