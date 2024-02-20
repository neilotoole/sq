package stringz_test

import (
	"errors"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh/tu"
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

func TestValidIdent(t *testing.T) {
	testCases := []struct {
		in      string
		wantErr bool
	}{
		{in: "", wantErr: true},
		{in: "hello world", wantErr: true},
		{in: "hello", wantErr: false},
		{in: "1", wantErr: true},
		{in: "$hello", wantErr: true},
		{in: "_hello", wantErr: true},
		{in: "hello_", wantErr: false},
		{in: "Hello_", wantErr: false},
		{in: "Hello_1", wantErr: false},
		{in: "Hello_!!", wantErr: true},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tu.Name(tc.in), func(t *testing.T) {
			gotErr := stringz.ValidIdent(tc.in)
			if tc.wantErr {
				require.Error(t, gotErr)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func TestStrings(t *testing.T) {
	testCases := []struct {
		in   []any
		want []any
	}{
		{
			in:   nil,
			want: []any{},
		},
		{
			in:   []any{"hello", 1, true},
			want: []any{"hello", "1", "true"},
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			got := stringz.Strings(tc.in)
			require.Len(t, got, len(tc.in))

			for j, v := range got {
				require.Equal(t, tc.want[j], v)
			}
		})
	}
}

func TestStringsD(t *testing.T) {
	testCases := []struct {
		in   []any
		want []any
	}{
		{
			in:   nil,
			want: []any{},
		},
		{
			in:   []any{"hello", lo.ToPtr("hello"), 1, lo.ToPtr(1), true, lo.ToPtr(true)},
			want: []any{"hello", "hello", "1", "1", "true", "true"},
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			got := stringz.StringsD(tc.in)
			require.Len(t, got, len(tc.in))

			for j, v := range got {
				require.Equal(t, tc.want[j], v)
			}
		})
	}
}

func TestTemplate(t *testing.T) {
	data := map[string]string{"Name": "wubble"}

	testCases := []struct {
		tpl     string
		data    any
		want    string
		wantErr bool
	}{
		// "upper" is a sprig func. Verify that it loads.
		{"{{.Name | upper}}", data, "WUBBLE", false},
		{"{{not_a_func .Name}}_", data, "", true},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.tpl), func(t *testing.T) {
			got, gotErr := stringz.ExecuteTemplate(t.Name(), tc.tpl, tc.data)
			t.Logf("\nTPL:   %s\nGOT:   %s\nERR:   %v", tc.tpl, got, gotErr)
			if tc.wantErr {
				require.Error(t, gotErr)
				// Also test ValidTemplate while we're at it.
				gotErr = stringz.ValidTemplate(t.Name(), tc.tpl)
				require.Error(t, gotErr)
				return
			}
			require.NoError(t, gotErr)
			gotErr = stringz.ValidTemplate(t.Name(), tc.tpl)
			require.NoError(t, gotErr)

			require.Equal(t, tc.want, got)
		})
	}
}

func TestShellEscape(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{"", "''"},
		{" ", `' '`},
		{"huzzah", "huzzah"},
		{"huz zah", `'huz zah'`},
		{`huz ' zah`, `'huz '"'"' zah'`},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc), func(t *testing.T) {
			got := stringz.ShellEscape(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestEllipsify(t *testing.T) {
	testCases := []struct {
		input  string
		maxLen int
		want   string
	}{
		{input: "", maxLen: 0, want: ""},
		{input: "", maxLen: 1, want: ""},
		{input: "abc", maxLen: 1, want: "…"},
		{input: "abc", maxLen: 2, want: "a…"},
		{input: "abcdefghijk", maxLen: 1, want: "…"},
		{input: "abcdefghijk", maxLen: 2, want: "a…"},
		{input: "abcdefghijk", maxLen: 3, want: "a…k"},
		{input: "abcdefghijk", maxLen: 4, want: "ab…k"},
		{input: "abcdefghijk", maxLen: 5, want: "ab…jk"},
		{input: "abcdefghijk", maxLen: 6, want: "abc…jk"},
		{input: "abcdefghijk", maxLen: 7, want: "abc…ijk"},
		{input: "abcdefghijk", maxLen: 8, want: "abcd…ijk"},
		{input: "abcdefghijk", maxLen: 9, want: "abcd…hijk"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.input, tc.maxLen), func(t *testing.T) {
			got := stringz.Ellipsify(tc.input, tc.maxLen)
			t.Logf("%12q  -->  %12q", tc.input, got)
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("test negative", func(t *testing.T) {
		got := stringz.Ellipsify("abc", -1)
		require.Equal(t, "", got)
	})
}

// TestEllipsifyASCII tests EllipsifyASCII. It verifies that
// the function trims the middle of a string, leaving the
// start and end intact.
func TestEllipsifyASCII(t *testing.T) {
	testCases := []struct {
		input  string
		maxLen int
		want   string
	}{
		{input: "", maxLen: 0, want: ""},
		{input: "", maxLen: 1, want: ""},
		{input: "abc", maxLen: 2, want: "ac"},
		{input: "abcdefghijk", maxLen: 2, want: "ak"},
		{input: "abcdefghijk", maxLen: 3, want: "a.k"},
		{input: "abcdefghijk", maxLen: 4, want: "a..k"},
		{input: "abcdefghijk", maxLen: 5, want: "a...k"},
		{input: "abcdefghijk", maxLen: 6, want: "a...k"},
		{input: "abcdefghijk", maxLen: 7, want: "ab...jk"},
		{input: "abcdefghijk", maxLen: 8, want: "ab...jk"},
		{input: "abcdefghijk", maxLen: 9, want: "abc...ijk"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.input, tc.maxLen), func(t *testing.T) {
			got := stringz.EllipsifyASCII(tc.input, tc.maxLen)
			require.True(t, len(got) <= tc.maxLen)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{in: "", want: ""},
		{in: " ", want: " "},
		{in: "a", want: "a"},
		{in: "a b", want: "a b"},
		{in: "a b c", want: "a b c"},
		{in: "a b c.txt", want: "a b c.txt"},
		{in: "conin$", want: "conin_"},
		{in: "a+b", want: "a+b"},
		{in: "some (file).txt", want: "some (file).txt"},
		{in: ".", want: "_"},
		{in: "..", want: "__"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			got := stringz.SanitizeFilename(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestTypeNames(t *testing.T) {
	errs := []error{errors.New("stdlib"), errz.New("errz")}
	names := stringz.TypeNames(errs...)
	require.Equal(t, []string{"*errors.errorString", "*errz.errz"}, names)

	a := []any{1, "hello", true, errs}
	names = stringz.TypeNames(a...)
	require.Equal(t, []string{"int", "string", "bool", "[]error"}, names)
}
