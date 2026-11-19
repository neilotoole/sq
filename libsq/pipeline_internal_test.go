package libsq

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// taskerFunc adapts a plain func to the tasker interface.
type taskerFunc func(context.Context) error

func (f taskerFunc) executeTask(ctx context.Context) error { return f(ctx) }

// recordPeak increments active, advances peak to the running maximum, and
// returns the new active count. The caller must decrement active when its
// task finishes.
func recordPeak(active, peak *atomic.Int32) int32 {
	n := active.Add(1)
	for {
		p := peak.Load()
		if n <= p || peak.CompareAndSwap(p, n) {
			return n
		}
	}
}

// TestPipeline_executeTasks_concurrentByDefault verifies that for a
// multi-writer joindb (tasksSingleWriter false), executeTasks runs tasks
// concurrently up to the errgroup limit. Each task blocks until a second
// task is simultaneously in-flight, so the overlap is observed
// deterministically rather than depending on sleep timing (which could flake
// on a loaded runner).
func TestPipeline_executeTasks_concurrentByDefault(t *testing.T) {
	const n = 4
	var (
		active, peak atomic.Int32
		once         sync.Once
		twoInFlight  = make(chan struct{})
	)

	tasks := make([]tasker, n)
	for i := range tasks {
		tasks[i] = taskerFunc(func(context.Context) error {
			cur := recordPeak(&active, &peak)
			defer active.Add(-1)
			if cur >= 2 {
				once.Do(func() { close(twoInFlight) })
			}
			// Block until two tasks are concurrently in-flight. The timeout is
			// a safety valve: it fires only if executeTasks failed to run tasks
			// concurrently, so the assertion below fails cleanly instead of the
			// test hanging.
			select {
			case <-twoInFlight:
			case <-time.After(3 * time.Second):
			}
			return nil
		})
	}

	p := &pipeline{tasks: tasks, tasksSingleWriter: false}
	require.NoError(t, p.executeTasks(context.Background()))
	require.Greater(t, peak.Load(), int32(1),
		"a multi-writer joindb must allow concurrent task execution")
}

// TestPipeline_executeTasks_singleWriterSerialFallback verifies the defensive
// serial fallback: when a single-writer joindb (tasksSingleWriter) has tasks
// that are not joinCopyTasks (so copyTasksOf reports false), executeTasks runs
// them one at a time via executeTasksSerial rather than concurrently, keeping
// the single-writer invariant. Real join copies take the fan-in path instead
// (concurrent reads, one serialized writer), covered by TestRunCopyFanIn_*.
//
// The sleep widens the window in which overlapping tasks would be observable,
// so a broken (concurrent) fallback is reliably caught; under a correct serial
// run the peak is structurally 1 regardless of the sleep, so this can't flake.
func TestPipeline_executeTasks_singleWriterSerialFallback(t *testing.T) {
	const n = 4
	var active, peak atomic.Int32

	tasks := make([]tasker, n)
	for i := range tasks {
		tasks[i] = taskerFunc(func(context.Context) error {
			recordPeak(&active, &peak)
			defer active.Add(-1)
			time.Sleep(20 * time.Millisecond)
			return nil
		})
	}

	p := &pipeline{tasks: tasks, tasksSingleWriter: true}
	require.NoError(t, p.executeTasks(context.Background()))
	require.Equal(t, int32(1), peak.Load(),
		"single-writer non-copy tasks must run serially via executeTasksSerial")
}
