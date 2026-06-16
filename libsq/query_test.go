package libsq_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// oracleDoubleQuotedRE matches Postgres-style "identifier" segments in expected SQL.
// Oracle's dialect enquotes by uppercasing the identifier; oracleSQL applies the same
// transform so shared wantSQL strings can be reused for Oracle golden tests.
var oracleDoubleQuotedRE = regexp.MustCompile(`"([^"]*)"`)

// oracleTableAliasASRE matches `FROM|JOIN <tbl> AS <alias>` where the table
// reference (optionally schema-qualified) and alias are double-quoted. Oracle
// rejects the AS keyword between a table reference and its alias; oracleSQL
// strips it so shared wantSQL strings stay reusable.
var oracleTableAliasASRE = regexp.MustCompile(
	`((?:FROM|JOIN)\s+(?:"[^"]+"\.)?"[^"]+")\s+AS(\s+"[^"]+")`,
)

// oracleSQL returns s with every double-quoted identifier uppercased inside the
// quotes (matching drivers/oracle.enquoteOracle) and the AS keyword removed
// from table-alias positions (matching drivers/oracle.preRenderOracle).
func oracleSQL(s string) string {
	s = oracleDoubleQuotedRE.ReplaceAllStringFunc(s, func(m string) string {
		inner := m[1 : len(m)-1]
		return stringz.DoubleQuote(strings.ToUpper(inner))
	})
	return oracleTableAliasASRE.ReplaceAllString(s, "$1$2")
}

func TestOracleSQL(t *testing.T) {
	t.Parallel()
	assert.Equal(t,
		`SELECT "ACTOR_ID" FROM "ACTOR" WHERE "FIRST_NAME" = 'x'`,
		oracleSQL(`SELECT "actor_id" FROM "actor" WHERE "first_name" = 'x'`),
	)
}

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

	// wantErrContains, when non-empty, requires that the returned error
	// message contains this substring. Implies wantErr=true.
	wantErrContains string

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
func execQueryTestCase(t *testing.T, tc queryTestCase) { //nolint:thelper
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

	th := testh.New(t)
	var handles []string
	for _, handle := range sakila.SQLLatest() {
		if th.SourceConfigured(handle) {
			handles = append(handles, handle)
		} else {
			t.Logf("Skipping because source %s is not configured", handle)
		}
	}

	coll := th.NewCollection(handles...)
	for _, src := range coll.Sources() {
		if len(tc.onlyFor) > 0 && !slices.Contains(tc.onlyFor, src.Type) {
			continue
		}

		t.Run(string(src.Type), func(t *testing.T) {
			t.Helper()

			in := strings.Replace(tc.in, "@sakila", src.Handle, 1)
			for _, handle := range testh.ExtractHandlesFromQuery(t, tc.in, false) {
				if !testh.New(t).SourceConfigured(handle) {
					t.Skipf("Skipping because source %s is not configured", handle)
					return
				}
			}

			t.Logf("QUERY:\n\n%s\n\n", in)
			want := tc.wantSQL
			if overrideWant, ok := tc.override[src.Type]; ok {
				want = overrideWant
			} else if want != "" && src.Type == drivertype.Oracle {
				// Multi-source queries execute on the scratch database
				// (SQLite by default), not the source's dialect, so the
				// Oracle uppercase transform doesn't apply.
				if len(testh.ExtractHandlesFromQuery(t, tc.in, false)) <= 1 {
					want = oracleSQL(want)
				}
			}

			_, err := coll.SetActive(src.Handle, false)
			require.NoError(t, err)

			th := testh.New(t)
			qc := &libsq.QueryContext{
				Collection: coll,
				Grips:      th.Grips(),
				Args:       tc.args,
			}

			if tc.beforeRun != nil {
				tc.beforeRun(tc, th, qc)
			}

			gotRes, gotErr := libsq.SLQ2SQL(th.Context, qc, in)
			if tc.wantErr || tc.wantErrContains != "" {
				require.Error(t, gotErr)
				if tc.wantErrContains != "" {
					require.ErrorContains(t, gotErr, tc.wantErrContains)
				}
				t.Logf("ERROR: %v", gotErr)
				return
			}

			require.NoError(t, gotErr)
			t.Logf("SQL:\n\n%s\n\n", gotRes.SQL)

			if want != "" {
				require.Equal(t, want, gotRes.SQL)
			}

			if tc.skipExec {
				return
			}

			sink, err := th.QuerySLQ(in, tc.args)
			require.NoError(t, err)
			sink.SrcType = src.Type
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
	return func(tb testing.TB, sink *testh.RecordSink) {
		tb.Helper()
		assert.Equal(tb, val, sink.Recs[rowi][coli], "record[%d:%d] (%s)", rowi, coli, sink.RecMeta[coli].Name())
	}
}

// assertSinkColValue returns a SinkTestFunc that asserts that
// the column with index coli of each record matches val.
func assertSinkColValue(coli int, val any) SinkTestFunc {
	return func(tb testing.TB, sink *testh.RecordSink) {
		tb.Helper()
		for rowi, rec := range sink.Recs {
			assert.Equal(tb, val, rec[coli], "record[%d:%d] (%s)", rowi, coli, sink.RecMeta[coli].Name())
		}
	}
}

// assertSinkColDecimal returns a SinkTestFunc that asserts that column coli has
// kind.Decimal and that each record's value in that column formats to want.
// Asserting the formatted value (rather than a specific Go type) keeps the check
// independent both of the scale a backend's cast produced and of how a driver
// represents a decimal in a record: most surface decimal.Decimal, but ClickHouse
// surfaces a pre-formatted string. A driver listed in perDriver uses its
// override value instead of want, covering backends like SQLite/rqlite that
// compute decimal sums in floating point and so surface a drifted value. See
// issue #839.
func assertSinkColDecimal(coli int, want string, perDriver driverMap) SinkTestFunc {
	return func(tb testing.TB, sink *testh.RecordSink) {
		tb.Helper()
		w := want
		if override, ok := perDriver[sink.SrcType]; ok {
			w = override
		}
		require.Equal(tb, kind.Decimal, sink.RecMeta[coli].Kind(),
			"column %d (%s) kind", coli, sink.RecMeta[coli].Name())
		for rowi, rec := range sink.Recs {
			var got string
			switch v := rec[coli].(type) {
			case decimal.Decimal:
				got = stringz.FormatDecimal(v)
			case string:
				got = v
			default:
				require.Fail(tb, "unexpected decimal representation",
					"record[%d:%d] (%s): want decimal.Decimal or string, got %T",
					rowi, coli, sink.RecMeta[coli].Name(), rec[coli])
			}
			require.Equal(tb, w, got, "record[%d:%d] (%s)",
				rowi, coli, sink.RecMeta[coli].Name())
		}
	}
}

// assertSinkColKind returns a SinkTestFunc that asserts that the column with
// index coli has kind k, independent of any row values (so it also works when
// the column is all-NULL, e.g. a sum() over an empty result set).
func assertSinkColKind(coli int, k kind.Kind) SinkTestFunc {
	return func(tb testing.TB, sink *testh.RecordSink) {
		tb.Helper()
		require.Equal(tb, k, sink.RecMeta[coli].Kind(),
			"column %d (%s) kind", coli, sink.RecMeta[coli].Name())
	}
}

// assertSinkColValue returns a SinkTestFunc that asserts that
// the name of column with index coli matches name.
func assertSinkColName(coli int, name string) SinkTestFunc {
	return func(tb testing.TB, sink *testh.RecordSink) {
		tb.Helper()
		got := sink.RecMeta[coli].Name()
		// Oracle uppercases quoted identifiers (table names, column names,
		// and AS aliases — see enquoteOracle), so column-name assertions
		// against Oracle compare case-insensitively. Test inputs and
		// expected names stay in their natural lowercase form for
		// consistency with other drivers.
		if sink.SrcType == drivertype.Oracle {
			if !strings.EqualFold(name, got) {
				assert.Equal(tb, name, got, "column %d", coli)
			}
			return
		}
		assert.Equal(tb, name, got, "column %d", coli)
	}
}

// assertSinkColMungedNames returns a SinkTestFunc that matches col names.
func assertSinkColMungedNames(names ...string) SinkTestFunc {
	return func(tb testing.TB, sink *testh.RecordSink) {
		tb.Helper()
		gotNames := sink.RecMeta.MungedNames()
		// Oracle uppercases quoted identifiers (table names, column names,
		// and AS aliases — see enquoteOracle), so column-name assertions
		// against Oracle compare case-insensitively. Test inputs and
		// expected names stay in their natural lowercase form for
		// consistency with other drivers.
		if sink.SrcType == drivertype.Oracle && len(names) == len(gotNames) {
			match := true
			for i := range names {
				if !strings.EqualFold(names[i], gotNames[i]) {
					match = false
					break
				}
			}
			if match {
				return
			}
		}
		assert.Equal(tb, names, gotNames)
	}
}
