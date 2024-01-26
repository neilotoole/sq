// Package files contains functionality for dealing with files,
// including remote files (e.g. HTTP). The files.Files type
// is the central API for interacting with files.
package files

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/neilotoole/streamcache"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/ioz/lockfile"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/files/downloader"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/location"
)

// Files is the centralized API for interacting with files.
//
// Why does Files exist? There's a need for functionality to
// transparently get a Reader for remote or local files, and most importantly,
// an ability for multiple goroutines to read/sample a file while
// it's being read (mainly to "sample" the file type, e.g. to determine
// if it's an XLSX file etc.). See: Files.NewReader.
//
// TODO: move Files to its own pkg, e.g. files.New, *files.Files, etc.
type Files struct {
	mu          sync.Mutex
	log         *slog.Logger
	cacheDir    string
	tempDir     string
	clnup       *cleanup.Cleanup
	optRegistry *options.Registry

	// mStreams is a map of source handles to streamcache.Stream
	// instances: this is used to cache non-regular-file streams, such
	// as stdin, or in-progress downloads. This streamcache mechanism
	// permits multiple readers to access the stream. For example:
	//
	//  $ cat FILE | sq .data
	//
	// In this scenario FILE is provided on os.Stdin, but sq needs
	// to read the stdin stream several times: first, to detect the type
	// of data on stdin (via Files.DetectStdinType), and then to actually
	// ingest the data.
	mStreams map[string]*streamcache.Stream

	// cfgLockFn is the lock func for sq's config.
	cfgLockFn lockfile.LockFunc

	// mDownloaders is a cache map of source handles to the downloader for
	// that source: we only ever want to have one downloader per source.
	// See Files.downloaderFor.
	mDownloaders map[string]*downloader.Downloader

	// downloadersWg is used to wait for any downloader goroutines
	// to complete. See
	downloadersWg *sync.WaitGroup

	// mDownloadedFiles is a map of source handles to filepath of
	// already downloaded files. Consulting this map allows Files
	// to directly serve the downloaded file from disk instead of
	// using download.Download (which typically makes an HTTP
	// call to check the freshness of an already downloaded file).
	mDownloadedFiles map[string]string

	// detectFns is the set of functions that can detect
	// the type of a file.
	detectFns []TypeDetectFunc
}

// New returns a new Files instance. If cleanFscache is true, the fscache
// is cleaned on Files.Close.
func New(ctx context.Context, optReg *options.Registry, cfgLock lockfile.LockFunc,
	tmpDir, cacheDir string,
) (*Files, error) {
	log := lg.FromContext(ctx)
	log.Debug("Creating new Files instance", "tmp_dir", tmpDir, "cache_dir", cacheDir)

	if optReg == nil {
		optReg = &options.Registry{}
	}

	fs := &Files{
		log:              lg.FromContext(ctx),
		optRegistry:      optReg,
		cacheDir:         cacheDir,
		tempDir:          tmpDir,
		cfgLockFn:        cfgLock,
		clnup:            cleanup.New(),
		mDownloaders:     map[string]*downloader.Downloader{},
		downloadersWg:    &sync.WaitGroup{},
		mDownloadedFiles: map[string]string{},
		mStreams:         map[string]*streamcache.Stream{},
	}

	return fs, nil
}

// Filesize returns the file size of src.Location. If the source is being
// ingested asynchronously, this function may block until loading completes.
// An error is returned if src is not a document/file source.
// For remote files, this method should only be invoked after the file has
// completed downloading (e.g. after ingestion), or an error may be returned.
func (fs *Files) Filesize(ctx context.Context, src *source.Source) (size int64, err error) {
	switch location.TypeOf(src.Location) {
	case location.TypeLocalFile:
		var fi os.FileInfo
		if fi, err = os.Stat(src.Location); err != nil {
			return 0, errz.Err(err)
		}
		return fi.Size(), nil

	case location.TypeStdin:
		fs.mu.Lock()
		stdinStream, ok := fs.mStreams[source.StdinHandle]
		fs.mu.Unlock()
		if !ok {
			// This is a programming error; probably should panic here.
			return 0, errz.Errorf("stdin not present in cache")
		}
		var total int
		if total, err = stdinStream.Total(ctx); err != nil {
			return 0, err
		}
		return int64(total), nil

	case location.TypeRemoteFile:
		fs.mu.Lock()

		// First check if the file is already downloaded
		// and in File's list of downloaded files.
		dlFile, ok := fs.mDownloadedFiles[src.Handle]
		if ok {
			// The file is already downloaded.
			fs.mu.Unlock()
			var fi os.FileInfo
			if fi, err = os.Stat(dlFile); err != nil {
				return 0, errz.Err(err)
			}
			return fi.Size(), nil
		}

		// It's not in File's list of downloaded files, so
		// check if there's an active download stream.
		dlStream, ok := fs.mStreams[src.Handle]
		if ok {
			fs.mu.Unlock()
			var total int
			if total, err = dlStream.Total(ctx); err != nil {
				return 0, err
			}
			return int64(total), nil
		}

		// Finally, we turn to the downloader.
		var dl *downloader.Downloader
		dl, err = fs.downloaderFor(ctx, src)
		fs.mu.Unlock()
		if err != nil {
			return 0, err
		}

		// dl.Filesize will fail if the file has not been downloaded yet, which
		// means that the source has not been ingested; but Files.Filesize should
		// not have been invoked before ingestion.
		return dl.Filesize(ctx)

	case location.TypeSQL:
		// Should be impossible.
		return 0, errz.Errorf("invalid to get size of SQL source: %s", src.Handle)

	default:
		// Should be impossible.
		return 0, errz.Errorf("unknown source location type: %s", src)
	}
}

// AddStdin copies f to fs's cache: the stdin data in f
// is later accessible via fs.NewReader(src) where src.Handle
// is StdinHandle; f's type can be detected via DetectStdinType.
// Note that f is ultimately closed by a goroutine spawned by
// this method, but may not be closed at the time of return.
func (fs *Files) AddStdin(ctx context.Context, f *os.File) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, ok := fs.mStreams[source.StdinHandle]; ok {
		return errz.Errorf("%s already added to reader cache", source.StdinHandle)
	}

	stream := streamcache.New(f)
	fs.mStreams[source.StdinHandle] = stream
	lg.FromContext(ctx).With(lga.Handle, source.StdinHandle, lga.File, f.Name()).
		Debug("Added stdin to reader cache")
	return nil
}

// filepath returns the file path of src.Location. An error is returned
// if the source's driver type is not a document type (e.g. it is a
// SQL driver). If src is a remote (http) location, the returned filepath
// is that of the cached download file. If that file is not present, an
// error is returned.
func (fs *Files) filepath(src *source.Source) (string, error) {
	switch location.TypeOf(src.Location) {
	case location.TypeLocalFile:
		return src.Location, nil
	case location.TypeRemoteFile:
		dlDir, err := fs.downloadDirFor(src)
		if err != nil {
			return "", err
		}

		// FIXME: We shouldn't be depending on knowledge of the internal
		// workings of download.Download here. Instead we should call
		// some method?
		dlFile := filepath.Join(dlDir, "main", "body")
		if !ioz.FileAccessible(dlFile) {
			return "", errz.Errorf("remote file for %s not downloaded at: %s", src.Handle, dlFile)
		}
		return dlFile, nil
	case location.TypeSQL:
		return "", errz.Errorf("cannot get filepath of SQL source: %s", src.Handle)
	case location.TypeStdin:
		return "", errz.Errorf("cannot get filepath of stdin source: %s", src.Handle)
	default:
		return "", errz.Errorf("unknown source location type for %s: %s", src.Handle, location.Redact(src.Location))
	}
}

// NewReader returns a new io.ReadCloser for src.Location. Arg ingesting is
// a performance hint that indicates that the reader is being used to ingest
// data (as opposed to, say, sampling the data for type detection). It's an
// error to invoke NewReader for a src after having invoked it for the same
// src with ingesting=true.
//
// If src.Handle is StdinHandle, AddStdin must first have been invoked.
//
// The caller must close the reader.
func (fs *Files) NewReader(ctx context.Context, src *source.Source, ingesting bool) (io.ReadCloser, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.newReader(ctx, src, ingesting)
}

// newReader returns a new io.ReadCloser for src.Location. If finalRdr is
// true, and src is using a streamcache.Stream, that cache is sealed after
// the reader is created: newReader must not be called again for src in the
// lifetime of this Files instance.
func (fs *Files) newReader(ctx context.Context, src *source.Source, finalRdr bool) (io.ReadCloser, error) {
	lg.FromContext(ctx).Debug("Files.NewReader", lga.Src, src, "final_reader", finalRdr)

	loc := src.Location
	switch location.TypeOf(loc) {
	case location.TypeUnknown:
		return nil, errz.Errorf("unknown source location type: %s", loc)
	case location.TypeSQL:
		return nil, errz.Errorf("invalid to read SQL source: %s", loc)
	case location.TypeLocalFile:
		return errz.Return(os.Open(loc))
	case location.TypeStdin:
		stdinStream, ok := fs.mStreams[source.StdinHandle]
		if !ok {
			// This is a programming error: AddStdin should have been invoked first.
			// Probably should panic here.
			return nil, errz.New("@stdin not cached: has AddStdin been invoked yet?")
		}
		r := stdinStream.NewReader(ctx)
		if finalRdr {
			stdinStream.Seal()
		}
		return r, nil
	default:
		// It's a remote file.
	}

	// Is there a download in progress?
	if dlStream, ok := fs.mStreams[src.Handle]; ok {
		r := dlStream.NewReader(ctx)
		if finalRdr {
			dlStream.Seal()
		}
		return r, nil
	}

	// Is the file already downloaded?
	if fp, ok := fs.mDownloadedFiles[src.Handle]; ok {
		return errz.Return(os.Open(fp))
	}

	// One of dlFile, dlStream, or err is guaranteed to be non-nil.
	dlFile, dlStream, err := fs.maybeStartDownload(ctx, src, false)
	switch {
	case err != nil:
		return nil, err
	case dlFile != "":
		return errz.Return(os.Open(dlFile))
	case dlStream != nil:
		r := dlStream.NewReader(ctx)
		if finalRdr {
			dlStream.Seal()
		}
		return r, nil
	default:
		// Should be impossible.
		panic("Files.maybeStartDownload returned all nils")
	}
}

// Ping implements a ping mechanism for document
// sources (local or remote files).
func (fs *Files) Ping(ctx context.Context, src *source.Source) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	switch location.TypeOf(src.Location) {
	case location.TypeStdin:
		// Stdin is always available.
		return nil
	case location.TypeLocalFile:
		if _, err := os.Stat(src.Location); err != nil {
			return errz.Wrapf(err, "ping: failed to stat file source %s: %s", src.Handle, src.Location)
		}
		return nil

	case location.TypeRemoteFile:
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, src.Location, nil)
		if err != nil {
			return errz.Wrapf(err, "ping: %s", src.Handle)
		}

		c := fs.httpClientFor(ctx, src)
		resp, err := c.Do(req) //nolint:bodyclose
		if err != nil {
			return errz.Wrapf(err, "ping: %s", src.Handle)
		}

		// This shouldn't be necessary because the request method was HEAD,
		// so resp.Body should be nil?
		lg.WarnIfCloseError(fs.log, lgm.CloseHTTPResponseBody, resp.Body)

		if resp.StatusCode != http.StatusOK {
			return errz.Errorf("ping: %s: expected {%s} but got {%s}",
				src.Handle, httpz.StatusText(http.StatusOK), httpz.StatusText(resp.StatusCode))
		}
		return nil

	default:
		// Shouldn't happen
		return errz.Errorf("ping: %s is not a document source", src.Handle)
	}
}

// Close closes any open resources and waits for any goroutines
// to complete.
func (fs *Files) Close() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.log.Debug("Files.Close: waiting for any downloads to complete")
	fs.downloadersWg.Wait()

	err := fs.clnup.Run()
	err = errz.Append(err, errz.Wrap(os.RemoveAll(fs.tempDir), "remove files temp dir"))
	fs.doCacheSweep()

	return err
}

// CreateTemp creates a new temporary file fs's temp dir with the given
// filename pattern, as per the os.CreateTemp docs. If arg clean is
// true, the file is added to the cleanup sequence invoked by fs.Close.
// It is the callers responsibility to close the returned file.
func (fs *Files) CreateTemp(pattern string, clean bool) (*os.File, error) {
	f, err := os.CreateTemp(fs.tempDir, pattern)
	if err != nil {
		return nil, errz.Err(err)
	}

	if clean {
		fname := f.Name()
		fs.clnup.AddE(func() error {
			return errz.Err(os.Remove(fname))
		})
	}
	return f, nil
}

// NewReaderFunc returns a func that returns an io.ReadCloser. The caller
// is responsible for closing the returned io.ReadCloser.
type NewReaderFunc func(ctx context.Context) (io.ReadCloser, error)
