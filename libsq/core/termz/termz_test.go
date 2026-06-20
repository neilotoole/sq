package termz

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsColorTerminal_EnvOverrides verifies that IsColorTerminal honors the
// NO_COLOR, FORCE_COLOR, and TERM environment variables. A bytes.Buffer (not an
// *os.File, and never a terminal) is used as the writer, so the result is
// determined solely by the environment override.
func TestIsColorTerminal_EnvOverrides(t *testing.T) {
	testCases := []struct {
		name       string
		noColor    string
		forceColor string
		term       string
		want       bool
	}{
		{name: "no_env", want: false},
		{name: "no_color_set", noColor: "1", want: false},
		{name: "no_color_zero_still_disables", noColor: "0", want: false},
		{name: "no_color_false_still_disables", noColor: "false", want: false},
		{name: "no_color_wins_over_force", noColor: "1", forceColor: "1", want: false},
		{name: "force_color_1", forceColor: "1", want: true},
		{name: "force_color_2", forceColor: "2", want: true},
		{name: "force_color_true", forceColor: "true", want: true},
		{name: "force_color_zero", forceColor: "0", want: false},
		{name: "force_color_false", forceColor: "false", want: false},
		{name: "force_color_false_mixed_case", forceColor: "False", want: false},
		{name: "term_dumb", term: "dumb", want: false},
		{name: "force_color_wins_over_term_dumb", forceColor: "1", term: "dumb", want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tc.noColor)
			t.Setenv("FORCE_COLOR", tc.forceColor)
			t.Setenv("TERM", tc.term)

			buf := &bytes.Buffer{}
			require.Equal(t, tc.want, IsColorTerminal(buf))
		})
	}
}

// TestIsColorTerminal_NilWriter verifies that a nil writer is never reported as
// a color terminal once env overrides are excluded.
func TestIsColorTerminal_NilWriter(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("TERM", "")

	require.False(t, IsColorTerminal(nil))
}
