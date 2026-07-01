package libsq

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// concProbeTask records the peak number of tasks executing concurrently,
// so tests can assert executeTasks' effective concurrency limit.
type concProbeTask struct {
	active *atomic.Int32
	peak   *atomic.Int32
}

func (ct *concProbeTask) executeTask(context.Context) error {
	n := ct.active.Add(1)
	defer ct.active.Add(-1)
	for {
		p := ct.peak.Load()
		if n <= p || ct.peak.CompareAndSwap(p, n) {
			break
		}
	}
	time.Sleep(50 * time.Millisecond)
	return nil
}

func newConcProbeTasks(n int) (tasks []tasker, peak *atomic.Int32) {
	active := &atomic.Int32{}
	peak = &atomic.Int32{}
	tasks = make([]tasker, n)
	for i := range tasks {
		tasks[i] = &concProbeTask{active: active, peak: peak}
	}
	return tasks, peak
}

// TestPipeline_executeTasks_singleWriterSerializes verifies gh975: when the
// join destination is single-writer (tasksSingleWriter, as SQLite is),
// executeTasks runs the copy tasks one at a time, so they never contend on
// the single write lock and fail with "database is locked".
func TestPipeline_executeTasks_singleWriterSerializes(t *testing.T) {
	tasks, peak := newConcProbeTasks(4)
	p := &pipeline{tasks: tasks, tasksSingleWriter: true}
	require.NoError(t, p.executeTasks(context.Background()))
	require.Equal(t, int32(1), peak.Load(),
		"tasksSingleWriter must serialize join copy tasks")
}

// TestPipeline_executeTasks_concurrentByDefault verifies that for a
// multi-writer joindb (tasksSingleWriter false), executeTasks runs tasks
// concurrently up to the errgroup limit.
func TestPipeline_executeTasks_concurrentByDefault(t *testing.T) {
	tasks, peak := newConcProbeTasks(4)
	p := &pipeline{tasks: tasks, tasksSingleWriter: false}
	require.NoError(t, p.executeTasks(context.Background()))
	require.Greater(t, peak.Load(), int32(1),
		"a multi-writer joindb must allow concurrent task execution")
}
