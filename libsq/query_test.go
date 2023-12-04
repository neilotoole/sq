package libsq_test

import (
	"slices"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// driverMap is a map of drivertype.Type to a string.
// It is used to specify a string for a specific driver.
type driverMap map[drivertype.Type]string

// queryTestCase is used to test libsq's rendering of SLQ into SQL.
// It is probably the most important test struct in the codebase.
type queryTestCase struct {
	// name is the test name
	name string

	// skip indicates the test should be skipped. Useful for test cases
	// that we want to implement in the future.
	skip bool

	// in is the SLQ input. The "@sakila" handle is replaced
	// with the source's actual handle before an individual
	// test cases is executed.
	in string

	// args contains args for the "--args a b" mechanism. Typically empty.
	args map[string]string

	// wantErr indicates that an error is expected
	wantErr bool

	// wantSQL is the desired SQL. If empty, the returned SQL is
	// not tested (but is still executed).
	wantSQL string

	// beforeRun is called just before the query is executed. It's an
	// opportunity to modify the query context before execution.
	beforeRun func(tc queryTestCase, th *testh.Helper, qc *libsq.QueryContext)

	// override allows an alternative "wantSQL" for a specific driver type.
	// For example, MySQL uses backtick as the quote char, so it needs
	// a separate wantSQL string.
	override driverMap

	// onlyFor indicates that this test should only run on sources of
	// the specified types. When empty, the test is executed on all types.
	onlyFor []drivertype.Type

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

	// repeatReplace, when non-empty, instructs the test runner to repeat
	// the test, replacing in the input string all occurrences of the first
	// slice element with each subsequent element. For example, given:
	//
	//  in: ".actor | join(.address, .address_id):
	//  repeatReplace: []string{"join", "inner_join"}
	//
	// The test will run once using "join" (the original query) and then another
	// time with ".actor | inner_join(.address, .address_id)".
	//
	// Thus the field must be empty, or have at least two elements.
	repeatReplace []string
}

// SinkTestFunc is a function that tests a sink.
type SinkTestFunc func(t testing.TB, sink *testh.RecordSink)

// execQueryTestCase is called by test functions to execute
// a queryTestCase.
func execQueryTestCase(t *testing.T, tc queryTestCase) {
	if tc.skip {
		t.Skip()
	}
	t.Helper()

	switch len(tc.repeatReplace) {
	case 0:
		doExecQueryTestCase(t, tc)
		return
	case 1:
		t.Fatalf("queryTestCase.repeatReplace must be empty or have at least two elements")
		return
	default:
	}

	subTests := make([]queryTestCase, len(tc.repeatReplace))
	for i := range tc.repeatReplace {
		subTests[i] = tc
		subTests[i].name = tu.Name(tc.repeatReplace[i])
		if i == 0 {
			// No need for replacement on first item, it's the original.
			continue
		}

		subTests[i].in = strings.ReplaceAll(
			subTests[i].in,
			tc.repeatReplace[0],
			tc.repeatReplace[i],
		)
	}

	for _, st := range subTests {
		st := st
		t.Run(st.name, func(t *testing.T) {
			doExecQueryTestCase(t, st)
		})
	}
}

// doExecQueryTestCase is called by execQueryTestCase to
// execute a queryTestCase. This function should not be called
// directly by test functions. The query is executed for each
// of the sources in sakila.SQLLatest. To do so, the first
// occurrence of the string "@sakila." is replaced with the
// actual handle of each source. E.g:
//
//	"@sakila | .actor"  -->  "@sakila_pg12 | .actor"
func doExecQueryTestCase(t *testing.T, tc queryTestCase) {
	t.Helper()

	coll := testh.New(t).NewCollection(sakila.SQLLatest()...)
	for _, src := range coll.Sources() {
		src := src

		if len(tc.onlyFor) > 0 && !slices.Contains(tc.onlyFor, src.Type) {
			continue
		}

		t.Run(string(src.Type), func(t *testing.T) {
			t.Helper()

			in := strings.Replace(tc.in, "@sakila", src.Handle, 1)
			t.Logf("QUERY:\n\n%s\n\n", in)
			want := tc.wantSQL
			if overrideWant, ok := tc.override[src.Type]; ok {
				want = overrideWant
			}

			_, err := coll.SetActive(src.Handle, false)
			require.NoError(t, err)

			th := testh.New(t)
			sources := th.Sources()

			qc := &libsq.QueryContext{
				Collection: coll,
				Sources:    sources,
				Args:       tc.args,
			}

			if tc.beforeRun != nil {
				tc.beforeRun(tc, th, qc)
			}

			gotSQL, gotErr := libsq.SLQ2SQL(th.Context, qc, in)
			if tc.wantErr {
				assert.Error(t, gotErr)
				t.Logf("ERROR: %v", gotErr)
				return
			}

			t.Logf("SQL:\n\n%s\n\n", gotSQL)
			require.NoError(t, gotErr)

			if want != "" {
				require.Equal(t, want, gotSQL)
			}

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

// assertSinkCellValue returns a SinkTestFunc that asserts that
// the cell at rowi, coli matches val.
func assertSinkCellValue(rowi, coli int, val any) SinkTestFunc {
	return func(t testing.TB, sink *testh.RecordSink) {
		assert.Equal(t, val, sink.Recs[rowi][coli], "record[%d:%d] (%s)", rowi, coli, sink.RecMeta[coli].Name())
	}
}

// assertSinkColValue returns a SinkTestFunc that asserts that
// the column with index coli of each record matches val.
func assertSinkColValue(coli int, val any) SinkTestFunc {
	return func(t testing.TB, sink *testh.RecordSink) {
		for rowi, rec := range sink.Recs {
			assert.Equal(t, val, rec[coli], "record[%d:%d] (%s)", rowi, coli, sink.RecMeta[coli].Name())
		}
	}
}

// assertSinkColValue returns a SinkTestFunc that asserts that
// the name of column with index coli matches name.
func assertSinkColName(coli int, name string) SinkTestFunc {
	return func(t testing.TB, sink *testh.RecordSink) {
		assert.Equal(t, name, sink.RecMeta[coli].Name(), "column %d", coli)
	}
}

// assertSinkColMungedNames returns a SinkTestFunc that matches col names.
func assertSinkColMungedNames(names ...string) SinkTestFunc {
	return func(t testing.TB, sink *testh.RecordSink) {
		gotNames := sink.RecMeta.MungedNames()
		assert.Equal(t, names, gotNames)
	}
}
