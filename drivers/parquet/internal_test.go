package parquet

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/duckdb"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
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

// TestGripSourceMetadata_NonHTTPRemote verifies that SourceMetadata works
// for s3-style remote locations. location.Filename and files.Filesize treat
// such locations as local file paths and fail, so the grip must derive the
// name from the URL path and leave Size nil.
func TestGripSourceMetadata_NonHTTPRemote(t *testing.T) {
	ctx := context.Background()
	log := slog.New(slog.DiscardHandler)
	reg := driver.NewRegistry(log)
	reg.AddProvider(drivertype.DuckDB, &duckdb.Provider{Log: log})
	drvr, err := reg.DriverFor(drivertype.DuckDB)
	require.NoError(t, err)

	memSrc := &source.Source{
		Type:     drivertype.DuckDB,
		Handle:   "@remote_pq",
		Location: "duckdb://:memory:",
	}
	dbGrip, err := drvr.Open(ctx, memSrc, driver.ModeReadWrite)
	require.NoError(t, err)
	t.Cleanup(func() { _ = dbGrip.Close() })

	g := &grip{
		log: log,
		src: &source.Source{
			Type:     drivertype.Parquet,
			Handle:   "@remote",
			Location: "s3://bucket/dir/actor.parquet?region=us-east-1",
		},
		dbGrip: dbGrip,
	}

	md, err := g.SourceMetadata(ctx, true)
	require.NoError(t, err)
	require.Equal(t, drivertype.Parquet, md.Driver)
	require.Equal(t, "@remote", md.Handle)
	require.Equal(t, "s3://bucket/dir/actor.parquet?region=us-east-1", md.Location)
	require.Equal(t, "actor.parquet", md.Name)
	require.Equal(t, "actor.parquet", md.FQName)
	require.Nil(t, md.Size)
}

func TestRemoteFileName(t *testing.T) {
	testCases := []struct {
		loc  string
		want string
	}{
		{"s3://bucket/k.parquet", "k.parquet"},
		{"s3://bucket/dir/sub/k.parquet", "k.parquet"},
		{"s3://bucket/k.parquet?region=us-east-1", "k.parquet"},
		{"gs://bucket/k.parquet", "k.parquet"},
		{"abfss://container@account/k.parquet", "k.parquet"},
		{"s3://bucket", "s3://bucket"},
	}
	for _, tc := range testCases {
		t.Run(tc.loc, func(t *testing.T) {
			require.Equal(t, tc.want, remoteFileName(tc.loc))
		})
	}
}

// TestParseLocation_LiteralPathWithQuestionMark verifies that a local file
// whose name legally contains '?' is not split into path + DSN options: if
// the literal path exists on disk, it wins.
func TestParseLocation_LiteralPathWithQuestionMark(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "data?.parquet")
	require.NoError(t, os.WriteFile(fpath, []byte("PAR1"), 0o600))

	gotPath, gotDSN, err := parseLocation(fpath)
	require.NoError(t, err)
	require.Equal(t, fpath, gotPath)
	require.Empty(t, gotDSN)
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
