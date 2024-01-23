package source

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/neilotoole/sq/libsq/core/ioz/downloader"

	"github.com/neilotoole/streamcache"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
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

// maybeStartDownload starts a download for src if one is not already in progress
// or completed. If there's a download in progress, dlStream returns non-nil.
// If the file is already downloaded to disk (and is valid/fresh), dlFile
// returns non-empty and contains the absolute path to the downloaded file.
// Otherwise, a new download is started, and dlStream returns non-nil. The
// download happens on a freshly-spawned goroutine, and Files.downloadersWg
// is incremented; wait on that WaitGroup to ensure that all downloads have
// completed.
//
// It is guaranteed that one (and only one) of the returned values will be non-nil.
// REVISIT: look into use of checkFresh?
func (fs *Files) maybeStartDownload(ctx context.Context, src *Source, checkFresh bool) (dlFile string,
	dlStream *streamcache.Stream, err error,
) {
	var ok bool
	if dlStream, ok = fs.mStreams[src.Handle]; ok {
		// A download stream is always fresh, so we
		// can ignore checkFresh here.
		return "", dlStream, nil
	}

	dldr, err := fs.downloaderFor(ctx, src)
	if err != nil {
		return "", nil, err
	}

	if !checkFresh {
		// If we don't care about freshness, check if the download is
		// already on disk. If so, Downloader.CacheFile will return the
		// path to the cached file.
		dlFile, err = dldr.CacheFile(ctx)
		if err == nil && dlFile != "" {
			// The file is already on disk, so we can just return it.
			return dlFile, nil, err
		}
	}

	// Having got this far, we need to talk to the downloader.
	var (
		dlErrCh    = make(chan error, 1)
		dlStreamCh = make(chan *streamcache.Stream, 1)
		dlFileCh   = make(chan string, 1)
	)

	h := downloader.Handler{
		Cached:   func(dlFile string) { dlFileCh <- dlFile },
		Uncached: func(dlStream *streamcache.Stream) { dlStreamCh <- dlStream },
		Error:    func(dlErr error) { dlErrCh <- dlErr },
	}

	fs.downloadersWg.Add(1)
	go func() {
		defer fs.downloadersWg.Done()
		dldr.Get(ctx, h)
	}()

	select {
	case <-ctx.Done():
		return "", nil, errz.Err(ctx.Err())
	case err = <-dlErrCh:
		return "", nil, err
	case dlStream = <-dlStreamCh:
		fs.mStreams[src.Handle] = dlStream
		return "", dlStream, nil
	case dlFile = <-dlFileCh:
		fs.mDownloadedFiles[src.Handle] = dlFile
		return dlFile, nil, nil
	}
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

// downloaderFor returns the downloader.Downloader for src, creating
// and caching it if necessary.
func (fs *Files) downloaderFor(ctx context.Context, src *Source) (*downloader.Downloader, error) {
	dl, ok := fs.mDownloaders[src.Handle]
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
	if dl, err = downloader.New(src.Handle, c, src.Location, dlDir); err != nil {
		return nil, err
	}
	fs.mDownloaders[src.Handle] = dl
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
