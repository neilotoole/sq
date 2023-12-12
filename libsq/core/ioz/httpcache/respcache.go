package httpcache

import (
	"bufio"
	"bytes"
	"context"
	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"sync"
)

// NewRespCache returns a new instance that stores responses in cacheDir.
// The caller should call RespCache.Close when finished with the cache.
func NewRespCache(cacheDir string) *RespCache {
	c := &RespCache{
		Dir:    cacheDir,
		Header: filepath.Join(cacheDir, "header"),
		Body:   filepath.Join(cacheDir, "body"),
		clnup:  cleanup.New(),
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

	// Header is the path to the file containing the http.Response header.
	Header string

	// Body is the path to the file containing the http.Response body.
	Body string
}

func (rc *RespCache) getPaths(req *http.Request) (header, body string) {
	if req.Method == http.MethodGet {
		return filepath.Join(rc.Dir, "header"), filepath.Join(rc.Dir, "body")
	}

	return filepath.Join(rc.Dir, req.Method+"_header"),
		filepath.Join(rc.Dir, req.Method+"_body")
}

// Get returns the cached http.Response for req if present, and nil
// otherwise.
func (rc *RespCache) Get(ctx context.Context, req *http.Request) (*http.Response, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if !ioz.FileAccessible(rc.Header) {
		return nil, nil
	}

	headerBytes, err := os.ReadFile(rc.Header)
	if err != nil {
		return nil, err
	}

	bodyFile, err := os.Open(rc.Body)
	if err != nil {
		lg.FromContext(ctx).Error("failed to open cached response body",
			lga.File, rc.Body, lga.Err, err)
		return nil, err
	}

	// We need to explicitly close bodyFile at some later point. It won't be
	// closed via a call to http.Response.Body.Close().
	rc.clnup.AddC(bodyFile)

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

// Delete deletes the cache entries from disk.
func (rc *RespCache) Delete() error {
	if rc == nil {
		return nil
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()

	return rc.doDelete()
}

func (rc *RespCache) doDelete() error {
	err1 := rc.clnup.Run()
	rc.clnup = cleanup.New()
	err2 := os.RemoveAll(rc.Header)
	err3 := os.RemoveAll(rc.Body)
	return errz.Combine(err1, err2, err3)
}

const msgDeleteCache = "Delete HTTP response cache"

// Write writes resp to the cache.
func (rc *RespCache) Write(ctx context.Context, resp *http.Response) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	err := rc.doWrite(ctx, resp)
	if err != nil {
		lg.WarnIfFuncError(lg.FromContext(ctx), msgDeleteCache, rc.doDelete)
	}
	return err
}

func (rc *RespCache) doWrite(ctx context.Context, resp *http.Response) error {
	log := lg.FromContext(ctx)

	if err := ioz.RequireDir(filepath.Dir(rc.Header)); err != nil {
		return err
	}

	if err := ioz.RequireDir(filepath.Dir(rc.Body)); err != nil {
		return err
	}

	respBytes, err := httputil.DumpResponse(resp, false)
	if err != nil {
		return err
	}

	if _, err = ioz.WriteToFile(ctx, rc.Header, bytes.NewReader(respBytes)); err != nil {
		return err
	}

	f, err := os.OpenFile(rc.Body, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}

	cr := contextio.NewReader(ctx, resp.Body)
	_, err = io.Copy(f, cr)
	if err != nil {
		lg.WarnIfCloseError(log, "Close cache body file", f)
		return err
	}

	if err = f.Close(); err != nil {
		return err
	}

	f, err = os.Open(rc.Body)
	if err != nil {
		return err
	}

	resp.Body = f
	return nil
}
