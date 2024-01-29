package downloader

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"sync"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
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
		lg.FromContext(ctx).Error(msgDeleteCache, lga.Dir, c.dir, lga.Err, err)
		return err
	}

	lg.FromContext(ctx).Info("Deleted cache dir", lga.Dir, c.dir)
	return nil
}

// writeHeader updates the main cache header file from resp. The response body
// is not written to the cache, nor is resp.Body closed.
func (c *cache) writeHeader(ctx context.Context, resp *http.Response) (err error) {
	header, err := httputil.DumpResponse(resp, false)
	if err != nil {
		return errz.Err(err)
	}

	mainDir := filepath.Join(c.dir, "main")
	if err = ioz.RequireDir(mainDir); err != nil {
		return err
	}

	fp := filepath.Join(mainDir, "header")
	if _, err = ioz.WriteToFile(ctx, fp, bytes.NewReader(header)); err != nil {
		return err
	}

	lg.FromContext(ctx).Info("Updated download main cache (header only)", lga.Dir, mainDir, lga.Resp, resp)
	return nil
}

// newResponseCacher returns a new responseCacher for resp.
// On return, resp.Body will be nil.
func (c *cache) newResponseCacher(ctx context.Context, resp *http.Response) (*responseCacher, error) {
	defer func() { resp.Body = nil }()

	stagingDir := filepath.Join(c.dir, "staging")
	if err := ioz.RequireDir(stagingDir); err != nil {
		_ = resp.Body.Close()
		return nil, err
	}

	header, err := httputil.DumpResponse(resp, false)
	if err != nil {
		_ = resp.Body.Close()
		return nil, errz.Err(err)
	}

	if _, err = ioz.WriteToFile(ctx, filepath.Join(stagingDir, "header"), bytes.NewReader(header)); err != nil {
		_ = resp.Body.Close()
		return nil, err
	}

	log := lg.FromContext(ctx)
	log.Debug("Wrote response header to staging cache", lga.Dir, c.dir, lga.Resp, resp)

	var f *os.File
	if f, err = os.Create(filepath.Join(stagingDir, "body")); err != nil {
		_ = resp.Body.Close()
		return nil, err
	}

	r := &responseCacher{
		log:        log,
		stagingDir: stagingDir,
		mainDir:    filepath.Join(c.dir, "main"),
		body:       resp.Body,
		f:          f,
	}
	return r, nil
}

var _ io.ReadCloser = (*responseCacher)(nil)

// responseCacher is an io.ReadCloser that wraps an [http.Response.Body],
// appending bytes read via Read to a staging cache file, and then returning
// those same bytes to the caller. It is conceptually similar to [io.TeeReader].
// When Read receives [io.EOF] from the wrapped response body, the staging cache
// is promoted and replaces the main cache. If an error occurs during Read, the
// staging cache is discarded, and the main cache is left untouched. If an error
// occurs during cache promotion (which happens on receipt of [io.EOF] from
// resp.Body), the promotion error, not [io.EOF], is returned by Read. Thus, a
// consumer of responseCacher will not receive [io.EOF] unless the cache is
// successfully promoted.
type responseCacher struct {
	log        *slog.Logger
	body       io.ReadCloser
	closeErr   *error
	f          *os.File
	mainDir    string
	stagingDir string
	mu         sync.Mutex
}

// Read implements [io.Reader]. It reads into p from the wrapped response body,
// appends the received bytes to the staging cache, and returns the number of
// bytes read, and any error. When Read encounters [io.EOF] from the response
// body, it promotes the staging cache to main, and on success returns [io.EOF].
// If an error occurs during cache promotion, Read returns that promotion error
// instead of [io.EOF].
func (r *responseCacher) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Use r.body as a sentinel to indicate that the cache
	// has been closed.
	if r.body == nil {
		return 0, errz.New("response cache already closed")
	}

	n, err = r.body.Read(p)

	switch {
	case err == nil:
		err = r.cacheAppend(p, n)
		return n, err
	case !errors.Is(err, io.EOF):
		// It's some other kind of error.
		// Clean up and return.
		_ = r.body.Close()
		r.body = nil
		_ = r.f.Close()
		_ = os.Remove(r.f.Name())
		r.f = nil
		_ = os.RemoveAll(r.stagingDir)
		r.stagingDir = ""
		return n, err
	default:
		// It's EOF time! Let's promote the cache.
	}

	var err2 error
	if err2 = r.cacheAppend(p, n); err2 != nil {
		return n, err2
	}

	if err2 = r.cachePromote(); err2 != nil {
		return n, err2
	}

	return n, err
}

// cacheAppend appends n bytes from p to the staging cache. If an error occurs,
// the staging cache is discarded, and the error is returned.
func (r *responseCacher) cacheAppend(p []byte, n int) error {
	_, err := r.f.Write(p[:n])
	if err == nil {
		return nil
	}
	_ = r.body.Close()
	r.body = nil
	_ = r.f.Close()
	_ = os.Remove(r.f.Name())
	r.f = nil
	_ = os.RemoveAll(r.stagingDir)
	r.stagingDir = ""
	return errz.Wrap(err, "failed to append http response body bytes to staging cache")
}

// cachePromote is invoked by [Read] when it receives [io.EOF] from the wrapped
// response body. It promotes the staging cache to main, and on success returns
// nil. If an error occurs during promotion, the staging cache is discarded, and
// the promotion error is returned.
func (r *responseCacher) cachePromote() error {
	defer func() {
		if r.f != nil {
			_ = r.f.Close()
			r.f = nil
		}
		if r.body != nil {
			_ = r.body.Close()
			r.body = nil
		}
		if r.stagingDir != "" {
			_ = os.RemoveAll(r.stagingDir)
			r.stagingDir = ""
		}
	}()

	fi, err := r.f.Stat()
	if err != nil {
		return errz.Wrap(err, "failed to stat staging cache body file")
	}

	fpBody := r.f.Name()
	err = r.f.Close()
	r.f = nil
	if err != nil {
		return errz.Wrap(err, "failed to close cache body file")
	}

	err = r.body.Close()
	r.body = nil
	if err != nil {
		return errz.Wrap(err, "failed to close http response body")
	}

	var sum checksum.Checksum
	if sum, err = checksum.ForFile(fpBody); err != nil {
		return errz.Wrap(err, "failed to compute checksum for cache body file")
	}

	if err = checksum.WriteFile(filepath.Join(r.stagingDir, "checksums.txt"), sum, "body"); err != nil {
		return errz.Wrap(err, "failed to write checksum file for cache body")
	}

	// We've got good data in the staging dir. Now we do the switcheroo.
	if err = ioz.RenameDir(r.stagingDir, r.mainDir); err != nil {
		return errz.Wrap(err, "failed to write download cache")
	}
	r.stagingDir = ""

	r.log.Info("Promoted download staging cache to main", lga.Size, fi.Size(), lga.Dir, r.mainDir)
	return nil
}

// Close implements [io.Closer]. Note that cache promotion happens when Read
// receives [io.EOF] from the wrapped response body, so the main action should
// be over by the time Close is invoked. Note that Close is idempotent, and
// returns the same error on subsequent invocations.
func (r *responseCacher) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closeErr != nil {
		// Already closed
		return *r.closeErr
	}

	// There's some duplication of logic with using both r.closeErr
	// and r.body == nil as sentinels. This could be cleaned up.

	var err error
	if r.f != nil {
		err = errz.Append(err, r.f.Close())
		r.f = nil
	}

	if r.body != nil {
		err = errz.Append(err, r.body.Close())
		r.body = nil
	}

	if r.stagingDir != "" {
		err = errz.Append(err, os.RemoveAll(r.stagingDir))
		r.stagingDir = ""
	}

	r.closeErr = &err
	return *r.closeErr
}
