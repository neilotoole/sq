// Command gentestdata regenerates the Sakila fixtures published at
// sq.io/testdata (site/static/testdata) from the canonical in-repo fixtures,
// so the published copies carry the same faithful data (restored accents, real
// phone numbers, original 2006 timestamps) as the rest of the Sakila fixtures.
// Run from the repo root:
//
//	go run ./test/fixtures/internal/gentestdata
//
// Plain files are copied verbatim. The two tarballs are rebuilt from the
// canonical CSV and TSV dirs, deterministically: members sorted by name, a
// fixed mtime, zeroed uid/gid and owner names, and a gzip header carrying
// neither a timestamp nor a filename. Re-running on an unchanged tree
// therefore reproduces the same bytes, so a rebuild shows up in git only when
// the data actually changed.
//
// The previous tarballs were built by hand on macOS in 2020 and carried
// AppleDouble resource-fork members (sakila-csv/._actor.csv), which appear as
// stray files when extracted on Linux or Windows. Building them here avoids
// that.
//
// test/fixtures guards the output: if the published copies drift from the
// canonical fixtures, the guard test fails and names this command.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/neilotoole/sq/test/fixtures/internal/fixtdata"
	"github.com/neilotoole/sq/testh/proj"
)

// fixedModTime is the mtime stamped on every tarball member. Its value is not
// meaningful beyond being constant: it is the Sakila actor.last_update
// timestamp, chosen so the archives are reproducible.
var fixedModTime = time.Date(2006, time.February, 15, 4, 34, 33, 0, time.UTC)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gentestdata:", err)
		os.Exit(1)
	}
}

func run() error {
	publishedDir := proj.Abs(fixtdata.PublishedDir)
	if _, err := os.Stat(publishedDir); err != nil {
		return fmt.Errorf("published dir: %w", err)
	}

	for _, name := range slices.Sorted(maps.Keys(fixtdata.Files)) {
		canonical := fixtdata.Files[name]

		b, err := os.ReadFile(proj.Abs(canonical))
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		//nolint:gosec // G703: name comes from fixtdata.Files, an in-repo constant map.
		if err = os.WriteFile(filepath.Join(publishedDir, name), b, 0o600); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}

		fmt.Fprintf(os.Stdout, "wrote %s from %s\n", filepath.Join(fixtdata.PublishedDir, name), canonical)
	}

	for _, name := range slices.Sorted(maps.Keys(fixtdata.Tarballs)) {
		canonicalDir := fixtdata.Tarballs[name]

		b, err := buildTarball(proj.Abs(canonicalDir))
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if err = os.WriteFile(filepath.Join(publishedDir, name), b, 0o600); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}

		fmt.Fprintf(os.Stdout, "wrote %s from %s\n", filepath.Join(fixtdata.PublishedDir, name), canonicalDir)
	}

	return nil
}

// buildTarball returns a gzipped tar of the regular files directly under
// srcDir. The archive holds a root dir entry named for srcDir, then that dir's
// files in name order.
func buildTarball(srcDir string) ([]byte, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	slices.Sort(names)

	buf := &bytes.Buffer{}

	// gzip.Writer omits the header timestamp and filename while Header.ModTime
	// is the zero time and Header.Name is empty, which is how it starts out.
	gz, err := gzip.NewWriterLevel(buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}

	root := filepath.Base(srcDir)
	tw := tar.NewWriter(gz)

	if err = tw.WriteHeader(&tar.Header{
		Name:     root + "/",
		Typeflag: tar.TypeDir,
		Mode:     0o755,
		ModTime:  fixedModTime,
		Format:   tar.FormatUSTAR,
	}); err != nil {
		return nil, err
	}

	for _, name := range names {
		var b []byte
		if b, err = os.ReadFile(filepath.Join(srcDir, name)); err != nil {
			return nil, err
		}

		if err = tw.WriteHeader(&tar.Header{
			Name:     root + "/" + name,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(b)),
			ModTime:  fixedModTime,
			Format:   tar.FormatUSTAR,
		}); err != nil {
			return nil, err
		}

		if _, err = tw.Write(b); err != nil {
			return nil, err
		}
	}

	if err = tw.Close(); err != nil {
		return nil, err
	}
	if err = gz.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
