package secret_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/secret"
)

func TestExtractRefs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []secret.Ref
	}{
		{
			name: "single keyring ref",
			in:   "postgres://alice:${keyring:my_db_pw}@db/sakila",
			want: []secret.Ref{{Scheme: "keyring", Path: "my_db_pw"}},
		},
		{
			name: "multiple refs across schemes",
			in:   "a ${keyring:x} and ${vault:y} and ${keyring:z}",
			want: []secret.Ref{
				{Scheme: "keyring", Path: "x"},
				{Scheme: "vault", Path: "y"},
				{Scheme: "keyring", Path: "z"},
			},
		},
		{
			name: "no placeholders",
			in:   "no placeholders here",
			want: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := secret.ExtractRefs(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestExtractRefs_Malformed(t *testing.T) {
	_, err := secret.ExtractRefs("${malformed")
	require.Error(t, err)
}
