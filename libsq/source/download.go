package source

import (
	"context"
	"github.com/neilotoole/streamcache"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/ioz/download"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/options"
)

var OptHTTPRequestTimeout = options.NewDuration(
	"http.request.timeout",
	"",
	0,
	time.Second*10,
	"HTTP/S request initial response timeout duration",
	`How long to wait for initial response from a HTTP/S endpoint before
timeout occurs. Reading the body of the response, such as a large HTTP file
download, is not affected by this option. Example: 500ms or 3s.
Contrast with http.response.timeout.`,
	options.TagSource,
)

var OptHTTPResponseTimeout = options.NewDuration(
	"http.response.timeout",
	"",
	0,
	0,
	"HTTP/S request completion timeout duration",
	`How long to wait for the entire HTTP transaction to complete. This includes
reading the body of the response, such as a large HTTP file download. Typically
this is set to 0, indicating no timeout. Contrast with http.request.timeout.`,
	options.TagSource,
)

var OptHTTPSInsecureSkipVerify = options.NewBool(
	"https.insecure-skip-verify",
	"",
	false,
	0,
	false,
	"Skip HTTPS TLS verification",
	"Skip HTTPS TLS verification. Useful when downloading against self-signed certs.",
	options.TagSource,
)

// downloadFor returns the download.Download for src, creating
// and caching it if necessary.
func (fs *Files) downloadFor(ctx context.Context, src *Source) (*download.Download, error) {
	dl, ok := fs.downloads[src.Handle]
	if ok {
		return dl, nil
	}

	dlDir, err := fs.downloadDirFor(src)
	if err != nil {
		return nil, err
	}
	if err = ioz.RequireDir(dlDir); err != nil {
		return nil, err
	}

	c := fs.httpClientFor(ctx, src)
	if dl, err = download.New(src.Handle, c, src.Location, dlDir); err != nil {
		return nil, err
	}
	fs.downloads[src.Handle] = dl
	return dl, nil
}

func (fs *Files) httpClientFor(ctx context.Context, src *Source) *http.Client {
	o := options.Merge(options.FromContext(ctx), src.Options)
	return httpz.NewClient(httpz.DefaultUserAgent,
		httpz.OptRequestTimeout(OptHTTPRequestTimeout.Get(o)),
		httpz.OptResponseTimeout(OptHTTPResponseTimeout.Get(o)),
		httpz.OptInsecureSkipVerify(OptHTTPSInsecureSkipVerify.Get(o)),
	)
}

// downloadDirFor gets the download cache dir for src. It is not
// guaranteed that the returned dir exists or is accessible.
func (fs *Files) downloadDirFor(src *Source) (string, error) {
	cacheDir, err := fs.CacheDirFor(src)
	if err != nil {
		return "", err
	}

	fp := filepath.Join(cacheDir, "download", checksum.Sum([]byte(src.Location)))
	return fp, nil
}

// openRemoteFile adds a remote file to fs's cache, returning a reader.
// If the remote file is already cached, the path to that cached download
// file is returned in cachedDownload; otherwise cachedDownload is empty.
// If checkFresh is false and the file is already fully downloaded, its
// freshness is not checked against the remote server.
func (fs *Files) openRemoteFile(ctx context.Context, src *Source, checkFresh bool) (cachedDownload string,
	rdr io.ReadCloser, err error,
) {
	loc := src.Location
	if getLocType(loc) != locTypeRemoteFile {
		return "", nil, errz.Errorf("not a remote file: %s", loc)
	}

	dl, err := fs.downloadFor(ctx, src)
	if err != nil {
		return "", nil, err
	}

	// checkFresh should be false on subsequent calls.

	if !checkFresh && fs.hasRdrCache(loc) {
		// If the download has completed, dl.CacheFile will return the
		// path to the cached file.
		cachedDownload, err = dl.CacheFile(ctx)
		if err != nil {
			return "", nil, err
		}

		cache, _ := fs.rdrCaches[loc]

		// The file is already cached, and we're not checking freshness.
		// So, we can just return the cached reader.
		rdr := cache.NewReader(ctx)
		//rdr, _, err = fs.fscache.Get(loc)
		//if err != nil {
		//	return "", nil, errz.Err(err)
		//}
		return cachedDownload, rdr, nil
	}

	errCh := make(chan error, 1)
	rdrCh := make(chan io.ReadCloser, 1)

	h := download.Handler{
		Cached: func(fp string) {
			//if !fs.fscache.Exists(fp) {
			//	if hErr := fs.fscache.MapFile(fp); hErr != nil {
			//		errCh <- errz.Wrapf(hErr, "failed to map file into fscache: %s", fp)
			//		return
			//	}
			//}

			cache, ok := fs.rdrCaches[loc]
			if !ok {
				f, err := os.Open(fp)
				if err != nil {
					errCh <- errz.Wrapf(err, "failed to open cached file: %s", fp)
					return
				}
				cache = streamcache.New(f)
				fs.rdrCaches[loc] = cache
			}

			r := cache.NewReader(ctx)

			//r, _, hErr := fs.fscache.Get(fp)
			//if hErr != nil {
			//	errCh <- errz.Err(hErr)
			//	return
			//}
			cachedDownload = fp
			rdrCh <- r
		},
		Uncached: func(cache *streamcache.Cache) {
			//r, w, wErrFn, hErr := fs.fscache.GetWithErr(loc)
			//if hErr != nil {
			//	errCh <- errz.Err(hErr)
			//	return nil
			//}
			//
			//wec := ioz.NewFuncWriteErrorCloser(w, func(err error) {
			//	lg.FromContext(ctx).Error("Error writing to fscache", lga.Src, src, lga.Err, err)
			//	wErrFn(err)
			//})
			fs.rdrCaches[loc] = cache
			r := cache.NewReader(ctx)
			rdrCh <- r
		},
		Error: func(hErr error) {
			errCh <- hErr
		},
	}

	fs.fillerWgs.Add(1)
	go func() {
		defer fs.fillerWgs.Done()
		dl.Get(ctx, h)
	}()

	select {
	case <-ctx.Done():
		return "", nil, errz.Err(ctx.Err())
	case err = <-errCh:
		return "", nil, err
	case rdr = <-rdrCh:
		return cachedDownload, rdr, nil
	}
}
