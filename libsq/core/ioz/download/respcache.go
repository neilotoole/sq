package download

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"sync"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
)

// NewRespCache returns a new instance that stores responses in cacheDir.
// The caller should call RespCache.Close when finished with the cache.
func NewRespCache(cacheDir string) *RespCache {
	c := &RespCache{
		Dir: cacheDir,
		// Header: filepath.Join(cacheDir, "header"),
		// Body:   filepath.Join(cacheDir, "body"),
		clnup: cleanup.New(),
	}
	return c
}

// RespCache is a cache for a single http.Response. The response is
// stored in two files, one for the header and one for the body.
// The caller should call RespCache.Close when finished with the cache.
type RespCache struct {
	mu    sync.Mutex
	clnup *cleanup.Cleanup

	Dir string
}

// Paths returns the paths to the header and body files for req.
// It is not guaranteed that they exist.
func (rc *RespCache) Paths(req *http.Request) (header, body string) {
	if req == nil || req.Method == http.MethodGet {
		return filepath.Join(rc.Dir, "header"), filepath.Join(rc.Dir, "body")
	}

	return filepath.Join(rc.Dir, req.Method+"_header"),
		filepath.Join(rc.Dir, req.Method+"_body")
}

// Exists returns true if the cache contains a response for req.
func (rc *RespCache) Exists(req *http.Request) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	fpHeader, _ := rc.Paths(req)
	fi, err := os.Stat(fpHeader)
	if err != nil {
		return false
	}
	return fi.Size() > 0
}

// Get returns the cached http.Response for req if present, and nil
// otherwise.
func (rc *RespCache) Get(ctx context.Context, req *http.Request) (*http.Response, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	fpHeader, fpBody := rc.Paths(req)

	if !ioz.FileAccessible(fpHeader) {
		return nil, nil
	}

	headerBytes, err := os.ReadFile(fpHeader)
	if err != nil {
		return nil, err
	}

	bodyFile, err := os.Open(fpBody)
	if err != nil {
		lg.FromContext(ctx).Error("failed to open cached response body",
			lga.File, fpBody, lga.Err, err)
		return nil, err
	}

	// We need to explicitly close bodyFile at some later point. It won't be
	// closed via a call to http.Response.Body.Close().
	rc.clnup.AddC(bodyFile)
	// TODO: consider adding contextio.NewReader?
	concatRdr := io.MultiReader(bytes.NewReader(headerBytes), bodyFile)
	return http.ReadResponse(bufio.NewReader(concatRdr), req)
}

// Close closes the cache, freeing any resources it holds. Note that
// it does not delete the cache: for that, see RespCache.Delete.
func (rc *RespCache) Close() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	err := rc.clnup.Run()
	rc.clnup = cleanup.New()
	return err
}

// Clear deletes the cache entries from disk.
func (rc *RespCache) Clear(ctx context.Context) error {
	if rc == nil {
		return nil
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()

	return rc.doClear(ctx)
}

func (rc *RespCache) doClear(ctx context.Context) error {
	cleanErr := rc.clnup.Run()
	rc.clnup = cleanup.New()
	deleteErr := errz.Wrap(os.RemoveAll(rc.Dir), "delete cache dir")
	err := errz.Combine(cleanErr, deleteErr)
	if err != nil {
		lg.FromContext(ctx).Error(msgDeleteCache,
			lga.Dir, rc.Dir, lga.Err, err)
		return err
	}

	lg.FromContext(ctx).Info("Deleted cache dir", lga.Dir, rc.Dir)
	return nil
}

const msgDeleteCache = "Delete HTTP response cache"

// Write writes resp to the cache. If copyWrtr is non-nil, the response
// bytes are copied to that destination also.
func (rc *RespCache) Write(ctx context.Context, resp *http.Response, copyWrtr io.WriteCloser) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	err := rc.doWrite(ctx, resp, copyWrtr)
	if err != nil {
		lg.WarnIfError(lg.FromContext(ctx), msgDeleteCache, rc.doClear(ctx))
	}
	return err
}

func (rc *RespCache) doWrite(ctx context.Context, resp *http.Response, copyWrtr io.WriteCloser) error {
	log := lg.FromContext(ctx)

	if err := ioz.RequireDir(rc.Dir); err != nil {
		return err
	}

	fpHeader, fpBody := rc.Paths(resp.Request)

	headerBytes, err := httputil.DumpResponse(resp, false)
	if err != nil {
		return err
	}

	if _, err = ioz.WriteToFile(ctx, fpHeader, bytes.NewReader(headerBytes)); err != nil {
		return err
	}

	cacheFile, err := os.OpenFile(fpBody, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}

	var cr io.Reader
	if copyWrtr == nil {
		cr = contextio.NewReader(ctx, resp.Body)
	} else {
		tr := io.TeeReader(resp.Body, copyWrtr)
		cr = contextio.NewReader(ctx, tr)
	}

	//if copyWrtr != nil {
	//	cr = io.TeeReader(cr, copyWrtr)
	//}
	var written int64
	written, err = io.Copy(cacheFile, cr)
	if err != nil {
		lg.WarnIfCloseError(log, "Close cache body file", cacheFile)
		return err
	}
	if copyWrtr != nil {
		lg.WarnIfCloseError(log, "Close copy writer", copyWrtr)
	}

	if err = cacheFile.Close(); err != nil {
		return err
	}

	log.Info("Wrote HTTP response to cache", lga.File, fpBody, lga.Size, written)
	cacheFile, err = os.Open(fpBody)
	if err != nil {
		return err
	}

	resp.Body = cacheFile
	return nil
}
