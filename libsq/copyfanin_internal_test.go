package libsq

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// TestRunCopyFanIn_concurrentReadsSerialWrites verifies the core fan-in
// contract for a single-writer joindb (gh975/#995): the N source reads run
// concurrently, while the writes are serialized (one at a time) and executed
// in task order. Reads rendezvous at a barrier so their overlap is observed
// deterministically rather than depending on sleep timing.
func TestRunCopyFanIn_concurrentReadsSerialWrites(t *testing.T) {
	const n = 4

	var (
		readActive, readPeak atomic.Int32
		arrived              atomic.Int32
		allIn                = make(chan struct{})
	)
	read := func(ctx context.Context, _ int) error {
		recordPeak(&readActive, &readPeak)
		defer readActive.Add(-1)
		if arrived.Add(1) == int32(n) {
			close(allIn)
		}
		// Block until every reader is simultaneously in-flight. The timeout is
		// a safety valve: it fires only if the reads were not run concurrently,
		// so the assertion below fails cleanly instead of hanging.
		select {
		case <-allIn:
		case <-ctx.Done():
		case <-time.After(3 * time.Second):
		}
		return nil
	}

	var (
		writeActive, writePeak atomic.Int32
		mu                     sync.Mutex
		writeOrder             []int
	)
	write := func(_ context.Context, i int) error {
		recordPeak(&writeActive, &writePeak)
		defer writeActive.Add(-1)
		mu.Lock()
		writeOrder = append(writeOrder, i)
		mu.Unlock()
		time.Sleep(2 * time.Millisecond) // widen the window for overlap detection
		return nil
	}

	err := runCopyFanIn(context.Background(), n, read, write)
	require.NoError(t, err)
	require.Equal(t, int32(n), readPeak.Load(),
		"all source reads must run concurrently")
	require.Equal(t, int32(1), writePeak.Load(),
		"writes must be serialized for a single-writer joindb")
	require.Equal(t, []int{0, 1, 2, 3}, writeOrder,
		"writes must run in task order")
}

// TestRunCopyFanIn_readErrorPropagates verifies that a source read failure
// surfaces from runCopyFanIn and cancels the shared context so the other
// readers stop.
func TestRunCopyFanIn_readErrorPropagates(t *testing.T) {
	const n = 4
	wantErr := errz.New("read boom")

	var cancelledReads atomic.Int32
	read := func(ctx context.Context, i int) error {
		if i == 2 {
			return wantErr
		}
		// The other readers block until the failing read cancels ctx.
		select {
		case <-ctx.Done():
			cancelledReads.Add(1)
			return ctx.Err()
		case <-time.After(3 * time.Second):
			return nil
		}
	}
	write := func(ctx context.Context, _ int) error {
		<-ctx.Done()
		return ctx.Err()
	}

	err := runCopyFanIn(context.Background(), n, read, write)
	require.Error(t, err)
	require.ErrorIs(t, err, wantErr)
	require.Positive(t, cancelledReads.Load(),
		"a read failure must cancel the other in-flight reads")
}

// TestRunCopyFanIn_writeErrorPropagates verifies that a write failure surfaces
// from runCopyFanIn and halts the sequential writer.
func TestRunCopyFanIn_writeErrorPropagates(t *testing.T) {
	const n = 4
	wantErr := errz.New("write boom")

	read := func(context.Context, int) error { return nil }

	var writeCount atomic.Int32
	write := func(_ context.Context, i int) error {
		writeCount.Add(1)
		if i == 1 {
			return wantErr
		}
		return nil
	}

	err := runCopyFanIn(context.Background(), n, read, write)
	require.Error(t, err)
	require.ErrorIs(t, err, wantErr)
	require.LessOrEqual(t, writeCount.Load(), int32(2),
		"the serial writer must stop after a write failure")
}

// TestRunCopyFanIn_cancellation verifies that cancelling the parent context
// aborts the fan-in and returns a context error.
func TestRunCopyFanIn_cancellation(t *testing.T) {
	const n = 4
	ctx, cancel := context.WithCancel(context.Background())

	read := func(ctx context.Context, _ int) error {
		<-ctx.Done()
		return ctx.Err()
	}
	write := func(ctx context.Context, _ int) error {
		<-ctx.Done()
		return ctx.Err()
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := runCopyFanIn(ctx, n, read, write)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}
