package jsonw

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
)

// TestEncodeString_MatchesJSONColor asserts that sq's own string encoder, used
// by the JSON record writer, escapes exactly the same way as the jsoncolor
// encoder used by writeJSON for inspect, error, config and ping output.
//
// sq has two JSON encoding paths, and they must not drift: the same input
// character has to render identically whichever sq command produced the JSON.
func TestEncodeString_MatchesJSONColor(t *testing.T) {
	var samples []string
	for c := 0; c < 0x80; c++ {
		samples = append(samples, fmt.Sprintf("a%sb", string(rune(c))))
	}
	samples = append(
		samples,
		"é", "中", "\U0001F600", // latin-1, CJK, emoji
		" ", " ", // line and paragraph separator
		"�",                  // replacement char
		string([]byte{0xff}), // invalid UTF-8
		"a<b>c&d",            // HTML-sensitive
		`quote" backslash\`,
	)

	pr := output.NewPrinting()
	pr.EnableColor(false)
	pr.Compact = true

	for _, s := range samples {
		sqOut, err := encodeString(nil, s, false)
		require.NoError(t, err)

		buf := &bytes.Buffer{}
		require.NoError(t, writeJSON(buf, pr, s))
		jcOut := bytes.TrimRight(buf.Bytes(), "\n")

		require.Equal(t, string(jcOut), string(sqOut),
			"encoders disagree for input %q", s)
	}
}
