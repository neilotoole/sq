// This file implements the disk-based cache for HTTP downloads.
// See the package documentation for an overview of the cache architecture.

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

// Log message constants for cache operations. These are used consistently
// throughout the cache implementation for logging and error reporting.
const (
	// msgCloseCacheHeaderFile is logged when closing the cached response header file.
	msgCloseCacheHeaderFile = "Close cached response header file"
	// msgCloseCacheBodyFile is logged when closing the cached response body file.
	msgCloseCacheBodyFile = "Close cached response body file"
	// msgDeleteCache is logged when deleting the HTTP response cache directory.
	msgDeleteCache = "Delete HTTP response cache"
)

// cache manages the on-disk storage for a single download. The cached HTTP
// response is stored as three files within a "main" subdirectory:
//
//   - header: The serialized HTTP response headers (via [httputil.DumpResponse])
//   - body: The raw response body bytes
//   - checksums.txt: SHA-256 checksum of the body file for integrity verification
//
// The cache uses a two-phase commit strategy to ensure atomicity:
//
//  1. New downloads are first written to a "staging" subdirectory
//  2. Upon successful completion (all bytes received, checksum computed),
//     the staging directory atomically replaces the main directory
//
// This prevents partial or corrupt downloads from replacing valid cached data.
// If a download fails mid-stream, the staging directory is discarded and
// the main cache (if any) remains intact.
//
// Use [cache.paths] to get the file paths, [cache.exists] to check validity,
// and [cache.get] to retrieve a cached response.
type cache struct {
	// dir is the root directory for this cache instance. It contains two
	// subdirectories: "main" (the active cache) and "staging" (for writes
	// in progress). The dir is specific to a particular download URL.
	dir string
}

// paths returns the file paths to the header, body, and checksum files for
// the given request. The paths are within the "main" subdirectory of [cache.dir].
// Note that the files may not exist; use [cache.exists] to verify.
//
// For GET requests (or nil request), the files are named simply "header",
// "body", and "checksums.txt". For other HTTP methods, the method name is
// prefixed (e.g., "HEAD_header"), though in practice only GET is used.
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

// exists reports whether a valid cache exists for the given request.
//
// A cache is considered valid if:
//   - The header file exists and is non-empty
//   - The body file exists
//   - The checksums.txt file exists
//   - The checksum in checksums.txt matches a freshly computed checksum of the body
//
// If the cache is inconsistent (some files exist but not others, or checksums
// don't match), exists calls [cache.clearIfInconsistent] to clean up the
// invalid state before returning false.
//
// The request is used to determine the cache file paths and to provide
// context for logging.
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

// clearIfInconsistent deletes the cache if it is in an inconsistent state.
//
// A cache is inconsistent if some but not all of the three required files
// (header, body, checksums.txt) exist, or if the stored checksum doesn't match
// a freshly computed checksum of the body file.
//
// An inconsistent cache can occur if:
//   - A previous download was interrupted mid-write
//   - Disk corruption affected some files
//   - Manual tampering with cache files
//
// This method is called by [cache.exists] before checking validity. If the
// cache is empty or fully consistent, this method is a no-op and returns nil.
// If an inconsistency is detected, [cache.clear] is called to remove all
// cache files.
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

// get retrieves a cached HTTP response for the given request, if available.
//
// If no valid cache exists, get returns (nil, nil). A valid cache requires
// the header file to exist and be accessible, and for the cached checksums
// to match (see [cache.checksumsMatch]).
//
// On success, get returns the reconstructed [http.Response] with a readable
// Body. The response is reconstructed by:
//  1. Reading the serialized headers from the "header" file
//  2. Opening the "body" file
//  3. Concatenating them via [io.MultiReader]
//  4. Parsing via [http.ReadResponse]
//
// The returned response's Body is wrapped with a notifier that closes the
// underlying body file when Body.Close() is called. Therefore, it is CRITICAL
// that the caller close the returned response body to avoid file descriptor
// leaks.
//
// The context is used for:
//   - Cancellation support via [contextio.NewReader]
//   - Logging on errors
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

// cachedChecksum reads and returns the checksum stored in the cache's
// checksums.txt file for the given request, if available.
//
// The checksums.txt file is written during cache promotion (see
// [responseCacher.cachePromote]) and contains a single SHA-256 checksum
// for the "body" file entry.
//
// Returns ("", false) if:
//   - The checksums.txt file doesn't exist or isn't accessible
//   - The file cannot be parsed
//   - The file doesn't contain exactly one entry for "body"
//
// This method only reads the stored checksum; it does not verify that
// the checksum matches the actual body file. Use [cache.checksumsMatch]
// for validation.
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

// checksumsMatch validates the integrity of the cached body file by comparing
// the stored checksum against a freshly computed checksum.
//
// This method performs the following steps:
//  1. Reads the stored checksum from checksums.txt via [cache.cachedChecksum]
//  2. Computes a fresh SHA-256 checksum of the body file
//  3. Compares the two checksums
//
// Returns (checksum, true) if the checksums match, indicating the cache
// is valid and uncorrupted. Returns ("", false) if:
//   - No cached checksum exists
//   - The body file cannot be read or checksummed
//   - The checksums don't match (logs a warning about inconsistent cache)
//
// This is the authoritative method for validating cache integrity and is
// called by [cache.exists] and [cache.get].
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

// clear removes all cache data from disk, including both the main cache
// and any staging data.
//
// The method performs two operations:
//  1. Recursively deletes the entire cache directory (including "main" and
//     "staging" subdirectories)
//  2. Recreates the empty cache directory
//
// This ensures the cache is in a clean, empty state. If either operation
// fails, the combined error is logged and returned. On success, an info
// message is logged.
//
// This method is called by [cache.clearIfInconsistent] when corruption is
// detected, and can be called directly via [Downloader.Clear] to manually
// invalidate the cache.
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

// writeHeader updates only the header file in the main cache from the
// provided response, leaving the body file unchanged.
//
// This method is used when the server returns HTTP 304 Not Modified, indicating
// that the cached body is still valid but the headers (such as cache-control
// directives or dates) have been updated. In this case, only the header file
// needs to be refreshed.
//
// The method:
//  1. Serializes the response headers via [httputil.DumpResponse] (body=false)
//  2. Ensures the main cache directory exists
//  3. Writes the header bytes to the "header" file
//
// Note that resp.Body is NOT read or closed by this method; the caller
// retains responsibility for the response body.
//
// This is distinct from [cache.newResponseCacher], which handles full
// response caching including the body via the two-phase staging approach.
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

// newResponseCacher creates a new [responseCacher] to stream and cache the
// response body.
//
// This method initiates the two-phase cache commit process:
//  1. Creates the "staging" subdirectory within the cache directory
//  2. Serializes and writes the response headers to staging/header
//  3. Creates an empty staging/body file for streaming writes
//  4. Returns a [responseCacher] that wraps resp.Body
//
// The returned [responseCacher] implements [io.ReadCloser]. As data is read
// from it, the bytes are simultaneously written to the staging body file.
// When [io.EOF] is received, [responseCacher] computes a checksum and atomically
// promotes the staging directory to replace the main cache.
//
// IMPORTANT: On return, resp.Body is set to nil. The [responseCacher] now owns
// the original response body and will close it appropriately. The caller should
// read from the returned [responseCacher] instead of the original resp.Body.
//
// If any error occurs during setup (creating directories, writing header,
// creating body file), the response body is closed and the error is returned.
// The caller should not attempt to read from resp.Body in this case.
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

// responseCacher is an [io.ReadCloser] that wraps an [http.Response.Body],
// simultaneously streaming bytes to the caller and caching them to disk.
//
// It functions similarly to [io.TeeReader]: bytes read from the wrapped
// response body are written to a staging cache file before being returned
// to the caller. This allows the download to be used immediately while also
// being persisted for future use.
//
// # Cache Promotion
//
// When Read receives [io.EOF] from the wrapped response body, indicating the
// download is complete, responseCacher performs cache promotion:
//  1. Closes the staging body file
//  2. Computes a SHA-256 checksum of the body
//  3. Writes the checksum to staging/checksums.txt
//  4. Atomically renames the staging directory to replace the main cache
//
// If promotion succeeds, [io.EOF] is returned to the caller. If promotion
// fails, the promotion error is returned instead of [io.EOF], signaling that
// the download completed but caching failed.
//
// # Error Handling
//
// If an error (other than [io.EOF]) occurs during Read, the staging cache is
// immediately discarded and the main cache is left untouched. This ensures
// that partial or corrupt downloads never replace valid cached data.
//
// # Thread Safety
//
// All methods are protected by a mutex for safe concurrent access, though
// typical usage involves a single goroutine reading to completion.
type responseCacher struct {
	// log is the logger for cache operations.
	log *slog.Logger

	// body is the wrapped HTTP response body being read and cached.
	// Set to nil after the response is fully read or on error.
	body io.ReadCloser

	// closeErr stores the error from Close(), enabling idempotent Close calls.
	// Once set, subsequent Close calls return this same error.
	closeErr *error

	// f is the open file handle for the staging body file being written to.
	// Set to nil after the file is closed (on EOF or error).
	f *os.File

	// mainDir is the path to the main cache directory that will receive
	// the promoted staging data on successful completion.
	mainDir string

	// stagingDir is the path to the staging directory where data is written
	// during the download. Atomically renamed to mainDir on success.
	stagingDir string

	// mu protects all fields for concurrent access.
	mu sync.Mutex
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

// cacheAppend writes the first n bytes of p to the staging body file.
//
// This is an internal helper called by [responseCacher.Read] after successfully
// reading bytes from the wrapped response body. The bytes are written to the
// staging file so they can be persisted for future cache hits.
//
// If the write fails (e.g., disk full, I/O error), cacheAppend performs
// cleanup: it closes the response body, closes and removes the staging body
// file, and removes the staging directory. This ensures no partial data
// remains. The wrapped error is returned to the caller.
//
// On success, returns nil.
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

// cachePromote finalizes the cache by promoting the staging directory to main.
//
// This method is invoked by [responseCacher.Read] when it receives [io.EOF]
// from the wrapped response body, indicating the download completed successfully.
//
// The promotion process:
//  1. Stats the staging body file to get its size for logging
//  2. Closes the staging body file
//  3. Closes the wrapped response body
//  4. Computes a SHA-256 checksum of the staging body file
//  5. Writes the checksum to staging/checksums.txt
//  6. Atomically renames the staging directory to replace the main directory
//     (via [ioz.RenameDir], which handles cross-device moves if needed)
//
// If any step fails, the deferred cleanup removes the staging directory and
// the error is returned. This ensures the main cache remains untouched on
// failure.
//
// On success, logs an info message with the file size and returns nil. The
// caller ([responseCacher.Read]) then returns [io.EOF] to its caller.
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
