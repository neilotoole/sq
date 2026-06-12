package explore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/record"
)

// drainMsgs reads up to n messages from a channel or fails the test
// on timeout.
func drainMsgs(t *testing.T, ch <-chan any, n int) []any {
	t.Helper()
	out := make([]any, 0, n)
	timeout := time.After(2 * time.Second)
	for len(out) < n {
		select {
		case m, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, m)
		case <-timeout:
			t.Fatalf("timed out waiting for messages; got %d/%d", len(out), n)
		}
	}
	return out
}

func TestPreviewWriter_EmitsMetaThenRows(t *testing.T) {
	msgCh := make(chan any, 16)
	stopped := false
	pw := newPreviewWriter("@x", "actor", 3, func(m any) { msgCh <- m }, func() { stopped = true })

	recMeta := record.Meta{}
	cancelled := false
	cancel := func() { cancelled = true }
	recCh, errCh, err := pw.Open(context.Background(), cancel, recMeta)
	require.NoError(t, err)

	go func() {
		recCh <- record.Record{nil}
		recCh <- record.Record{nil}
		close(recCh)
	}()

	// Expect a meta msg first, then at least one row batch.
	msgs := drainMsgs(t, msgCh, 2)
	_, ok := msgs[0].(previewMetaLoadedMsg)
	require.True(t, ok, "first msg should be meta; got %T", msgs[0])
	_, ok = msgs[1].(previewRowsAppendedMsg)
	require.True(t, ok, "second msg should be rows; got %T", msgs[1])

	written, err := pw.Wait()
	require.NoError(t, err)
	require.Equal(t, int64(2), written)
	_ = errCh
	require.False(t, stopped, "should not stop the pipeline when caller closes recCh below cap")
	require.True(t, cancelled, "Wait must invoke the Open cancelFn per the RecordWriter contract")
}

func TestPreviewWriter_CancelsAtCap(t *testing.T) {
	stopped := false
	pw := newPreviewWriter("@x", "actor", 2, func(_ any) {}, func() { stopped = true })

	cancelled := false
	cancel := func() { cancelled = true }
	recCh, _, err := pw.Open(context.Background(), cancel, record.Meta{})
	require.NoError(t, err)

	go func() {
		recCh <- record.Record{nil}
		recCh <- record.Record{nil}
		// Writer should stop and drain after the 2nd record.
		// A 3rd send simulates the upstream producer not yet noticing
		// the stop — the writer's drain goroutine should absorb it
		// without blocking. We send with a goroutine + select so the
		// test isn't itself blocking forever.
		select {
		case recCh <- record.Record{nil}:
		case <-time.After(200 * time.Millisecond):
		}
	}()

	written, err := pw.Wait()
	require.NoError(t, err)
	require.Equal(t, int64(2), written, "should have written exactly capRows records")
	require.True(t, stopped, "writer must stop the pipeline at row cap")
	require.True(t, cancelled, "Wait must invoke the Open cancelFn per the RecordWriter contract")
}

func TestPreviewWriter_NoRowsBeforeClose(t *testing.T) {
	msgCh := make(chan any, 16)
	pw := newPreviewWriter("@x", "actor", 100, func(m any) { msgCh <- m }, func() {})

	recCh, errCh, err := pw.Open(context.Background(), func() {}, record.Meta{})
	require.NoError(t, err)
	_ = errCh

	close(recCh)
	written, err := pw.Wait()
	require.NoError(t, err)
	require.Equal(t, int64(0), written)
}
