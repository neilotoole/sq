package download

import (
	"github.com/neilotoole/streamcache"
	"log/slog"
	"sync"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
)

// Handler is a callback invoked by Download.Get. Exactly one of the
// handler functions will be invoked, exactly one time.
type Handler struct {
	// Cached is invoked when the download is already cached on disk. The
	// fp arg is the path to the downloaded file.
	Cached func(fp string)

	// Uncached is invoked when the download is not cached. The handler should
	// return an ioz.WriteErrorCloser, which the download contents will be written
	// to (as well as being written to the disk cache). On success, the dest
	// writer is closed. If an error occurs during download or writing,
	// WriteErrorCloser.Error is invoked (but Close is not invoked). If the
	// handler returns a nil dest, the Download will log a warning and return.
	//
	// FIXME: Update docs
	Uncached func(cache *streamcache.Cache)

	// Error is invoked on any error, other than writing to the destination
	// io.WriteCloser returned by Handler.Uncached, which has its own error
	// handling mechanism.
	Error func(err error)
}

// SinkHandler is a download.Handler that records the results of the callbacks
// it receives. This is useful for testing.
type SinkHandler struct {
	Handler
	mu  sync.Mutex
	log *slog.Logger

	// Errors records the errors received via Handler.Error.
	Errors []error

	// CachedFiles records the cached files received via Handler.Cached.
	CachedFiles []string

	// Uncached records in bytes.Buffer instances the data written
	// via Handler.Uncached.
	// FIXME: Update docs
	UncachedFiles []*streamcache.Cache

	// WriteErrors records the write errors received via Handler.Uncached.
	WriteErrors []error
}

// Reset resets the handler sinks.
func (sh *SinkHandler) Reset() {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.Errors = nil
	sh.CachedFiles = nil
	sh.UncachedFiles = nil
	sh.WriteErrors = nil
}

// NewSinkHandler returns a new SinkHandler.
func NewSinkHandler(log *slog.Logger) *SinkHandler {
	h := &SinkHandler{log: log}
	h.Cached = func(fp string) {
		log.Info("Cached", lga.File, fp)
		h.mu.Lock()
		defer h.mu.Unlock()
		h.CachedFiles = append(h.CachedFiles, fp)
	}

	h.Uncached = func(cache *streamcache.Cache) {
		log.Info("Uncached")
		h.mu.Lock()
		defer h.mu.Unlock()
		//buf := &bytes.Buffer{}
		//h.UncachedBufs = append(h.UncachedBufs, buf)
		//return ioz.NewFuncWriteErrorCloser(ioz.WriteCloser(buf), func(err error) {
		//	h.mu.Lock()
		//	defer h.mu.Unlock()
		//	h.WriteErrors = append(h.WriteErrors, err)
		//})
		h.UncachedFiles = append(h.UncachedFiles, cache)
	}

	h.Error = func(err error) {
		log.Info("Error", lga.Err, err)
		h.mu.Lock()
		defer h.mu.Unlock()
		h.Errors = append(h.Errors, err)
	}
	return h
}
