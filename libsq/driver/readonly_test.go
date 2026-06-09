package driver_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/driver"
)

func TestIsReadOnly_BareContext(t *testing.T) {
	require.False(t, driver.IsReadOnly(context.Background()))
}

func TestWithReadOnly_RoundTrip(t *testing.T) {
	ctx := driver.WithReadOnly(context.Background())
	require.True(t, driver.IsReadOnly(ctx))
}

func TestWithReadOnly_SurvivesChildContext(t *testing.T) {
	ctx := driver.WithReadOnly(context.Background())
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	require.True(t, driver.IsReadOnly(child),
		"read-only flag must propagate through derived contexts")
}

func TestWithReadOnly_Idempotent(t *testing.T) {
	ctx := driver.WithReadOnly(driver.WithReadOnly(context.Background()))
	require.True(t, driver.IsReadOnly(ctx))
}
