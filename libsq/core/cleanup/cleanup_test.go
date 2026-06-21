package cleanup_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
)

func TestCleanup(t *testing.T) {
	want := []int{4, 3, 2, 1, 0}
	var got []int

	clnup := cleanup.New()

	for i := range 5 {
		clnup.AddE(func() error {
			got = append(got, i)
			return nil
		})
	}

	require.Equal(t, 5, clnup.Len())

	err := clnup.Run()
	require.NoError(t, err)
	require.Equal(t, want, got)

	// Run clears the func set, so Len is now zero and a
	// second Run is a no-op that doesn't re-execute funcs.
	require.Equal(t, 0, clnup.Len())
	require.NoError(t, clnup.Run())
	require.Equal(t, want, got, "funcs must not run twice")
}

func TestCleanup_Error(t *testing.T) {
	clnup := cleanup.New()

	clnup.AddE(func() error {
		return nil
	})

	clnup.AddE(func() error {
		return errz.New("err1")
	})

	clnup.AddE(func() error {
		return errz.New("err2")
	})

	err := clnup.Run()
	require.Error(t, err)

	require.Equal(t, "err2; err1", err.Error())
	errs := errz.Errors(err)
	require.Equal(t, 2, len(errs))
	require.Equal(t, "err2", errs[0].Error())
	require.Equal(t, "err1", errs[1].Error())
}

// TestCleanup_Nop verifies the package-level Nop func is a no-op.
func TestCleanup_Nop(t *testing.T) {
	require.NotNil(t, cleanup.Nop)
	require.NoError(t, cleanup.Nop())
}

// TestCleanup_NilReceiver verifies that the methods which guard
// against a nil receiver behave gracefully.
func TestCleanup_NilReceiver(t *testing.T) {
	var clnup *cleanup.Cleanup
	require.Equal(t, 0, clnup.Len())
	require.NoError(t, clnup.Run())
}

// TestCleanup_Empty verifies that Run on a fresh (empty) Cleanup
// is a no-op returning nil.
func TestCleanup_Empty(t *testing.T) {
	clnup := cleanup.New()
	require.Equal(t, 0, clnup.Len())
	require.NoError(t, clnup.Run())
}

func TestCleanup_Add(t *testing.T) {
	var got []int
	clnup := cleanup.New()

	// nil fn is ignored without affecting Len.
	clnup.Add(nil)
	require.Equal(t, 0, clnup.Len())

	clnup.Add(func() { got = append(got, 0) })
	clnup.Add(func() { got = append(got, 1) })
	require.Equal(t, 2, clnup.Len())

	require.NoError(t, clnup.Run())
	require.Equal(t, []int{1, 0}, got, "funcs run in reverse order")
}

func TestCleanup_AddE_Nil(t *testing.T) {
	clnup := cleanup.New()
	// nil fn is ignored.
	require.Same(t, clnup, clnup.AddE(nil))
	require.Equal(t, 0, clnup.Len())
}

// fakeCloser is an io.Closer that records whether it was closed and
// returns a configurable error.
type fakeCloser struct {
	closed bool
	err    error
}

func (c *fakeCloser) Close() error {
	c.closed = true
	return c.err
}

func TestCleanup_AddC(t *testing.T) {
	clnup := cleanup.New()

	// nil closer is ignored.
	require.Same(t, clnup, clnup.AddC(nil))
	require.Equal(t, 0, clnup.Len())

	c1 := &fakeCloser{}
	c2 := &fakeCloser{}
	clnup.AddC(c1)
	clnup.AddC(c2)
	require.Equal(t, 2, clnup.Len())

	require.NoError(t, clnup.Run())
	require.True(t, c1.closed)
	require.True(t, c2.closed)
}

func TestCleanup_AddC_Error(t *testing.T) {
	clnup := cleanup.New()
	c := &fakeCloser{err: errz.New("close failed")}
	clnup.AddC(c)

	err := clnup.Run()
	require.Error(t, err)
	require.Equal(t, "close failed", err.Error())
	require.True(t, c.closed)
}

func TestCleanup_Append(t *testing.T) {
	var got []int

	cu := cleanup.New()
	cu.AddE(func() error { got = append(got, 0); return nil })
	cu.AddE(func() error { got = append(got, 1); return nil })

	c := cleanup.New()
	c.AddE(func() error { got = append(got, 2); return nil })
	c.AddE(func() error { got = append(got, 3); return nil })

	require.Same(t, cu, cu.Append(c))
	require.Equal(t, 4, cu.Len())

	require.NoError(t, cu.Run())
	// After Append, cu.fns is [0,1,2,3] and Run executes in reverse, so
	// c's appended funcs (2,3) run first, then cu's own funcs (0,1): 3,2,1,0.
	require.Equal(t, []int{3, 2, 1, 0}, got)
}

// TestCleanup_Append_Nil verifies that appending a nil Cleanup, or a
// Cleanup to itself, is a no-op that returns the receiver.
func TestCleanup_Append_Nil(t *testing.T) {
	cu := cleanup.New()
	cu.AddE(func() error { return nil })

	require.Same(t, cu, cu.Append(nil))
	require.Equal(t, 1, cu.Len())

	// Self-append is a no-op (and must not deadlock).
	require.Same(t, cu, cu.Append(cu))
	require.Equal(t, 1, cu.Len())
}

// TestCleanup_Append_Concurrent verifies that reciprocal concurrent
// Append calls don't deadlock (regression test for lock ordering). Each
// iteration uses a fresh pair so the slices don't compound across the
// loop; the point is to race a.Append(b) against b.Append(a), not to
// grow either Cleanup.
func TestCleanup_Append_Concurrent(t *testing.T) {
	for range 100 {
		a := cleanup.New()
		a.AddE(func() error { return nil })
		b := cleanup.New()
		b.AddE(func() error { return nil })

		var wg sync.WaitGroup
		wg.Go(func() { a.Append(b) })
		wg.Go(func() { b.Append(a) })
		wg.Wait()

		require.NoError(t, a.Run())
		require.NoError(t, b.Run())
	}
}

// TestCleanup_Concurrent exercises concurrent Add/AddE/Len calls under
// the race detector to verify Cleanup is safe for concurrent use.
func TestCleanup_Concurrent(t *testing.T) {
	clnup := cleanup.New()

	const n = 50
	var wg sync.WaitGroup
	var counter struct {
		mu sync.Mutex
		n  int
	}

	for range n {
		wg.Go(func() {
			clnup.AddE(func() error {
				counter.mu.Lock()
				counter.n++
				counter.mu.Unlock()
				return nil
			})
			_ = clnup.Len()
		})
	}
	wg.Wait()

	require.Equal(t, n, clnup.Len())
	require.NoError(t, clnup.Run())
	require.Equal(t, n, counter.n)
}
