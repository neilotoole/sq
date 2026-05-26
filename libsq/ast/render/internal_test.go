package render

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnquoteLiteral(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		in      string
		wantVal string
		wantOk  bool
		wantErr string
	}{
		// Unquoted: returned verbatim with ok=false.
		{in: ``, wantVal: ``, wantOk: false},
		{in: `42`, wantVal: `42`, wantOk: false},

		// Quoted plain literals.
		{in: `""`, wantVal: ``, wantOk: true},
		{in: `"foo"`, wantVal: `foo`, wantOk: true},
		{in: `"O'Brien"`, wantVal: `O'Brien`, wantOk: true},

		// JSON-style escapes per grammar/SLQ.g4 STRING/ESC.
		{in: `"a\nb"`, wantVal: "a\nb", wantOk: true},
		{in: `"a\tb"`, wantVal: "a\tb", wantOk: true},
		{in: `"a\rb"`, wantVal: "a\rb", wantOk: true},
		{in: `"a\bb"`, wantVal: "a\bb", wantOk: true},
		{in: `"a\fb"`, wantVal: "a\fb", wantOk: true},
		{in: `"\""`, wantVal: `"`, wantOk: true},
		{in: `"\\"`, wantVal: `\`, wantOk: true},
		{in: `"\/"`, wantVal: `/`, wantOk: true},
		{in: `"aéb"`, wantVal: "aéb", wantOk: true}, // é = U+00E9
		{in: `"你好"`, wantVal: "你好", wantOk: true},

		// UTF-16 surrogate pairs (JSON semantics): a high+low pair combines
		// into one astral codepoint; an unpaired surrogate decodes to U+FFFD.
		{in: `"\uD83D\uDE00"`, wantVal: "\U0001F600", wantOk: true}, // 😀 via surrogate pair
		{in: `"\uD834\uDD1E"`, wantVal: "\U0001D11E", wantOk: true}, // 𝄞 via surrogate pair
		{in: `"\uD800"`, wantVal: "�", wantOk: true},                // lone high surrogate
		{in: `"\uDC00"`, wantVal: "�", wantOk: true},                // lone low surrogate
		{in: `"\uD800x"`, wantVal: "�x", wantOk: true},              // high surrogate, no pair follows
		{in: `"\uD83DA"`, wantVal: "�A", wantOk: true},              // high surrogate + non-low-surrogate escape

		// Malformed.
		{in: `"abc`, wantErr: "malformed literal"},
		{in: `"\"`, wantErr: "dangling backslash"},
		{in: `"\x"`, wantErr: `invalid escape \x`},
		{in: `"\u12"`, wantErr: `short \u escape`},
		{in: `"\uZZZZ"`, wantErr: `invalid \u escape`},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			gotVal, gotOk, gotErr := unquoteLiteral(tc.in)
			if tc.wantErr != "" {
				require.ErrorContains(t, gotErr, tc.wantErr)
				return
			}
			require.NoError(t, gotErr)
			require.Equal(t, tc.wantVal, gotVal)
			require.Equal(t, tc.wantOk, gotOk)
		})
	}
}
