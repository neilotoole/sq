package termz

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsTerminal verifies IsTerminal for the non-terminal cases. A non-*os.File
// writer hits the default branch, and a regular file is an *os.File that is not
// a terminal. A real terminal cannot be synthesized here, so the true path is
// only exercised by interactive use.
func TestIsTerminal(t *testing.T) {
	// Non-*os.File writer hits the default branch.
	require.False(t, IsTerminal(&bytes.Buffer{}))

	// A regular file is an *os.File, but not a terminal.
	f, err := os.CreateTemp(t.TempDir(), "termz")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	require.False(t, IsTerminal(f))

	// A nil writer must not panic, and is not a terminal.
	require.False(t, IsTerminal(nil))

	// A typed-nil *os.File must not panic either: (*os.File).Fd() is nil-safe
	// (it returns ^uintptr(0)), so this resolves to a non-terminal fd.
	var nilFile *os.File
	require.NotPanics(t, func() { require.False(t, IsTerminal(nilFile)) })
}

// TestIsColorTerminal_FileWriter verifies that, absent any env override, a
// regular *os.File (not a terminal) is not reported as a color terminal. This
// exercises the terminal-detection fall-through that the buffer-based cases in
// TestIsColorTerminal_EnvOverrides never reach.
func TestIsColorTerminal_FileWriter(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("TERM", "")

	f, err := os.CreateTemp(t.TempDir(), "termz")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	require.False(t, IsColorTerminal(f))

	// A typed-nil *os.File must not panic: the nil-interface guard does not
	// catch it, but (*os.File).Fd() is nil-safe and resolves to a non-terminal.
	var nilFile *os.File
	require.NotPanics(t, func() { require.False(t, IsColorTerminal(nilFile)) })
}

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
		// An empty-but-present FORCE_COLOR= is deliberately treated as unset:
		// it falls through to terminal detection, which is false for a buffer.
		{name: "force_color_empty", forceColor: "", want: false},
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
