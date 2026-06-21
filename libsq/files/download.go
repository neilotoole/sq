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
		nil,
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
		nil,
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
		nil,
		false,
		"Skip HTTPS TLS verification",
		"Skip HTTPS TLS verification. Useful when downloading against self-signed certs.",
		options.TagSource,
	)
	OptDownloadContinueOnError = downloader.OptContinueOnError
	OptDownloadCache           = downloader.OptCache
)

// maybeStartDownload returns the remote file for src, either as the path to an
// already-cached file (dlFile) or as a stream of an in-progress download
// (dlStream). Exactly one of dlFile, dlStream, err is non-nil.
//
// Resolution order:
//   - If src's download path is already recorded in Files.downloadedFiles, it
//     is returned.
//   - If a download stream for src is already registered in Files.streams, it
//     is returned. Note that a stream stays registered for the lifetime of this
//     Files instance: it is not removed when the download completes. So a
//     completed download is re-served from its stream here, and (because the
//     stream lingers) cachedBackingSourceForFile conservatively treats src as
//     still downloading.
//   - Otherwise a downloader is consulted. If checkFresh is false and the file
//     is already cached on disk, that path is returned (but is not added to
//     downloadedFiles). Otherwise the downloader either returns an
//     already-cached file path (which is added to downloadedFiles) or starts a
//     new download, whose stream is added to Files.streams and returned.
//
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
		// A new download stream was created. Append it to Files.streams,
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
