package output

import (
	"context"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/driver"
)

// RecordWriterAdapter implements libsq.RecordWriter and
// wraps an output.RecordWriter instance, providing a
// bridge between the asynchronous libsq.RecordWriter and
// synchronous output.RecordWriter interfaces.
//
// Note that a writer implementation such as the JSON or
// CSV writer could directly implement libsq.RecordWriter.
// But that interface is non-trivial to implement, hence
// this bridge type.
//
// The FlushAfterN and FlushAfterDuration fields control
// flushing of the writer.
type RecordWriterAdapter struct {
	rw       RecordWriter
	wg       *sync.WaitGroup
	recCh    chan record.Record
	errCh    chan error
	errs     []error
	written  *atomic.Int64
	cancelFn context.CancelFunc

	// FlushAfterN indicates that the writer's Flush method
	// should be invoked after N invocations of WriteRecords.
	// A value of 0 will flush every time a record is written.
	// Set to -1 to disable.
	FlushAfterN int64

	// FlushAfterDuration controls whether the writer's Flush method
	// is invoked periodically. A duration <= 0 disables periodic flushing.
	FlushAfterDuration time.Duration
}

// NewRecordWriterAdapter returns a new RecordWriterAdapter.
// The size of the internal buffer is controlled by driver.OptTuningRecChanSize.
func NewRecordWriterAdapter(ctx context.Context, rw RecordWriter) *RecordWriterAdapter {
	chSize := driver.OptTuningRecChanSize.Get(options.FromContext(ctx))
	recCh := make(chan record.Record, chSize)

	return &RecordWriterAdapter{rw: rw, recCh: recCh, wg: &sync.WaitGroup{}, written: atomic.NewInt64(0)}
}

// Open implements libsq.RecordWriter.
func (w *RecordWriterAdapter) Open(ctx context.Context, cancelFn context.CancelFunc,
	recMeta record.Meta,
) (chan<- record.Record, <-chan error, error) {
	lg.FromContext(ctx).Debug("Open RecordWriterAdapter", "fields", recMeta)
	w.cancelFn = cancelFn

	err := w.rw.Open(ctx, recMeta)
	if err != nil {
		return nil, nil, err
	}

	// errCh has size 2 because that's the maximum number of
	// errs that could be sent. Typically only one err is sent,
	// but in the case of ctx.Done, we send ctx.Err, followed
	// by any error returned by r.rw.Close.
	w.errCh = make(chan error, 2)
	w.wg.Add(1)

	go func() {
		defer func() {
			w.wg.Done()
			close(w.errCh)
		}()

		var lastFlushN, recN int64
		var flushTimer *time.Timer
		var flushCh <-chan time.Time

		if w.FlushAfterDuration > 0 {
			flushTimer = time.NewTimer(w.FlushAfterDuration)
			flushCh = flushTimer.C
			defer flushTimer.Stop()
		}

		for {
			select {
			case <-ctx.Done():
				w.addErrs(ctx.Err(), w.rw.Close(ctx))
				return

			case <-flushCh:
				// The flushTimer has expired, time to flush.
				err = w.rw.Flush(ctx)
				if err != nil {
					w.addErrs(err)
					return
				}
				lastFlushN = recN
				flushTimer.Reset(w.FlushAfterDuration)
				continue

			case rec := <-w.recCh:
				if rec == nil { // no more results on recCh, it has been closed
					err = w.rw.Close(ctx)
					if err != nil {
						w.addErrs()
					}
					return
				}

				// rec is not nil, therefore we write it out.

				// We could accumulate a bunch of recs into a slice here,
				// but we'll worry about that if benchmarking shows it'll matter.
				writeErr := w.rw.WriteRecords(ctx, []record.Record{rec})
				if writeErr != nil {
					w.addErrs(writeErr)
					return
				}

				recN = w.written.Inc()

				// Check if we should flush
				if w.FlushAfterN >= 0 && (recN-lastFlushN >= w.FlushAfterN) {
					err = w.rw.Flush(ctx)
					if err != nil {
						w.addErrs(err)
						return
					}
					lastFlushN = recN

					if flushTimer != nil {
						// Reset the timer, but we need to stop and drain it first.
						// See the timer.Reset docs.
						if !flushTimer.Stop() {
							<-flushTimer.C
						}

						flushTimer.Reset(w.FlushAfterDuration)
					}
				}

				// If we got this far, we successfully wrote rec to rw.
				// Therefore continue to wait/select for the next
				// element on recCh (or for recCh to close)
				// or for ctx.Done indicating timeout or cancel etc.
				continue
			}
		}
	}()

	return w.recCh, w.errCh, nil
}

// Wait implements libsq.RecordWriter.
func (w *RecordWriterAdapter) Wait() (written int64, err error) {
	w.wg.Wait()
	if w.cancelFn != nil {
		w.cancelFn()
	}

	return w.written.Load(), errz.Combine(w.errs...)
}

// addErrs handles any non-nil err in errs by appending it to w.errs
// and sending it on w.errCh.
func (w *RecordWriterAdapter) addErrs(errs ...error) {
	for _, err := range errs {
		if err != nil {
			w.errs = append(w.errs, err)
			w.errCh <- err
		}
	}
}
