// This file defines the [Handler] callback interface and [SinkHandler]
// test helper for receiving download events from [Downloader.Get].

package downloader

import (
	"io"
	"log/slog"
	"sync"

	"github.com/neilotoole/streamcache"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
)

// Handler defines callbacks for receiving download events from [Downloader.Get].
//
// The Handler provides a way for callers to react to download status early,
// before [Downloader.Get] returns. This enables streaming use cases where
// processing can begin before the entire download completes.
//
// # Callback Guarantees
//
// Exactly one of the three callbacks (Cached, Uncached, or Error) will be
// invoked, exactly once, during a call to [Downloader.Get]:
//
//   - [Handler.Cached]: Called when a valid cached file exists
//   - [Handler.Uncached]: Called when download begins (no cache or stale cache)
//   - [Handler.Error]: Called if an error occurs before Cached/Uncached
//
// All callbacks may be nil; nil callbacks are simply not invoked.
//
// # Streaming with Uncached
//
// When Uncached is called, the provided [streamcache.Stream] allows immediate
// reading of download bytes even though the download is still in progress.
// This enables:
//   - Processing data as it arrives (streaming)
//   - Multiple concurrent readers of the same download
//   - Notification when download completes via Stream.Filled()
//
// Any errors encountered during download will be returned when reading from
// the Stream, not via the Error callback (Error is only for pre-download errors).
type Handler struct {
	// Cached is invoked when the download is already cached on disk and the
	// cache is valid (fresh). The dlFile arg is the absolute path to the
	// cached file, which can be read immediately.
	Cached func(dlFile string)

	// Uncached is invoked when the download is not cached (or cache is stale)
	// and downloading has begun. The dlStream provides access to the download
	// bytes as they arrive (similar to http.Response.Body, but supporting
	// multiple concurrent readers).
	//
	// [Downloader.Get] continues downloading after Uncached returns. To wait
	// for download completion, use the channel returned by Stream.Filled().
	// To check for download errors, use Stream.Err() after Filled() signals.
	Uncached func(dlStream *streamcache.Stream)

	// Error is invoked if an error occurs before Cached or Uncached can be
	// called. This includes network errors, invalid URLs, and similar failures
	// that prevent the download from starting.
	//
	// Errors that occur during an active download (after Uncached is called)
	// are NOT reported via this callback; instead, they appear when reading
	// from the provided Stream.
	Error func(err error)
}

// SinkHandler is a [Handler] implementation that records all callback
// invocations for inspection. It is primarily used for testing.
//
// SinkHandler logs each callback invocation and appends the received values
// to the appropriate slice (Errors, Downloaded, or Streams). All operations
// are mutex-protected for safe concurrent use.
//
// Use [NewSinkHandler] to create a properly initialized instance.
// Use [SinkHandler.Reset] to clear recorded values between test cases.
type SinkHandler struct {
	// Handler embeds the callback functions. These are set by [NewSinkHandler]
	// to record invocations to the sink slices.
	Handler

	// log is used to log each callback invocation.
	log *slog.Logger

	// Errors records all errors received via Handler.Error, in order of receipt.
	Errors []error

	// Downloaded records the file paths received via Handler.Cached,
	// in order of receipt. Each entry is the path to a cached file.
	Downloaded []string

	// Streams records the streams received via Handler.Uncached,
	// in order of receipt. Each stream can be read to access download bytes.
	Streams []*streamcache.Stream

	// mu protects all slice fields for concurrent access.
	mu sync.Mutex
}

// Reset clears all recorded values and prepares the SinkHandler for reuse.
//
// This method:
//  1. Clears the Errors slice
//  2. Clears the Downloaded slice
//  3. Closes the source reader of each recorded Stream (to release resources)
//  4. Clears the Streams slice
//
// Reset is typically called between test cases to ensure a clean state.
// It is safe to call concurrently with other SinkHandler methods.
func (sh *SinkHandler) Reset() {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.Errors = nil
	sh.Downloaded = nil

	for _, stream := range sh.Streams {
		_ = stream.Source().(io.Closer).Close()
	}

	sh.Streams = nil
}

// NewSinkHandler creates a new [SinkHandler] with callbacks that record
// all invocations.
//
// The returned handler's callbacks:
//   - Cached: Logs the file path and appends to Downloaded
//   - Uncached: Logs "Uncached" and appends the stream to Streams
//   - Error: Logs the error and appends to Errors
//
// The provided logger is used for all logging. Pass a test logger (e.g.,
// from lgt.New(t)) for test output.
//
// Example:
//
//	log := lgt.New(t)
//	h := downloader.NewSinkHandler(log)
//	dl.Get(ctx, h)
//	require.Len(t, h.Downloaded, 1) // Check if cache was used
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
