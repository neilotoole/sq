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
		{
			// gh #787: a path containing '}' truncates at the first '}',
			// leaving "rets/pw}" as literal text. The stray '}' must be a
			// parse error, not a silent misparse.
			name:    "truncated path with unbalanced brace",
			in:      "postgres://alice:${file:/run/sec}rets/pw}@db/sakila",
			wantErr: "unbalanced '}' at offset 40",
		},
		{
			// gh #787 regression guard: a '}' immediately following a
			// placeholder is literal text and must keep parsing as today.
			name: "placeholder then literal close brace",
			in:   "${env:X}}",
			want: []placeholder{{
				start: 0, end: 8, scheme: "env", path: "X",
			}},
		},
		{
			name: "placeholder then run of literal close braces",
			in:   "${env:X}}}",
			want: []placeholder{{
				start: 0, end: 8, scheme: "env", path: "X",
			}},
		},
		{
			name: "balanced braces after placeholder",
			in:   "${env:X}-{a}-b",
			want: []placeholder{{
				start: 0, end: 8, scheme: "env", path: "X",
			}},
		},
		{
			name: "placeholder wrapped in literal braces",
			in:   "{${env:X}}",
			want: []placeholder{{
				start: 1, end: 9, scheme: "env", path: "X",
			}},
		},
		{
			name:    "unbalanced brace between placeholders",
			in:      "${env:A}x}${env:B}",
			wantErr: "unbalanced '}' at offset 9",
		},
		{
			// A '}' before the first placeholder cannot be a truncation
			// artifact: it stays literal.
			name: "brace before first placeholder is literal",
			in:   "we}ird-${env:X}",
			want: []placeholder{{
				start: 7, end: 15, scheme: "env", path: "X",
			}},
		},
		{
			// With no placeholders at all, braces are always literal.
			name: "literal braces without placeholders",
			in:   "postgres://alice:hu}nter2@db/sakila",
			want: nil,
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
