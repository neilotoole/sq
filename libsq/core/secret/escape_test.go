package secret

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEscape(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "no dollar", in: "postgres://alice:hunter2@db/sakila", want: "postgres://alice:hunter2@db/sakila"},
		{name: "lone dollar", in: "p$ss", want: "p$$ss"},
		{name: "malformed placeholder", in: "p${ss}w", want: "p$${ss}w"},
		{
			name: "well-formed placeholder",
			in:   "postgres://alice:${env:HOME}@db/sakila",
			want: "postgres://alice:$${env:HOME}@db/sakila",
		},
		{name: "preexisting double dollar", in: "p$$wd", want: "p$$$$wd"},
		{name: "triple dollar", in: "p$$$wd", want: "p$$$$$$wd"},
		{name: "dollar at end", in: "pass$", want: "pass$$"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Escape(tc.in)
			require.Equal(t, tc.want, got)

			// The escaped form must always parse cleanly with zero refs:
			// every '$' in the output is part of a '$$' escape pair.
			refs, err := ExtractRefs(got)
			require.NoError(t, err)
			require.Empty(t, refs)

			// Round-trip: unescaping the escaped form yields the input.
			require.Equal(t, tc.in, Unescape(got))
		})
	}
}

func TestUnescape(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "no dollar", in: "hunter2", want: "hunter2"},
		{name: "lone dollar untouched", in: "p$ss", want: "p$ss"},
		{name: "double dollar", in: "p$$ss", want: "p$ss"},
		{name: "escaped placeholder", in: "$${env:HOME}", want: "${env:HOME}"},
		{name: "quadruple dollar", in: "p$$$$ss", want: "p$$ss"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, Unescape(tc.in))
		})
	}
}
