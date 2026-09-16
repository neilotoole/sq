// Package fixtures guards the Sakila fixtures published at sq.io/testdata
// (site/static/testdata) against drifting from the canonical in-repo fixtures
// they are copied from.
//
// The published copies exist so the docs can tell a reader to download a file,
// but they hold the same data as the driver test fixtures. Keeping the two in
// sync by hand did not work: #949 restored faithful Sakila data across the
// repo and missed site/static entirely, leaving the published copies on 2020
// timestamps and without the restored Unicode accents (#1207).
package fixtures

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/test/fixtures/internal/fixtdata"
	"github.com/neilotoole/sq/testh/proj"
)

// TestPublishedFilesMatchCanonical verifies that every published Sakila
// fixture is byte-identical to the in-repo fixture it is copied from.
func TestPublishedFilesMatchCanonical(t *testing.T) {
	for _, name := range slices.Sorted(maps.Keys(fixtdata.Files)) {
		canonical := fixtdata.Files[name]

		t.Run(name, func(t *testing.T) {
			gotPublished, err := os.ReadFile(proj.Abs(filepath.Join(fixtdata.PublishedDir, name)))
			require.NoError(t, err)

			wantCanonical, err := os.ReadFile(proj.Abs(canonical))
			require.NoError(t, err)

			require.True(t, bytes.Equal(wantCanonical, gotPublished),
				"%s/%s has drifted from %s: run %s",
				fixtdata.PublishedDir, name, canonical, fixtdata.GenCmd)
		})
	}
}

// TestPublishedTarballsMatchCanonical verifies that each published tarball
// packs exactly the canonical dir's files, with identical contents. It
// compares the archive members rather than the tarball bytes, so that a gzip
// or tar encoding change is not reported as fixture drift.
func TestPublishedTarballsMatchCanonical(t *testing.T) {
	for _, name := range slices.Sorted(maps.Keys(fixtdata.Tarballs)) {
		canonicalDir := fixtdata.Tarballs[name]

		t.Run(name, func(t *testing.T) {
			gotMembers := readTarball(t, proj.Abs(filepath.Join(fixtdata.PublishedDir, name)))
			wantMembers := readDir(t, proj.Abs(canonicalDir), filepath.Base(canonicalDir))

			require.Equal(t, slices.Sorted(maps.Keys(wantMembers)), slices.Sorted(maps.Keys(gotMembers)),
				"%s packs a different set of files than %s: run %s",
				name, canonicalDir, fixtdata.GenCmd)

			for member, want := range wantMembers {
				require.True(t, bytes.Equal(want, gotMembers[member]),
					"%s: member %s has drifted from %s: run %s",
					name, member, canonicalDir, fixtdata.GenCmd)
			}
		})
	}
}

// readTarball returns the regular-file members of the gzipped tarball at
// path, keyed by their in-archive name.
func readTarball(t *testing.T, path string) map[string][]byte {
	t.Helper()

	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	gz, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer func() { require.NoError(t, gz.Close()) }()

	members := map[string][]byte{}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		b, err := io.ReadAll(tr)
		require.NoError(t, err)
		members[hdr.Name] = b
	}

	return members
}

// readDir returns the regular files directly under dir, keyed by prefix plus
// the file name, matching the layout of the published tarballs.
func readDir(t *testing.T, dir, prefix string) map[string][]byte {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	members := map[string][]byte{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		b, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		require.NoError(t, err)
		members[prefix+"/"+entry.Name()] = b
	}

	return members
}
