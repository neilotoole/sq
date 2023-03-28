package stringz_test

import (
	"strings"
	"testing"
	"time"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/stringz"
)

func TestGenerateAlphaColName(t *testing.T) {
	quantity := 704
	colNames := make([]string, quantity)

	for i := 0; i < quantity; i++ {
		colNames[i] = stringz.GenerateAlphaColName(i, false)
	}

	items := []struct {
		index   int
		colName string
	}{
		{0, "A"},
		{1, "B"},
		{25, "Z"},
		{26, "AA"},
		{27, "AB"},
		{51, "AZ"},
		{52, "BA"},
		{53, "BB"},
		{77, "BZ"},
		{78, "CA"},
		{701, "ZZ"},
		{702, "AAA"},
		{703, "AAB"},
	}

	for _, item := range items {
		assert.Equal(t, item.colName, colNames[item.index])
	}
}

func TestUUID(t *testing.T) {
	for i := 0; i < 100; i++ {
		u := stringz.Uniq32()
		require.Equal(t, 32, len(u))
	}
}

func TestPluralize(t *testing.T) {
	testCases := []struct {
		s    string
		i    int
		want string
	}{
		{s: "row(s)", i: 0, want: "rows"},
		{s: "row(s)", i: 1, want: "row"},
		{s: "row(s)", i: 2, want: "rows"},
		{s: "row(s) col(s)", i: 0, want: "rows cols"},
		{s: "row(s) col(s)", i: 1, want: "row col"},
		{s: "row(s) col(s)", i: 2, want: "rows cols"},
		{s: "row(s)", i: 2, want: "rows"},
		{s: "rows", i: 0, want: "rows"},
		{s: "rows", i: 1, want: "rows"},
		{s: "rows", i: 2, want: "rows"},
	}

	for _, tc := range testCases {
		got := stringz.Plu(tc.s, tc.i)
		require.Equal(t, tc.want, got)
	}
}

func TestTrimLen(t *testing.T) {
	testCases := []struct {
		s    string
		i    int
		want string
	}{
		{s: "", i: 0, want: ""},
		{s: "", i: 1, want: ""},
		{s: "abc", i: 0, want: ""},
		{s: "abc", i: 1, want: "a"},
		{s: "abc", i: 2, want: "ab"},
		{s: "abc", i: 3, want: "abc"},
		{s: "abc", i: 4, want: "abc"},
		{s: "abc", i: 5, want: "abc"},
	}

	for _, tc := range testCases {
		got := stringz.TrimLen(tc.s, tc.i)
		require.Equal(t, tc.want, got)
	}
}

func TestRepeatJoin(t *testing.T) {
	testCases := []struct {
		s     string
		count int
		sep   string
		want  string
	}{
		{s: "", count: 0, sep: ",", want: ""},
		{s: "", count: 1, sep: ",", want: ""},
		{s: "?", count: 1, sep: ",", want: "?"},
		{s: "?", count: 2, sep: ",", want: "?,?"},
		{s: "?", count: 3, sep: ",", want: "?,?,?"},
		{s: "?", count: 3, sep: "", want: "???"},
		{s: "?", count: 0, sep: "", want: ""},
		{s: "?", count: 1, sep: "", want: "?"},
		{s: "?", count: 2, sep: "", want: "??"},
		{s: "?", count: 3, sep: "", want: "???"},
	}

	for _, tc := range testCases {
		got := stringz.RepeatJoin(tc.s, tc.count, tc.sep)
		require.Equal(t, tc.want, got)
	}
}

func TestSurround(t *testing.T) {
	testCases := []struct {
		s    string
		w    string
		want string
	}{
		{s: "", w: "__", want: "____"},
		{s: "hello", w: "__", want: "__hello__"},
	}

	for _, tc := range testCases {
		got := stringz.Surround(tc.s, tc.w)
		require.Equal(t, tc.want, got)
	}
}

func TestSurroundSlice(t *testing.T) {
	testCases := []struct {
		a    []string
		w    string
		want []string
	}{
		{a: nil, w: "__", want: nil},
		{a: []string{}, w: "__", want: []string{}},
		{a: []string{""}, w: "__", want: []string{"____"}},
		{a: []string{"hello", "world"}, w: "__", want: []string{"__hello__", "__world__"}},
		{a: []string{"hello", "world"}, w: "", want: []string{"hello", "world"}},
	}

	for _, tc := range testCases {
		got := stringz.SurroundSlice(tc.a, tc.w)
		require.Equal(t, tc.want, got)
	}
}

func TestPrefixSlice(t *testing.T) {
	testCases := []struct {
		a    []string
		w    string
		want []string
	}{
		{a: nil, w: "__", want: nil},
		{a: []string{}, w: "__", want: []string{}},
		{a: []string{""}, w: "__", want: []string{"__"}},
		{a: []string{"hello", "world"}, w: "__", want: []string{"__hello", "__world"}},
		{a: []string{"hello", "world"}, w: "", want: []string{"hello", "world"}},
	}

	for _, tc := range testCases {
		got := stringz.PrefixSlice(tc.a, tc.w)
		require.Equal(t, tc.want, got)
	}
}

func TestParseBool(t *testing.T) {
	testCases := map[string]bool{
		"1":     true,
		"t":     true,
		"true":  true,
		"TRUE":  true,
		"y":     true,
		"Y":     true,
		"yes":   true,
		"Yes":   true,
		"YES":   true,
		"0":     false,
		"f":     false,
		"false": false,
		"False": false,
		"n":     false,
		"N":     false,
		"no":    false,
		"No":    false,
		"NO":    false,
	}

	for input, wantBool := range testCases {
		gotBool, gotErr := stringz.ParseBool(input)
		require.NoError(t, gotErr)
		require.Equal(t, wantBool, gotBool)
	}

	invalid := []string{"", " ", " true ", "gibberish", "-1"}
	for _, input := range invalid {
		_, gotErr := stringz.ParseBool(input)
		require.Error(t, gotErr)
	}
}

// TestSliceIndex_InSlice tests SliceIndex and InSlice.
func TestSliceIndex_InSlice(t *testing.T) {
	const needle = "hello"

	testCases := []struct {
		haystack []string
		needle   string
		want     int
	}{
		{haystack: nil, needle: needle, want: -1},
		{haystack: []string{}, needle: needle, want: -1},
		{haystack: []string{needle}, needle: needle, want: 0},
		{haystack: []string{"a", needle}, needle: needle, want: 1},
		{haystack: []string{"a", needle, "c"}, needle: needle, want: 1},
		{haystack: []string{"a", "b", needle}, needle: needle, want: 2},
	}

	for _, tc := range testCases {
		require.Equal(t, tc.want, stringz.SliceIndex(tc.haystack, tc.needle))
		// Also test sister func InSlice
		require.Equal(t, tc.want >= 0, stringz.InSlice(tc.haystack, tc.needle))
	}
}

func TestUniqTableName(t *testing.T) {
	t.Parallel()

	testCases := []string{
		"",
		"a",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	for _, inputTblName := range testCases {
		gotTblName := stringz.UniqTableName(inputTblName)
		require.True(t, len(gotTblName) > 0)
		require.True(t, len(gotTblName) < 64)
		t.Logf("%d: %s", len(gotTblName), gotTblName)
	}
}

func TestSanitizeAlphaNumeric(t *testing.T) {
	testCases := map[string]string{
		"":         "",
		" ":        "_",
		"_":        "_",
		"abc123":   "abc123",
		"_abc_123": "_abc_123",
		"a#2%3.4_": "a_2_3_4_",
		"😀abc123😀": "_abc123_",
	}

	for input, want := range testCases {
		got := stringz.SanitizeAlphaNumeric(input, '_')
		require.Equal(t, want, got)
	}
}

func TestLineCount(t *testing.T) {
	testCases := []struct {
		in        string
		withEmpty int
		skipEmpty int
	}{
		{in: "", withEmpty: 0, skipEmpty: 0},
		{in: "\n", withEmpty: 1, skipEmpty: 0},
		{in: "\n\n", withEmpty: 2, skipEmpty: 0},
		{in: "\n\n", withEmpty: 2, skipEmpty: 0},
		{in: " ", withEmpty: 1, skipEmpty: 1},
		{in: "one", withEmpty: 1, skipEmpty: 1},
		{in: "one\n", withEmpty: 1, skipEmpty: 1},
		{in: "\none\n", withEmpty: 2, skipEmpty: 1},
		{in: "one\ntwo", withEmpty: 2, skipEmpty: 2},
		{in: "one\ntwo\n", withEmpty: 2, skipEmpty: 2},
		{in: "one\ntwo\n ", withEmpty: 3, skipEmpty: 3},
		{in: "one\n\nthree", withEmpty: 3, skipEmpty: 2},
		{in: "one\n\nthree\n", withEmpty: 3, skipEmpty: 2},
	}

	require.Equal(t, -1, stringz.LineCount(nil, true))

	for i, tc := range testCases {
		tc := tc

		t.Run(tutil.Name(i, tc.in), func(t *testing.T) {
			count := stringz.LineCount(strings.NewReader(tc.in), false)
			require.Equal(t, tc.withEmpty, count)
			count = stringz.LineCount(strings.NewReader(tc.in), true)
			require.Equal(t, tc.skipEmpty, count)
		})
	}
}

func TestTimestampUTC(t *testing.T) {
	tm := time.Date(2021, 0o1, 0o1, 7, 7, 7, 0, time.UTC)
	s := stringz.TimestampUTC(tm)
	t.Log(s)
	require.Equal(t, "2021-01-01T07:07:07.000Z", s)

	s = stringz.TimestampUTC(time.Now().UTC())
	t.Log(s)
}

func TestParseDateOrTimestampUTC(t *testing.T) {
	testCases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{in: "", wantErr: true},
		{in: "not_a_time", wantErr: true},
		{in: "2021-01-16T18:18:49.348-0700", want: 1610846329},
		{in: "2021-01-16T18:26:39.216-07:00", want: 1610846799},
		{in: "2021-01-16T18:26:39-07:00", want: 1610846799},
		{in: "2021-01-17T01:26:39.216Z", want: 1610846799},
		{in: "2021-01-17", want: 1610841600},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, tc.in), func(t *testing.T) {
			tm, err := stringz.ParseDateOrTimestampUTC(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			t.Logf("Got: %s", stringz.TimestampUTC(tm))
			ut := tm.Unix()
			require.Equal(t, tc.want, ut)
		})
	}
}

func TestZuluTimestamp(t *testing.T) {
	const (
		input = `2022-12-30T09:36:31-0700`
		want  = `2022-12-30T16:36:31Z`
	)
	got := stringz.TimestampToRFC3339(input)
	require.Equal(t, want, got)
}

func TestDoubleQuote(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{in: ``, want: `""`},
		{in: `"hello"`, want: `"""hello"""`},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			got := stringz.DoubleQuote(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestBacktickQuote(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{in: "", want: "``"},
		{in: "`world`", want: "```world```"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			got := stringz.BacktickQuote(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestSingleQuote(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{in: "", want: "''"},
		{in: "jessie's girl", want: "'jessie''s girl'"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			got := stringz.SingleQuote(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}
