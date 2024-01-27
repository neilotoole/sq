// Package cleanup provides functionality for executing
// cleanup functions.
package cleanup

import (
	"io"
	"sync"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// Nop is a no-op cleanup.Func.
var Nop = func() error { return nil }

// New returns a new Cleanup instance.
func New() *Cleanup {
	return &Cleanup{}
}

// Cleanup encapsulates a slice of cleanup funcs. The
// funcs are executed by Run in reverse order to which
// they are added.
// Cleanup is safe for concurrent use.
type Cleanup struct {
	fns []func() error
	mu  sync.Mutex
}

// Len returns the count of cleanup funcs.
func (cu *Cleanup) Len() int {
	if cu == nil {
		return 0
	}
	cu.mu.Lock()
	defer cu.mu.Unlock()

	return len(cu.fns)
}

// Append c's cleanup funcs to cu.
func (cu *Cleanup) Append(c *Cleanup) *Cleanup {
	if c == nil || c == cu {
		return cu
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	cu.mu.Lock()
	defer cu.mu.Unlock()

	cu.fns = append(cu.fns, c.fns...)
	return cu
}

// Add adds a cleanup func.
func (cu *Cleanup) Add(fn func()) *Cleanup {
	if fn == nil {
		// For convenience in avoiding boilerplate nil checks
		// in the caller, we just ignore the nil arg.
		return cu
	}
	cu.mu.Lock()
	defer cu.mu.Unlock()

	cu.fns = append(cu.fns, func() error {
		fn()
		return nil
	})
	return cu
}

// AddE adds an error-returning cleanup func.
func (cu *Cleanup) AddE(fn func() error) *Cleanup {
	if fn == nil {
		// For convenience in avoiding boilerplate nil checks
		// in the caller, we just ignore the nil arg.
		return cu
	}
	cu.mu.Lock()
	defer cu.mu.Unlock()

	cu.fns = append(cu.fns, fn)
	return cu
}

// AddC adds c.Close as a cleanup func. This method
// is no-op if c is nil.
func (cu *Cleanup) AddC(c io.Closer) *Cleanup {
	if c == nil {
		// For convenience in avoiding boilerplate nil checks
		// in the caller, we just ignore the nil arg.
		return cu
	}
	cu.mu.Lock()
	defer cu.mu.Unlock()

	cu.fns = append(cu.fns, c.Close)
	return cu
}

// Run executes the cleanup funcs in reverse order to which
// they were added. All funcs are executed, even in the presence of
// an error from a func. Any errors are combined into a single error.
// The set of cleanup funcs is removed when Run returns.
//
// TODO: Consider renaming Run to Close so that Cleanup
// implements io.Closer?
func (cu *Cleanup) Run() error {
	if cu == nil {
		return nil
	}

	cu.mu.Lock()
	defer cu.mu.Unlock()

	if len(cu.fns) == 0 {
		return nil
	}

	// Capture any cleanup func errors
	var err error

	// Run cleanups in reverse order
	for i := len(cu.fns) - 1; i >= 0; i-- {
		fn := cu.fns[i]
		if fn == nil {
			// skip any nil fns
			continue
		}
		err = errz.Append(err, fn())
	}

	// Set fns to nil so that the cleanup funcs
	// can't get run twice.
	cu.fns = nil
	return err
}
