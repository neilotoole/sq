package updatecheck

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/cli/config/yamlstore"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/files"
)

func TestCacheDirForRun_withoutFiles(t *testing.T) {
	t.Parallel()

	const loc = "/tmp/sq-test-config/sq.yml"
	store := &yamlstore.Store{Path: loc}
	ru := &run.Run{ConfigStore: store}
	got := CacheDirForRun(ru)
	require.Equal(t, filepath.Join(files.DefaultCacheDir(), checksum.Sum([]byte(store.Location()))), got)
}

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
	t.Parallel()

	latest, err := fetchBrewVersion(context.Background())
	require.NoError(t, err)
	require.True(t, semver.IsValid("v"+latest))
}

func TestCachedLatest_andTTL(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	require.False(t, isCacheFresh(cacheDir))

	latest, ok := CachedLatest(cacheDir)
	require.False(t, ok)
	require.Empty(t, latest)

	require.NoError(t, writeCache(cacheDir, Cache{
		LatestVersion: "v9.9.9",
		CheckedAt:     time.Now().UTC(),
	}))

	latest, ok = CachedLatest(cacheDir)
	require.True(t, ok)
	require.Equal(t, "v9.9.9", latest)
	require.True(t, isCacheFresh(cacheDir))

	stale := time.Now().UTC().Add(-defaultTTL - time.Minute)
	require.NoError(t, writeCache(cacheDir, Cache{
		LatestVersion: "v9.9.8",
		CheckedAt:     stale,
	}))
	require.False(t, isCacheFresh(cacheDir))
}

func TestNormalizeVersion(t *testing.T) {
	t.Parallel()

	got, err := NormalizeVersion("1.2.3")
	require.NoError(t, err)
	require.Equal(t, "v1.2.3", got)

	got, err = NormalizeVersion("v1.2.3")
	require.NoError(t, err)
	require.Equal(t, "v1.2.3", got)

	_, err = NormalizeVersion("not-a-version")
	require.Error(t, err)
}

func isCacheFresh(cacheDir string) bool {
	c, ok := readCache(cacheDir)
	return ok && time.Since(c.CheckedAt) < defaultTTL
}

func TestWriteCache_roundTrip(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	require.NoError(t, writeCache(cacheDir, Cache{
		LatestVersion: "v0.54.0",
		CheckedAt:     time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
	}))

	b, err := os.ReadFile(filepath.Join(cacheDir, cacheFileName))
	require.NoError(t, err)

	var got Cache
	require.NoError(t, json.Unmarshal(b, &got))
	require.Equal(t, "v0.54.0", got.LatestVersion)
	require.Equal(t, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC), got.CheckedAt)
}
