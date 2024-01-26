package downloader

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
)

const (
	msgCloseCacheHeaderFile = "Close cached response header file"
	msgCloseCacheBodyFile   = "Close cached response body file"
	msgDeleteCache          = "Delete HTTP response cache"
)

// cache is a cache for an individual download. The cached response is
// stored in two files, one for the header and one for the body, with
// a checksum (of the body file) stored in a third file.
// Use cache.paths to get the cache file paths.
type cache struct {
	// dir is the directory in which the cache files are stored.
	// It is specific to a particular download. There are actually
	// two subdirectories of dir: "main" and "staging". The staging dir
	// is used to write new cache files, and on successful cache write,
	// it is swapped with the main dir. This two-step "atomic-write-lite"
	// exists so that a failed response read doesn't destroy the existing
	// cache.
	dir string
}

// paths returns the paths to the header, body, and checksum files for req.
// It is not guaranteed that they exist.
func (c *cache) paths(req *http.Request) (header, body, sum string) {
	mainDir := filepath.Join(c.dir, "main")
	if req == nil || req.Method == http.MethodGet {
		return filepath.Join(mainDir, "header"),
			filepath.Join(mainDir, "body"),
			filepath.Join(mainDir, "checksums.txt")
	}

	// This is probably not strictly necessary because we're always
	// using GET, but in an earlier incarnation of the code, it was relevant.
	// Can probably delete.
	return filepath.Join(mainDir, req.Method+"_header"),
		filepath.Join(mainDir, req.Method+"_body"),
		filepath.Join(mainDir, req.Method+"_checksums.txt")
}

// exists returns true if the cache exists and is consistent.
// If it's inconsistent, it will be automatically cleared.
// See also: clearIfInconsistent.
func (c *cache) exists(req *http.Request) bool {
	if err := c.clearIfInconsistent(req); err != nil {
		lg.FromContext(req.Context()).Error("Failed to clear inconsistent cache",
			lga.Err, err, lga.Dir, c.dir)
		return false
	}

	fpHeader, _, _ := c.paths(req)
	fi, err := os.Stat(fpHeader)
	if err != nil {
		return false
	}

	if fi.Size() == 0 {
		return false
	}

	_, ok := c.checksumsMatch(req)
	return ok
}

// clearIfInconsistent deletes the cache if it is inconsistent.
func (c *cache) clearIfInconsistent(req *http.Request) error {
	mainDir := filepath.Join(c.dir, "main")
	if !ioz.DirExists(mainDir) {
		return nil
	}

	entries, err := ioz.ReadDir(mainDir, false, false, false)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		// If it's an empty cache, that's consistent.
		return nil
	}

	// We know that there's at least one file in the cache.
	// To be consistent, all three cache files must exist.
	inconsistent := false
	fpHeader, fpBody, fpChecksum := c.paths(req)
	for _, fp := range []string{fpHeader, fpBody, fpChecksum} {
		if !ioz.FileAccessible(fp) {
			inconsistent = true
			break
		}
	}

	if !inconsistent {
		// All three cache files exist. Verify that checksums match.
		if _, ok := c.checksumsMatch(req); !ok {
			inconsistent = true
		}
	}

	if inconsistent {
		lg.FromContext(req.Context()).Warn("Deleting inconsistent cache", lga.Dir, c.dir)
		return c.clear(req.Context())
	}
	return nil
}

// Get returns the cached http.Response for req if present, and nil
// otherwise. The caller MUST close the returned response body.
func (c *cache) get(ctx context.Context, req *http.Request) (*http.Response, error) {
	log := lg.FromContext(ctx)

	fpHeader, fpBody, _ := c.paths(req)
	if !ioz.FileAccessible(fpHeader) {
		// If the header file doesn't exist, it's a nil, nil situation.
		return nil, nil //nolint:nilnil
	}

	if _, ok := c.checksumsMatch(req); !ok {
		// If the checksums don't match, it's a nil, nil situation.

		// REVISIT: should we clear the cache here?
		return nil, nil //nolint:nilnil
	}

	headerBytes, err := os.ReadFile(fpHeader)
	if err != nil {
		return nil, errz.Wrap(err, "failed to read cached response header file")
	}

	bodyFile, err := os.Open(fpBody)
	if err != nil {
		log.Error("Failed to open cached response body file",
			lga.File, fpBody, lga.Err, err)
		return nil, errz.Wrap(err, "failed to open cached response body file")
	}

	// Now it's time for the Matroyshka readers. First we concatenate the
	// header and body via io.MultiReader. Then, we wrap that in
	// a contextio.NewReader, for context-awareness. Finally,
	// http.ReadResponse requires a bufio.Reader, so we wrap the
	// context reader via bufio.NewReader, and then we're ready to go.
	r := contextio.NewReader(ctx, io.MultiReader(bytes.NewReader(headerBytes), bodyFile))
	resp, err := http.ReadResponse(bufio.NewReader(r), req)
	if err != nil {
		lg.WarnIfCloseError(log, msgCloseCacheBodyFile, bodyFile)
		return nil, errz.Err(err)
	}

	// We need to explicitly close bodyFile. To do this (on the happy path),
	// we wrap bodyFile in a ReadCloserNotifier, which will close bodyFile
	// when resp.Body is closed. Thus, it's critical that the caller
	// close the returned resp.
	resp.Body = ioz.ReadCloserNotifier(resp.Body, func(error) {
		lg.WarnIfCloseError(log, msgCloseCacheBodyFile, bodyFile)
	})
	return resp, nil
}

// checksum returns the contents of the cached checksum file, if available.
func (c *cache) cachedChecksum(req *http.Request) (sum checksum.Checksum, ok bool) {
	_, _, fp := c.paths(req)
	if !ioz.FileAccessible(fp) {
		return "", false
	}

	sums, err := checksum.ReadFile(fp)
	if err != nil {
		lg.FromContext(req.Context()).Warn("Failed to read checksum file",
			lga.File, fp, lga.Err, err)
		return "", false
	}

	if len(sums) != 1 {
		// Shouldn't happen.
		return "", false
	}

	sum, ok = sums["body"]
	return sum, ok
}

// checksumsMatch returns true (and the valid checksum) if there is a cached
// checksum file for req, and there is a cached response body file, and a fresh
// checksum calculated from that body file matches the cached checksum.
func (c *cache) checksumsMatch(req *http.Request) (sum checksum.Checksum, ok bool) { //nolint:unparam
	sum, ok = c.cachedChecksum(req)
	if !ok {
		return "", false
	}

	_, fpBody, _ := c.paths(req)
	calculatedSum, err := checksum.ForFile(fpBody)
	if err != nil {
		return "", false
	}

	if calculatedSum != sum {
		lg.FromContext(req.Context()).Warn("Inconsistent cache: checksums don't match", lga.Dir, c.dir)
		return "", false
	}

	return sum, true
}

// clear deletes the cache entries from disk.
func (c *cache) clear(ctx context.Context) error {
	deleteErr := errz.Wrap(os.RemoveAll(c.dir), "delete cache dir")
	recreateErr := ioz.RequireDir(c.dir)
	err := errz.Append(deleteErr, recreateErr)
	if err != nil {
		lg.FromContext(ctx).Error(msgDeleteCache,
			lga.Dir, c.dir, lga.Err, err)
		return err
	}

	lg.FromContext(ctx).Info("Deleted cache dir", lga.Dir, c.dir)
	return nil
}

// write writes resp header and body to the cache, returning the number of
// body bytes written to disk.
//
// If headerOnly is true, only the header cache file is updated.
//
// A checksum file, computed from the body file, is also written to disk.
//
// If an error occurs while attempting to write to the cache, any existing
// cache artifacts are left untouched. It's a sort of atomic-write-lite.
// To achieve this, cache files are first written to a staging dir, and that
// staging dir is only swapped with the cache dir if there are no errors.
//
// The response body is always closed.
func (c *cache) write(ctx context.Context, resp *http.Response, headerOnly bool) (written int64, err error) {
	log := lg.FromContext(ctx)
	start := time.Now()
	var stagingDir string

	defer func() {
		lg.WarnIfCloseError(log, lgm.CloseHTTPResponseBody, resp.Body)

		if stagingDir != "" && ioz.DirExists(stagingDir) {
			lg.WarnIfError(log, "Remove cache staging dir", os.RemoveAll(stagingDir))
		}
	}()

	mainDir := filepath.Join(c.dir, "main")
	if err = ioz.RequireDir(mainDir); err != nil {
		return 0, err
	}

	stagingDir = filepath.Join(c.dir, "staging")
	if err = ioz.RequireDir(mainDir); err != nil {
		return 0, err
	}

	log.Debug("Writing HTTP response header to cache", lga.Dir, c.dir, lga.Resp, resp)
	headerBytes, err := httputil.DumpResponse(resp, false)
	if err != nil {
		return 0, errz.Err(err)
	}

	fpHeaderStaging := filepath.Join(stagingDir, "header")
	if _, err = ioz.WriteToFile(ctx, fpHeaderStaging, bytes.NewReader(headerBytes)); err != nil {
		return 0, err
	}

	fpHeader, fpBody, _ := c.paths(resp.Request)
	if headerOnly {
		// It's only the header that we're changing, so we don't need to
		// swap the entire staging dir, just the header file.
		if err = os.Rename(fpHeaderStaging, fpHeader); err != nil {
			return 0, errz.Wrap(err, "failed to move staging cache header file")
		}

		return 0, nil
	}

	fpBodyStaging := filepath.Join(stagingDir, "body")
	if written, err = ioz.WriteToFile(ctx, fpBodyStaging, resp.Body); err != nil {
		log.Error("Cache write: failed to write cache body file", lga.Err, err, lga.Path, fpBodyStaging)
		return 0, err
	}

	sum, err := checksum.ForFile(fpBodyStaging)
	if err != nil {
		return written, errz.Wrap(err, "failed to compute checksum for cache body file")
	}

	if err = checksum.WriteFile(filepath.Join(stagingDir, "checksums.txt"), sum, "body"); err != nil {
		return written, errz.Wrap(err, "failed to write checksum file for cache body")
	}

	// We've got good data in the staging dir. Now we do the switcheroo.
	if err = ioz.RenameDir(stagingDir, mainDir); err != nil {
		return 0, errz.Wrap(err, "failed to write download cache")
	}

	stagingDir = ""
	log.Info("Wrote HTTP response body to cache",
		lga.Written, written, lga.File, fpBody, lga.Elapsed, time.Since(start).Round(time.Millisecond))
	return written, nil
}
