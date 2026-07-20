package diffdoc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTerminateNewline(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want []byte
	}{
		{"nil", nil, nil},
		{"empty", []byte{}, []byte{}},
		{"single-newline", []byte("\n"), []byte("\n")},
		{"already-terminated", []byte("hello\n"), []byte("hello\n")},
		{"not-terminated", []byte("hello"), []byte("hello\n")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := terminateNewline(test.in)
			require.Equal(t, test.want, got)
		})
	}
}

// TestTerminateNewline_NoAlias verifies that terminating an unterminated input
// neither mutates it nor shares a backing array with the result. The input is
// given spare capacity so that a naive append(b, '\n') would write into the
// caller's backing array, which this test must catch.
func TestTerminateNewline_NoAlias(t *testing.T) {
	in := make([]byte, 5, 8)
	copy(in, "hello")

	got := terminateNewline(in)
	require.Equal(t, []byte("hello\n"), got)

	// The spare-capacity byte must be untouched: terminateNewline must copy
	// rather than append into b's backing array.
	require.Equal(t, byte(0), in[:6][5])

	// Mutating the result must not be visible through the input, proving the
	// two slices do not share a backing array.
	got[0] = 'x'
	require.Equal(t, []byte("hello"), in)
}
