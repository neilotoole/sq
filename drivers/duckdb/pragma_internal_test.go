package duckdb

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// mockExecer is a driver.ExecerContext stub that records every query it
// receives and optionally returns failErr on the next ExecContext call.
// failNext is consumed (set to false) after the first failure so the
// next call succeeds — letting a single instance drive a "fail then
// succeed" sequence if needed.
type mockExecer struct {
	failErr  error
	calls    []string
	failNext bool
}

func (m *mockExecer) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	m.calls = append(m.calls, query)
	if m.failNext {
		m.failNext = false
		return nil, m.failErr
	}
	return driver.RowsAffected(0), nil
}

// TestInstallExtensions_RetryAfterFailure verifies that installExtensions
// memoizes only on success: a transient failure leaves installComplete=false
// so the next caller retries. This is the contract that motivates the
// mutex+bool pattern instead of sync.Once (which would permanently poison
// the process on a single transient failure).
func TestInstallExtensions_RetryAfterFailure(t *testing.T) {
	// Reset the package-global install state so we can drive a fresh
	// sequence. Restore the original state on exit so we don't disturb
	// other tests in this package.
	installMu.Lock()
	originalComplete := installComplete
	installComplete = false
	installMu.Unlock()
	t.Cleanup(func() {
		installMu.Lock()
		installComplete = originalComplete
		installMu.Unlock()
	})

	// Phase 1: first INSTALL fails. Loop must short-circuit, return the
	// wrapped error, and NOT set installComplete.
	failErr := errors.New("simulated disk-full")
	failMock := &mockExecer{failNext: true, failErr: failErr}
	err := installExtensions(failMock)
	require.Error(t, err)
	require.ErrorContains(t, err, "simulated disk-full")
	require.Equal(t, []string{"INSTALL " + bundledExtensions[0]}, failMock.calls,
		"failure on first INSTALL should short-circuit the loop")
	installMu.Lock()
	require.False(t, installComplete, "transient failure must not poison the process")
	installMu.Unlock()

	// Phase 2: retry with a non-failing mock — every bundled extension is
	// INSTALLed and installComplete flips to true.
	successMock := &mockExecer{}
	err = installExtensions(successMock)
	require.NoError(t, err)
	require.Len(t, successMock.calls, len(bundledExtensions),
		"retry must INSTALL every bundled extension")
	for i, ext := range bundledExtensions {
		require.Equal(t, "INSTALL "+ext, successMock.calls[i])
	}
	installMu.Lock()
	require.True(t, installComplete, "success must memoize")
	installMu.Unlock()

	// Phase 3: subsequent calls hit the memoization early-return and
	// execute no INSTALLs.
	memoMock := &mockExecer{}
	err = installExtensions(memoMock)
	require.NoError(t, err)
	require.Empty(t, memoMock.calls, "memoized success must skip the INSTALL loop")
}
