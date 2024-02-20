package bytez_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/bytez"
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
			got := bytez.TerminateNewline(test.in)
			require.Equal(t, test.want, got)
		})
	}
}
