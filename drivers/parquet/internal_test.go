package parquet

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrw(t *testing.T) {
	// errw(nil) returns nil; errz.Wrap propagates the nil-in/nil-out
	// contract, and we depend on it at call sites.
	require.Nil(t, errw(nil))

	wrapped := errw(errors.New("underlying"))
	require.Error(t, wrapped)
	require.Contains(t, wrapped.Error(), "parquet")
}

func TestParseLocation(t *testing.T) {
	testCases := []struct {
		name       string
		loc        string
		wantPath   string
		wantDsnQry string
		wantErr    bool
	}{
		{
			name:     "plain_local",
			loc:      "/abs/path/events.parquet",
			wantPath: "/abs/path/events.parquet",
		},
		{
			name:       "local_with_options",
			loc:        "/abs/path/events.parquet?threads=4&memory_limit=2GB",
			wantPath:   "/abs/path/events.parquet",
			wantDsnQry: "threads=4&memory_limit=2GB",
		},
		{
			name:     "https_url",
			loc:      "https://example.com/data.parquet",
			wantPath: "https://example.com/data.parquet",
		},
		{
			name:       "https_with_options",
			loc:        "https://example.com/data.parquet?threads=2",
			wantPath:   "https://example.com/data.parquet",
			wantDsnQry: "threads=2",
		},
		{
			name:    "empty",
			loc:     "",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path, dsn, err := parseLocation(tc.loc)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantPath, path)
			require.Equal(t, tc.wantDsnQry, dsn)
		})
	}
}

func TestEscapeSingleQuotes(t *testing.T) {
	require.Equal(t, `'abc'`, "'"+escapeSingleQuotes("abc")+"'")
	require.Equal(t, `'a''b'`, "'"+escapeSingleQuotes("a'b")+"'")
	require.Equal(t, `''''`, "'"+escapeSingleQuotes("'")+"'")
}
