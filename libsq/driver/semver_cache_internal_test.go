package driver

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSemverCache(t *testing.T) {
	// Caches on success: fetch runs once.
	var c SemverCache
	calls := 0
	fetch := func() (string, error) { calls++; return "v8.0.36", nil }
	v, err := c.Get(fetch)
	require.NoError(t, err)
	require.Equal(t, "v8.0.36", v)
	v, err = c.Get(fetch)
	require.NoError(t, err)
	require.Equal(t, "v8.0.36", v)
	require.Equal(t, 1, calls, "should fetch once and cache")

	// Does NOT cache on failure: first fetch errors, second succeeds.
	var c2 SemverCache
	errCalls := 0
	errFetch := func() (string, error) {
		errCalls++
		if errCalls == 1 {
			return "", errors.New("boom")
		}
		return "v9.0.0", nil
	}
	_, err = c2.Get(errFetch)
	require.Error(t, err)
	v, err = c2.Get(errFetch)
	require.NoError(t, err)
	require.Equal(t, "v9.0.0", v)
	require.Equal(t, 2, errCalls, "failed fetch must not poison the cache")
}

func TestSemverCache_Prime(t *testing.T) {
	// Prime seeds the cache: Get returns the primed value without fetching.
	var c SemverCache
	c.Prime("v8.0.36")
	v, err := c.Get(func() (string, error) { return "", errors.New("fetch must not run") })
	require.NoError(t, err)
	require.Equal(t, "v8.0.36", v)

	// Prime with an empty value (version undeterminable) is a no-op: the
	// cache stays unprimed and a later Get fetches as normal.
	var c2 SemverCache
	c2.Prime("")
	calls := 0
	v, err = c2.Get(func() (string, error) { calls++; return "v9.0.0", nil })
	require.NoError(t, err)
	require.Equal(t, "v9.0.0", v)
	require.Equal(t, 1, calls, "empty Prime must leave the cache unprimed")
}
