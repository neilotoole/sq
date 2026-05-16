package explore

import (
	"context"
	"sync"

	"github.com/neilotoole/sq/libsq/core/record"
)

// previewWriter is a libsq.RecordWriter that streams up to capRows
// records into the explore TUI via dispatchFn (the tea.Program's
// Send method, captured at construction time).
//
// Contract:
//   - Open returns the inbound channel the pipeline writes into.
//   - The writer's goroutine reads up to capRows records, dispatching
//     batches via previewRowsAppendedMsg, then calls cancelFn to abort
//     the upstream pipeline cleanly.
//   - The first message dispatched is always previewMetaLoadedMsg
//     (carrying the record.Meta).
//   - On error, previewErrMsg is dispatched and the writer terminates.
type previewWriter struct {
	dispatch func(any) // typically tea.Program.Send.
	cancelFn context.CancelFunc
	recCh    chan record.Record
	errCh    chan error
	doneCh   chan struct{}
	waitErr  error
	handle   string
	table    string
	capRows  int64
	written  int64
	mu       sync.Mutex
	started  bool
}

func newPreviewWriter(handle, table string, capRows int, dispatch func(any)) *previewWriter {
	if capRows < 1 {
		capRows = 100
	}
	return &previewWriter{
		handle:   handle,
		table:    table,
		capRows:  int64(capRows),
		dispatch: dispatch,
		errCh:    make(chan error, 1),
		doneCh:   make(chan struct{}),
	}
}

// Open satisfies the libsq.RecordWriter interface. See libsq.go for the
// full contract.
func (pw *previewWriter) Open(
	ctx context.Context,
	cancelFn context.CancelFunc,
	recMeta record.Meta,
) (chan<- record.Record, <-chan error, error) {
	pw.mu.Lock()
	pw.recCh = make(chan record.Record, 8)
	pw.cancelFn = cancelFn
	pw.started = true
	pw.mu.Unlock()

	pw.dispatch(previewMetaLoadedMsg{
		handle:    pw.handle,
		tableName: pw.table,
		recMeta:   recMeta,
	})

	go pw.run(ctx)

	return pw.recCh, pw.errCh, nil
}

func (pw *previewWriter) run(ctx context.Context) {
	defer close(pw.doneCh)
	defer close(pw.errCh)

	batch := make([]record.Record, 0, 16)
	flush := func(done bool) {
		if len(batch) == 0 && !done {
			return
		}
		pw.dispatch(previewRowsAppendedMsg{
			handle:    pw.handle,
			tableName: pw.table,
			rows:      append([]record.Record(nil), batch...),
			done:      done,
		})
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush(true)
			return
		case rec, ok := <-pw.recCh:
			if !ok {
				// Upstream closed normally — no more records.
				flush(true)
				return
			}
			pw.written++
			batch = append(batch, rec)
			if pw.written >= pw.capRows {
				flush(true)
				// Tell the upstream pipeline we're done.
				if pw.cancelFn != nil {
					pw.cancelFn()
				}
				// Drain any remaining records sent before the
				// pipeline sees the cancel, to avoid blocking the
				// producer.
				go func() {
					for range pw.recCh {
						_ = struct{}{}
					}
				}()
				return
			}
			// Flush incremental batches every 16 rows.
			if len(batch) >= 16 {
				flush(false)
			}
		}
	}
}

// Wait satisfies the libsq.RecordWriter interface.
func (pw *previewWriter) Wait() (int64, error) {
	pw.mu.Lock()
	started := pw.started
	pw.mu.Unlock()
	if !started {
		return 0, nil
	}
	<-pw.doneCh
	return pw.written, pw.waitErr
}
