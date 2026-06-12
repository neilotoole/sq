package sqlparser

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/tu"
)

// TestTrimIdentQuotes verifies that trimIdentQuotes strips each of SQLite's
// four legal identifier-quote styles and collapses the doubled escape
// sequence for the three styles that have one: a doubled quote character
// inside double quotes, single quotes, or backticks denotes a literal
// occurrence of that character. Square brackets have no escape mechanism
// in SQLite (a ] cannot appear inside [..]), so bracket content is
// returned verbatim. See issue #789.
func TestTrimIdentQuotes(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		// Unquoted and degenerate inputs pass through unchanged.
		{in: "", want: ""},
		{in: "a", want: "a"},
		{in: "actor", want: "actor"},
		{in: `"`, want: `"`},
		{in: `"actor`, want: `"actor`},
		{in: `actor"`, want: `actor"`},
		{in: `[actor`, want: `[actor`},

		// Plain quoted forms.
		{in: `"actor"`, want: "actor"},
		{in: `'actor'`, want: "actor"},
		{in: "`actor`", want: "actor"},
		{in: `[actor]`, want: "actor"},

		// Doubled-escape collapse per quote style.
		{in: `"my""col"`, want: `my"col`},
		{in: `'my''col'`, want: `my'col`},
		{in: "`my``col`", want: "my`col"},
		{in: `"a""b""c"`, want: `a"b"c`},
		{in: `""""`, want: `"`},
		{in: `''''`, want: `'`},

		// The empty quoted identifier.
		{in: `""`, want: ""},
		{in: `''`, want: ""},
		{in: "``", want: ""},
		{in: `[]`, want: ""},

		// A quote char of a different style inside the quoted body is
		// literal, not an escape.
		{in: `"it's"`, want: "it's"},
		{in: `'he said "hi"'`, want: `he said "hi"`},
		{in: `[my"col]`, want: `my"col`},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			require.Equal(t, tc.want, trimIdentQuotes(tc.in))
		})
	}
}
