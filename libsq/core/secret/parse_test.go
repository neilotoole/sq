package secret

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindPlaceholders(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []placeholder
		wantErr string
	}{
		{
			name: "none",
			in:   "postgres://alice:hunter2@db/sakila",
		},
		{
			name: "single password placeholder",
			in:   "postgres://alice:${keyring:my_db_pw}@db/sakila",
			want: []placeholder{{
				start: 17, end: 36, scheme: "keyring", path: "my_db_pw",
			}},
		},
		{
			name: "whole dsn placeholder",
			in:   "${keyring:@prod/dsn}",
			want: []placeholder{{
				start: 0, end: 20, scheme: "keyring", path: "@prod/dsn",
			}},
		},
		{
			name: "two placeholders",
			in:   "postgres://${keyring:user}:${keyring:pass}@db/sakila",
			want: []placeholder{
				{start: 11, end: 26, scheme: "keyring", path: "user"},
				{start: 27, end: 42, scheme: "keyring", path: "pass"},
			},
		},
		{
			name: "escape dollar sign",
			in:   "postgres://alice:has$$dollar@db/sakila",
			want: nil,
		},
		{
			name:    "unclosed placeholder",
			in:      "postgres://alice:${keyring:foo@db",
			wantErr: "unclosed",
		},
		{
			name:    "empty path",
			in:      "${keyring:}",
			wantErr: "empty path",
		},
		{
			name:    "empty scheme",
			in:      "${:foo}",
			wantErr: "empty scheme",
		},
		{
			name:    "invalid scheme chars",
			in:      "${Keyring:foo}",
			wantErr: "invalid scheme",
		},
		{
			name:    "no colon",
			in:      "${keyring}",
			wantErr: "missing ':' separator",
		},
		{
			name:    "whitespace in placeholder",
			in:      "${ keyring:foo }",
			wantErr: "invalid scheme",
		},
		{
			name: "path with slashes, colons, at-signs",
			in:   "${keyring:@grp/sub/name}",
			want: []placeholder{{
				start: 0, end: 24, scheme: "keyring", path: "@grp/sub/name",
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := findPlaceholders(tc.in)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
