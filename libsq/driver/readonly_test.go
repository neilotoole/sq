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

func TestIsReadOnlyExplicit_BareContext(t *testing.T) {
	require.False(t, driver.IsReadOnlyExplicit(context.Background()))
}

func TestWithReadOnly_NotExplicit(t *testing.T) {
	ctx := driver.WithReadOnly(context.Background())
	require.True(t, driver.IsReadOnly(ctx))
	require.False(t, driver.IsReadOnlyExplicit(ctx),
		"implicit hint must not register as explicit")
}

func TestWithReadOnlyExplicit_RoundTrip(t *testing.T) {
	ctx := driver.WithReadOnlyExplicit(context.Background())
	require.True(t, driver.IsReadOnly(ctx),
		"explicit read-only implies read-only")
	require.True(t, driver.IsReadOnlyExplicit(ctx))
}

func TestWithReadOnly_DoesNotDowngradeExplicit(t *testing.T) {
	ctx := driver.WithReadOnly(driver.WithReadOnlyExplicit(context.Background()))
	require.True(t, driver.IsReadOnly(ctx))
	require.True(t, driver.IsReadOnlyExplicit(ctx),
		"an implicit hint must not weaken an existing explicit marker")
}

func TestWithReadOnlyExplicit_SurvivesChildContext(t *testing.T) {
	ctx := driver.WithReadOnlyExplicit(context.Background())
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	require.True(t, driver.IsReadOnlyExplicit(child),
		"explicit read-only flag must propagate through derived contexts")
}
