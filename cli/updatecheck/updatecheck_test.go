package updatecheck

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/testh/tu"
)

func TestVersionFromBrewFormula(t *testing.T) {
	f, err := os.ReadFile(filepath.Join("..", "testdata", "sq-0.20.0.rb"))
	require.NoError(t, err)

	vers, err := versionFromBrewFormula(f)
	require.NoError(t, err)
	require.Equal(t, "0.20.0", vers)
}

func TestVersionFromBrewFormula_URLBased(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		wantVer string
		wantErr bool
	}{
		{
			name: "homebrew-core_tar.gz_format",
			input: `class Sq < Formula
  desc "swiss-army knife for data"
  homepage "https://sq.io"
  url "https://github.com/neilotoole/sq/archive/refs/tags/v0.48.3.tar.gz"
  sha256 "abc123"
  license "MIT"

  bottle do
`,
			wantVer: "0.48.3",
		},
		{
			name: "homebrew-core_zip_format",
			input: `class Sq < Formula
  url "https://github.com/neilotoole/sq/archive/refs/tags/v1.2.3.zip"
  sha256 "abc123"

  bottle do
`,
			wantVer: "1.2.3",
		},
		{
			name: "invalid_semver_in_URL",
			input: `class Sq < Formula
  url "https://github.com/neilotoole/sq/archive/refs/tags/vnotvalid.tar.gz"

  bottle do
`,
			wantErr: true,
		},
		{
			name: "explicit_version_before_url",
			input: `class Sq < Formula
  version "0.50.0"
  url "https://github.com/neilotoole/sq/archive/refs/tags/v0.48.3.tar.gz"

  bottle do
`,
			wantVer: "0.50.0",
		},
		{
			name: "explicit_version_after_url",
			input: `class Sq < Formula
  url "https://github.com/neilotoole/sq/archive/refs/tags/v0.48.3.tar.gz"
  version "0.50.0"

  bottle do
`,
			wantVer: "0.50.0",
		},
		{
			name: "unrecognized_extension_falls_through_to_explicit_version",
			input: `class Sq < Formula
  url "https://github.com/neilotoole/sq/archive/refs/tags/v1.0.0.tar.xz"
  version "0.48.11"

  bottle do
`,
			wantVer: "0.48.11",
		},
		{
			name: "no_version_found",
			input: `class Sq < Formula
  desc "swiss-army knife for data"

  bottle do
`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vers, err := versionFromBrewFormula([]byte(tc.input))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantVer, vers)
		})
	}
}

func TestFetchBrewVersion(t *testing.T) {
	tu.SkipNoNetwork(t)
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	latest, err := fetchBrewVersion(ctx)
	require.NoError(t, err)
	require.True(t, semver.IsValid("v"+latest))
}

func TestNormalizeVersion(t *testing.T) {
	t.Parallel()

	got, err := normalizeVersion("1.2.3")
	require.NoError(t, err)
	require.Equal(t, "v1.2.3", got)

	got, err = normalizeVersion("v1.2.3")
	require.NoError(t, err)
	require.Equal(t, "v1.2.3", got)

	_, err = normalizeVersion("not-a-version")
	require.Error(t, err)
}

func TestReadCache_emptyDir(t *testing.T) {
	t.Parallel()

	_, ok := readCache("")
	require.False(t, ok)
}

func TestReadFreshCache_stale(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	require.NoError(t, writeCache(cacheDir, cacheEntry{
		LatestVersion: "v9.9.9",
		CheckedAt:     time.Now().UTC().Add(-defaultCacheTTL - time.Minute),
	}))

	latest, ok := readFreshCache(cacheDir)
	require.False(t, ok)
	require.Empty(t, latest)
}

func TestWriteCache_roundTrip(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	require.NoError(t, writeCache(cacheDir, cacheEntry{
		LatestVersion: "v0.54.0",
		CheckedAt:     time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
	}))

	got, ok := readCache(cacheDir)
	require.True(t, ok)
	require.Equal(t, "v0.54.0", got.LatestVersion)
	require.Equal(t, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC), got.CheckedAt)
}

// blockingFetch returns a fetch func that ignores ctx and blocks until the
// test ends, so that only the timeout branch of fetchLatestWithWait can fire.
// A fetch that returned ctx.Err() on cancellation would race the timeout
// branch against the fetch-error branch, and both read the cache.
func blockingFetch(t *testing.T) func(context.Context) (string, error) {
	t.Helper()
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	return func(context.Context) (string, error) {
		<-release
		return "", errors.New("released")
	}
}

func TestFetchLatestWithWait(t *testing.T) {
	t.Parallel()

	fresh := cacheEntry{LatestVersion: "v9.9.9", CheckedAt: time.Now().UTC()}
	stale := cacheEntry{
		LatestVersion: "v9.9.9",
		CheckedAt:     time.Now().UTC().Add(-defaultCacheTTL - time.Minute),
	}
	fetchErr := func(context.Context) (string, error) { return "", errors.New("boom") }
	fetchInvalid := func(context.Context) (string, error) { return "not-a-version", nil }

	testCases := []struct {
		name    string
		cache   *cacheEntry
		fetch   func(t *testing.T) func(context.Context) (string, error)
		timeout time.Duration
		want    string
	}{
		{
			name:    "timeout_fresh_cache",
			timeout: 10 * time.Millisecond,
			cache:   &fresh,
			fetch:   blockingFetch,
			want:    "v9.9.9",
		},
		{
			name:    "timeout_stale_cache",
			timeout: 10 * time.Millisecond,
			cache:   &stale,
			fetch:   blockingFetch,
			want:    "",
		},
		{
			name:    "timeout_no_cache",
			timeout: 10 * time.Millisecond,
			fetch:   blockingFetch,
			want:    "",
		},
		{
			name:  "fetch_error_fresh_cache",
			cache: &fresh,
			fetch: func(*testing.T) func(context.Context) (string, error) { return fetchErr },
			want:  "v9.9.9",
		},
		{
			name:  "fetch_error_no_cache",
			fetch: func(*testing.T) func(context.Context) (string, error) { return fetchErr },
			want:  "",
		},
		{
			name:  "invalid_version_fresh_cache",
			cache: &fresh,
			fetch: func(*testing.T) func(context.Context) (string, error) { return fetchInvalid },
			want:  "v9.9.9",
		},
		{
			name:  "invalid_version_no_cache",
			fetch: func(*testing.T) func(context.Context) (string, error) { return fetchInvalid },
			want:  "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cacheDir := t.TempDir()
			if tc.cache != nil {
				require.NoError(t, writeCache(cacheDir, *tc.cache))
			}

			timeout := tc.timeout
			if timeout == 0 {
				timeout = time.Second
			}

			got := fetchLatestWithWait(context.Background(), cacheDir, timeout, tc.fetch(t))
			require.Equal(t, tc.want, got)
		})
	}
}

func TestFetchLatestWithWait_writesCacheOnSuccess(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	before := time.Now().UTC()

	got := fetchLatestWithWait(context.Background(), cacheDir, time.Second,
		func(context.Context) (string, error) { return "0.60.0", nil })
	require.Equal(t, "v0.60.0", got)

	c, ok := readCache(cacheDir)
	require.True(t, ok)
	require.Equal(t, "v0.60.0", c.LatestVersion)
	require.False(t, c.CheckedAt.Before(before))
}

func TestFetchLatestWithWait_noCacheDir(t *testing.T) {
	t.Parallel()

	got := fetchLatestWithWait(context.Background(), "", time.Second,
		func(context.Context) (string, error) { return "0.60.0", nil })
	require.Equal(t, "v0.60.0", got)
}
