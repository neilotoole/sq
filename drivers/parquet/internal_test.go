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
			// For URLs, the query string belongs to the URL itself, not to sq.
			// Connection options on remote sources must come via env or config.
			name:     "https_query_belongs_to_url",
			loc:      "https://example.com/data.parquet?threads=2",
			wantPath: "https://example.com/data.parquet?threads=2",
		},
		{
			// Presigned S3 URLs carry signature/expiry as query parameters that
			// must reach read_parquet() intact; stripping them breaks auth.
			name:     "s3_presigned_url_preserved",
			loc:      "https://bucket.s3.amazonaws.com/k.parquet?X-Amz-Signature=abc&X-Amz-Expires=900",
			wantPath: "https://bucket.s3.amazonaws.com/k.parquet?X-Amz-Signature=abc&X-Amz-Expires=900",
		},
		{
			name:     "s3_scheme_url_preserved",
			loc:      "s3://bucket/k.parquet?region=us-east-1",
			wantPath: "s3://bucket/k.parquet?region=us-east-1",
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

func TestIsNonHTTPRemote(t *testing.T) {
	testCases := []struct {
		loc  string
		want bool
	}{
		{"/abs/path/to.parquet", false},
		{"./rel.parquet", false},
		{"http://example.com/x.parquet", false},
		{"https://example.com/x.parquet", false},
		{"https://example.com/x.parquet?X-Amz-Signature=abc", false},
		{"s3://bucket/k.parquet", true},
		{"gs://bucket/k.parquet", true},
		{"r2://bucket/k.parquet", true},
		{"azure://account/c/k.parquet", true},
		{"abfss://container@account/k.parquet", true},
	}
	for _, tc := range testCases {
		t.Run(tc.loc, func(t *testing.T) {
			require.Equal(t, tc.want, isNonHTTPRemote(tc.loc))
		})
	}
}

func TestUpdateSlidingWindow(t *testing.T) {
	t.Run("fills_then_slides_across_iterations", func(t *testing.T) {
		var window [4]byte
		have := 0

		updateSlidingWindow(&window, &have, []byte{1, 2})
		require.Equal(t, 2, have)
		require.Equal(t, []byte{1, 2, 0, 0}, window[:])

		updateSlidingWindow(&window, &have, []byte{3})
		require.Equal(t, 3, have)
		require.Equal(t, []byte{1, 2, 3, 0}, window[:])

		updateSlidingWindow(&window, &have, []byte{4, 5})
		require.Equal(t, 4, have)
		require.Equal(t, []byte{2, 3, 4, 5}, window[:])

		updateSlidingWindow(&window, &have, []byte{6})
		require.Equal(t, 4, have)
		require.Equal(t, []byte{3, 4, 5, 6}, window[:])
	})

	t.Run("chunk_at_least_window_overwrites", func(t *testing.T) {
		window := [4]byte{9, 9, 9, 9}
		have := 4
		updateSlidingWindow(&window, &have, []byte{1, 2, 3, 4, 5, 6})
		require.Equal(t, 4, have)
		require.Equal(t, []byte{3, 4, 5, 6}, window[:])
	})

	t.Run("empty_chunk_is_noop", func(t *testing.T) {
		window := [4]byte{1, 2, 3, 4}
		have := 4
		updateSlidingWindow(&window, &have, nil)
		require.Equal(t, 4, have)
		require.Equal(t, []byte{1, 2, 3, 4}, window[:])
	})
}
