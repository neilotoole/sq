package downloader

import (
	"log/slog"
	"sync"

	"github.com/neilotoole/streamcache"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
)

// Handler is a callback invoked by Downloader.Get. Exactly one of the
// handler functions will be invoked, exactly one time. The handler is
// called as early as possible, and Downloader.Get may continue afterwards,
// e.g. to download the file. This mechanism allows the caller to start
// processing the download stream before the download completes.
type Handler struct {
	// Cached is invoked when the download is already cached on disk. The
	// fp arg is the path to the downloaded file.
	Cached func(fp string)

	// Uncached is invoked when the download is not cached on disk and
	// downloading has begun. The dlStream arg can be used to read the
	// bytes as would be returned from resp.Body. Downloader.Get will
	// continue the download process after Uncached returns. The caller
	// can wait on the download to complete using the channel returned
	// by streamcache.Stream's Filled method.
	Uncached func(dlStream *streamcache.Stream)

	// Error is invoked by Downloader.Get if an error occurs before Handler.Cached
	// or Handler.Uncached is invoked. If Uncached is invoked, any error from
	// reading the download resp.Body will be returned when reading
	// from the streamcache.Stream provided to Uncached.
	Error func(err error)
}

// SinkHandler is a downloader.Handler that records the results of the
// callbacks it receives. This is used for testing.
type SinkHandler struct {
	Handler
	mu  sync.Mutex
	log *slog.Logger

	// Errors records the errors received via Handler.Error.
	Errors []error

	// Downloaded records the already-downloaded files received via Handler.Cached.
	Downloaded []string

	// Streams records the streams received via Handler.Uncached.
	Streams []*streamcache.Stream
}

// Reset resets the handler sinks.
func (sh *SinkHandler) Reset() {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.Errors = nil
	sh.Downloaded = nil
	sh.Streams = nil
}

// NewSinkHandler returns a new SinkHandler.
func NewSinkHandler(log *slog.Logger) *SinkHandler {
	h := &SinkHandler{log: log}
	h.Cached = func(fp string) {
		log.Info("Cached", lga.File, fp)
		h.mu.Lock()
		defer h.mu.Unlock()
		h.Downloaded = append(h.Downloaded, fp)
	}

	h.Uncached = func(dlStream *streamcache.Stream) {
		log.Info("Uncached")
		h.mu.Lock()
		defer h.mu.Unlock()
		h.Streams = append(h.Streams, dlStream)
	}

	h.Error = func(err error) {
		log.Info("Error", lga.Err, err)
		h.mu.Lock()
		defer h.mu.Unlock()
		h.Errors = append(h.Errors, err)
	}
	return h
}
