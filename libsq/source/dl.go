package source

import (
	"context"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/ioz/httpcache"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"io"
	"log/slog"
	"net/http"
	"sync"
)

// newDownloader creates a new downloader using cacheDir for the given url.
func newDownloader2(cacheDir, userAgent, dlURL string) (*downloader2, error) {
	//dv := diskv.New(diskv.Options{
	//	BasePath:     filepath.Join(cacheDir, "cache"),
	//	TempDir:      filepath.Join(cacheDir, "working"),
	//	CacheSizeMax: 10000 * 1024 * 1024, // 10000MB
	//})
	if err := ioz.RequireDir(cacheDir); err != nil {
		return nil, err
	}

	//dc := diskcache.NewWithDiskv(dv)
	rc := httpcache.NewRespCache(cacheDir)
	tp := httpcache.NewTransport(rc)

	//respCache := httpcache.NewRespCache(cacheDir)
	//tp.RespCache = respCache
	//tp.BodyFilepath = filepath.Join(cacheDir, "body.data")

	c := &http.Client{Transport: tp}

	return &downloader2{
		c: c,
		//dc: dc,
		//dv:        dv,
		cacheDir:  cacheDir,
		url:       dlURL,
		userAgent: userAgent,
		tp:        tp,
	}, nil
}

type downloader2 struct {
	c         *http.Client
	mu        sync.Mutex
	userAgent string
	cacheDir  string
	url       string
	tp        *httpcache.Transport
}

func (d *downloader2) log(log *slog.Logger) *slog.Logger {
	return log.With(lga.URL, d.url, lga.Dir, d.cacheDir)
}

// ClearCache clears the cache dir.
func (d *downloader2) ClearCache(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.tp.RespCache.Delete(ctx); err != nil {
		//log.Error("Failed to delete cache dir", lga.Dir, d.cacheDir, lga.Err, err)
		return errz.Wrapf(err, "failed to clear cache dir: %s", d.cacheDir)
	}

	return ioz.RequireDir(d.cacheDir)
}

func (d *downloader2) Download(ctx context.Context, dest io.Writer) (written int64, fp string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	log := d.log(lg.FromContext(ctx))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url, nil)
	if d.userAgent != "" {
		req.Header.Set("User-Agent", d.userAgent)
	}

	isCached := d.tp.IsCached(req)
	_ = isCached

	isFresh := d.tp.IsFresh(req)
	_ = isFresh

	resp, err := d.c.Do(req)
	if err != nil {
		return written, "", errz.Wrapf(err, "download failed for: %s", d.url)
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			lg.WarnIfCloseError(log, lgm.CloseHTTPResponseBody, resp.Body)
		}
	}()

	written, err = io.Copy(
		contextio.NewWriter(ctx, dest),
		contextio.NewReader(ctx, resp.Body),
	)

	return written, "", err
}

func (d *downloader2) Download2(ctx context.Context, dest io.Writer) (written int64, fp string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	log := d.log(lg.FromContext(ctx))
	_ = log

	var destWrtr io.WriteCloser
	var ok bool
	if destWrtr, ok = dest.(io.WriteCloser); !ok {
		destWrtr = ioz.WriteCloser(dest)
	}

	log.Debug("huzzah Download2")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url, nil)
	if d.userAgent != "" {
		req.Header.Set("User-Agent", d.userAgent)
	}

	isCached := d.tp.IsCached(req)
	_ = isCached

	isFresh := d.tp.IsFresh(req)
	_ = isFresh

	var gotFp string
	var gotErr error
	//buf := &bytes.Buffer{}
	cb := httpcache.CallbackHandler{
		HandleCached: func(cachedFilepath string) error {
			gotFp = cachedFilepath
			return nil
		},
		HandleUncached: func() (wc io.WriteCloser, errFn func(error), err error) {
			return destWrtr, func(err error) {
				gotErr = err
			}, nil
		},
		HandleError: func(err error) {
			gotErr = err
		},
	}

	d.tp.Fetch(req, cb)
	_ = gotFp
	_ = gotErr

	return written, "", err
}
