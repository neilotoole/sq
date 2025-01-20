// Package files contains functionality for dealing with files,
// including remote files (e.g. HTTP). The files.Files type
// is the central API for interacting with files.
package files

import (
	"context"
	"fmt"
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
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tuning"
	"github.com/neilotoole/sq/libsq/files/internal/downloader"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/location"
)

// Files is the centralized API for interacting with files. It provides
// a uniform mechanism for reading files, whether from local disk, stdin,
// or remote HTTP.
type Files struct {
	// fileBufs provides  file-backed buffers, as used by Files.NewBuffer.
	fileBufs *ioz.Buffers

	log         *slog.Logger
	clnup       *cleanup.Cleanup
	optRegistry *options.Registry

	// streams is a map of source handles to streamcache.Stream
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
	streams map[string]*streamcache.Stream

	// downloaders is a cache map of source handles to the downloader for
	// that source: we only ever want to have one downloader per source.
	// See Files.downloaderFor.
	downloaders map[string]*downloader.Downloader

	// downloadedFiles is a map of source handles to filepath of
	// already downloaded files. Consulting this map allows Files
	// to directly serve the downloaded file from disk instead of
	// using downloader.Downloader (which typically makes an HTTP
	// call to check the freshness of an already downloaded file).
	downloadedFiles map[string]string

	// cfgLockFn is the lock func for sq's config.
	cfgLockFn lockfile.LockFunc

	cacheDir string
	tempDir  string

	// detectFns is the set of functions that can detect
	// the type of a file.
	detectFns []TypeDetectFunc

	// mu guards access to Files' internals.
	mu sync.Mutex
}

// NewBuffer returns a new [ioz.Buffer] instance which may be in-memory or
// on-disk, or both, for use as a temporary buffer for potentially large data
// that may not fit in memory. The caller MUST invoke [ioz.Buffer.Close] on the
// returned buffer when done.
func (fs *Files) NewBuffer() ioz.Buffer {
	return fs.fileBufs.NewMem2Disk()
}

// New returns a new Files instance. The caller must invoke Files.Close
// when done with the instance.
func New(ctx context.Context, optReg *options.Registry, cfgLock lockfile.LockFunc,
	tmpDir, cacheDir string,
) (*Files, error) {
	log := lg.FromContext(ctx)
	log.Debug("Creating new Files instance", "tmp_dir", tmpDir, "cache_dir", cacheDir)

	if optReg == nil {
		optReg = &options.Registry{}
	}

	fs := &Files{
		log:             lg.FromContext(ctx),
		optRegistry:     optReg,
		cacheDir:        cacheDir,
		tempDir:         tmpDir,
		cfgLockFn:       cfgLock,
		clnup:           cleanup.New(),
		downloaders:     map[string]*downloader.Downloader{},
		downloadedFiles: map[string]string{},
		streams:         map[string]*streamcache.Stream{},
	}

	var err error
	if fs.fileBufs, err = ioz.NewBuffers(
		filepath.Join(tmpDir, fmt.Sprintf("filebuf_%d_%s", os.Getpid(), stringz.Uniq8())),
		int(tuning.OptBufSpillLimit.Get(options.FromContext(ctx)).Bytes()), //nolint:gosec // ignore overflow concern
	); err != nil {
		return nil, err
	}

	return fs, nil
}

// Filesize returns the file size of src.Location. If the source is being
// ingested asynchronously, this function may block until loading completes.
// An error is returned if src is not a document/file source.
func (fs *Files) Filesize(ctx context.Context, src *source.Source) (size int64, err error) {
	switch location.TypeOf(src.Location) {
	case location.TypeFile:
		var fi os.FileInfo
		if fi, err = os.Stat(src.Location); err != nil {
			return 0, errz.Err(err)
		}
		return fi.Size(), nil

	case location.TypeStdin:
		fs.mu.Lock()
		stdinStream, ok := fs.streams[source.StdinHandle]
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

	case location.TypeHTTP:
		fs.mu.Lock()

		// First check if the file is already downloaded
		// and in File's list of downloaded files.
		dlFile, ok := fs.downloadedFiles[src.Handle]
		if ok {
			// The file is already downloaded.
			defer fs.mu.Unlock()
			return ioz.Filesize(dlFile)
		}

		// It's not in the list of downloaded files, so
		// check if there's an active download stream.
		dlStream, ok := fs.streams[src.Handle]
		if ok {
			fs.mu.Unlock()

			var total int
			// Block until the download completes.
			if total, err = dlStream.Total(ctx); err != nil {
				return 0, err
			}
			return int64(total), nil
		}

		// Finally, we turn to the downloader.
		defer fs.mu.Unlock()
		var dl *downloader.Downloader
		if dl, err = fs.downloaderFor(ctx, src); err != nil {
			return 0, err
		}

		if dlFile, err = dl.CacheFile(ctx); err != nil {
			return 0, err
		}

		return ioz.Filesize(dlFile)

	case location.TypeSQL:
		// Should be impossible.
		return 0, errz.Errorf("invalid to get size of SQL source: %s", src.Handle)

	default:
		// Should be impossible.
		return 0, errz.Errorf("unknown source location type: %s", src)
	}
}

// AddStdin copies f to fs's cache: the stdin data in f
// is later accessible via Files.NewReader(src) where src.Handle
// is source.StdinHandle; f's type can be detected via DetectStdinType.
func (fs *Files) AddStdin(ctx context.Context, f *os.File) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, ok := fs.streams[source.StdinHandle]; ok {
		return errz.Errorf("%s already added to reader cache", source.StdinHandle)
	}

	stream := streamcache.New(f)
	fs.streams[source.StdinHandle] = stream
	lg.FromContext(ctx).With(lga.Handle, source.StdinHandle, lga.File, f.Name()).
		Debug("Added stdin to reader cache")
	return nil
}

// filepath returns the file path of src.Location. An error is returned
// if the source's driver type is not a document type (e.g. it is a
// SQL driver). If src is a remote (http) location, the returned filepath
// is that of the cached download file. It's not guaranteed that that
// file exists.
func (fs *Files) filepath(src *source.Source) (string, error) {
	switch location.TypeOf(src.Location) {
	case location.TypeFile:
		return src.Location, nil
	case location.TypeHTTP:
		_, dlFile, err := fs.downloadPaths(src)
		if err != nil {
			return "", err
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
	log := lg.FromContext(ctx).With(lga.Src, src)
	lg.Depth(log, slog.LevelDebug, 2, "Invoked Files.NewReader", "final_reader", finalRdr)

	loc := src.Location
	switch location.TypeOf(loc) {
	case location.TypeUnknown:
		return nil, errz.Errorf("unknown source location type: %s", loc)
	case location.TypeSQL:
		return nil, errz.Errorf("invalid to read SQL source: %s", loc)
	case location.TypeFile:
		return errz.Return(os.Open(loc))
	case location.TypeStdin:
		stdinStream, ok := fs.streams[source.StdinHandle]
		if !ok {
			// This is a programming error: AddStdin should have been invoked first.
			// Probably should panic here.
			return nil, errz.New("@stdin not cached: has AddStdin been invoked yet?")
		}
		r := stdinStream.NewReader(ctx)
		if finalRdr {
			lg.FromContext(ctx).Debug("Sealing source stream")
			stdinStream.Seal()
		}
		return r, nil
	default:
		// It's a remote file.
	}

	// Is there a download in progress?
	if dlStream, ok := fs.streams[src.Handle]; ok {
		r := dlStream.NewReader(ctx)
		if finalRdr {
			log.Debug("Sealing download source stream")
			dlStream.Seal()
		}
		return r, nil
	}

	// Is the file already downloaded?
	if fp, ok := fs.downloadedFiles[src.Handle]; ok {
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
			log.Debug("Sealing download source stream")
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
	case location.TypeFile:
		if _, err := os.Stat(src.Location); err != nil {
			return errz.Wrapf(err, "ping: failed to stat file source %s: %s", src.Handle, src.Location)
		}
		return nil

	case location.TypeHTTP:
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

// Close closes any open resources.
func (fs *Files) Close() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	var err error
	for _, stream := range fs.streams {
		select {
		case <-stream.Done():
		// Nothing to do, it's already closed.
		default:
			if c, ok := stream.Source().(io.Closer); ok {
				err = errz.Append(err, c.Close())
			}
		}
	}

	err = errz.Append(err, fs.fileBufs.Close())
	err = errz.Append(err, fs.clnup.Run())
	err = errz.Append(err, errz.Wrap(os.RemoveAll(fs.tempDir), "remove files temp dir"))

	fs.doCacheSweep()

	return err
}

// CreateTemp creates a new temporary file in fs's temp dir with the given
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
