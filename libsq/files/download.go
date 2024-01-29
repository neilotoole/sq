package files

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/neilotoole/streamcache"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/files/internal/downloader"
	"github.com/neilotoole/sq/libsq/source"
)

var (
	OptHTTPRequestTimeout = options.NewDuration(
		"http.request.timeout",
		"",
		0,
		time.Second*10,
		"HTTP/S request initial response timeout duration",
		`How long to wait for initial response from a HTTP/S endpoint before timeout
occurs. Reading the body of the response, such as a large HTTP file download,
is not affected by this option. Example: 500ms or 3s.

Contrast with http.response.timeout.`,
		options.TagSource,
	)
	OptHTTPResponseTimeout = options.NewDuration(
		"http.response.timeout",
		"",
		0,
		0,
		"HTTP/S request completion timeout duration",
		`How long to wait for the entire HTTP transaction to complete. This includes
reading the body of the response, such as a large HTTP file download. Typically
this is set to 0, indicating no timeout.

Contrast with http.request.timeout.`,
		options.TagSource,
	)
	OptHTTPSInsecureSkipVerify = options.NewBool(
		"https.insecure-skip-verify",
		"",
		false,
		0,
		false,
		"Skip HTTPS TLS verification",
		"Skip HTTPS TLS verification. Useful when downloading against self-signed certs.",
		options.TagSource,
	)
	OptDownloadContinueOnError = downloader.OptContinueOnError
	OptDownloadCache           = downloader.OptCache
)

// maybeStartDownload starts a download for src if one is not already in progress
// or completed. If there's a download in progress, dlStream returns non-nil.
// If the file is already downloaded to disk (and is valid/fresh), dlFile
// returns non-empty and contains the absolute path to the downloaded file.
// Otherwise, a new download is started (on a spawned goroutine), and the
// stream returned from the downloader is added to Files.streams. On successful
// download and cache update completion, the stream is removed from Files.streams
// and the path to the cached file is added to Files.downloadedFiles.
//
// If arg checkFresh is false, and there's already a cached download on disk,
// then the cached file is returned immediately, and no download is started.
//
// It is guaranteed that one (and only one) of the returned values will be non-nil.
// REVISIT: look into use of checkFresh?
func (fs *Files) maybeStartDownload(ctx context.Context, src *source.Source, checkFresh bool) (dlFile string,
	dlStream *streamcache.Stream, err error,
) {
	var ok bool

	// If the file has just been downloaded, just return it. It doesn't
	// matter about checkFresh.
	if dlFile, ok = fs.downloadedFiles[src.Handle]; ok {
		return dlFile, nil, nil
	}

	// If there's already a download in progress, then we can just return
	// that stream.
	if dlStream, ok = fs.streams[src.Handle]; ok {
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
			// REVISIT: Should we add dlFile to the downloadFiles map?
			return dlFile, nil, err
		}
	}

	dlFile, dlStream, err = dldr.Get(ctx)
	switch {
	case err != nil:
		return "", nil, err
	case dlFile != "":
		// The file is already on disk, so we can just return it.
		fs.downloadedFiles[src.Handle] = dlFile
		return dlFile, nil, nil
	default:
		// A new download stream was created. Add it to Files.streams,
		// and return the stream.
		fs.streams[src.Handle] = dlStream
		return "", dlStream, nil
	}
}

// downloadPaths returns the paths for src's download cache dir and
// cache body file. It is not guaranteed that the returned paths exist.
func (fs *Files) downloadPaths(src *source.Source) (dlDir, dlFile string, err error) {
	var cacheDir string
	cacheDir, err = fs.CacheDirFor(src)
	if err != nil {
		return "", dlFile, err
	}

	// Note: we depend on internal knowledge of the downloader impl here,
	// which is not great. It might be better to implement a function
	// in pkg downloader.
	dlDir = filepath.Join(cacheDir, "download", checksum.Sum([]byte(src.Location)))
	dlFile = filepath.Join(dlDir, "main", "body")
	return dlDir, dlFile, nil
}

// downloaderFor returns the downloader.Downloader for src, creating
// and caching it if necessary.
func (fs *Files) downloaderFor(ctx context.Context, src *source.Source) (*downloader.Downloader, error) {
	dl, ok := fs.downloaders[src.Handle]
	if ok {
		return dl, nil
	}

	dlDir, _, err := fs.downloadPaths(src)
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
	fs.downloaders[src.Handle] = dl
	return dl, nil
}

func (fs *Files) httpClientFor(ctx context.Context, src *source.Source) *http.Client {
	o := options.Merge(options.FromContext(ctx), src.Options)
	return httpz.NewClient(httpz.DefaultUserAgent,
		httpz.OptRequestTimeout(OptHTTPRequestTimeout.Get(o)),
		httpz.OptResponseTimeout(OptHTTPResponseTimeout.Get(o)),
		httpz.OptInsecureSkipVerify(OptHTTPSInsecureSkipVerify.Get(o)),
	)
}
