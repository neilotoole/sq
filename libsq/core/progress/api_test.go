package progress_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/progress"
)

// TestNew_disabled verifies that a non-positive maxBars yields a nil Progress.
func TestNew_disabled(t *testing.T) {
	t.Parallel()

	require.Nil(t, progress.New(context.Background(), io.Discard, 0, time.Millisecond, nil))
	require.Nil(t, progress.New(context.Background(), io.Discard, -1, time.Millisecond, nil))
}

// TestNilProgress_safe verifies the documented contract that all exported
// methods of Progress are safe to call on a nil Progress, and that the bars it
// hands out are safe no-ops.
func TestNilProgress_safe(t *testing.T) {
	t.Parallel()

	var p *progress.Progress
	require.Nil(t, p)

	require.NotPanics(t, func() {
		buf := &bytes.Buffer{}
		require.Same(t, buf, p.HideOnWriter(buf))

		bars := []progress.Bar{
			p.NewByteCounter("msg", -1),
			p.NewByteCounter("msg", 100),
			p.NewFilesizeCounter("msg", nil, "nonexistent"),
			p.NewUnitCounter("msg", "rec"),
			p.NewUnitTotalCounter("msg", "rec", 10),
			p.NewWaiter("msg"),
			p.NewTimeoutWaiter("msg", time.Now().Add(time.Minute)),
		}
		for _, b := range bars {
			require.NotNil(t, b)
			b.Incr(1)
			b.Stop()
		}

		p.Stop()
	})
}

// TestConstructors_smoke exercises each bar constructor against a live
// Progress, covering both their determinate and indeterminate branches.
func TestConstructors_smoke(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := progress.New(ctx, io.Discard, progress.DefaultMaxBars, time.Millisecond, nil)
	t.Cleanup(p.Stop)

	tmpFile := filepath.Join(t.TempDir(), "data.bin")
	require.NoError(t, os.WriteFile(tmpFile, []byte("0123456789"), 0o600))
	f, err := os.Open(tmpFile)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	bars := []progress.Bar{
		p.NewByteCounter("bytes indeterminate", -1),
		p.NewByteCounter("bytes determinate", 1000),
		p.NewFilesizeCounter("filesize from file", f, ""),
		p.NewFilesizeCounter("filesize from path", nil, tmpFile),
		p.NewFilesizeCounter("filesize missing", nil, "does-not-exist"),
		p.NewUnitCounter("units", "rec"),
		p.NewUnitCounter("units no unit", ""),
		p.NewUnitTotalCounter("unit total", "sheet", 16),
		p.NewUnitTotalCounter("unit total zero", "sheet", 0), // delegates to NewUnitCounter
		p.NewWaiter("waiter"),
		p.NewTimeoutWaiter("timeout future", time.Now().Add(time.Hour)),
		p.NewTimeoutWaiter("timeout past", time.Now().Add(-time.Hour)),
	}

	for _, b := range bars {
		require.NotNil(t, b)
		b.Incr(3)
		b.Stop()
	}
}

// TestNewUnitTotalCounter_zeroTotal verifies that a non-positive total causes
// NewUnitTotalCounter to behave like an indeterminate unit counter (i.e. it
// still returns a usable bar).
func TestNewUnitTotalCounter_zeroTotal(t *testing.T) {
	t.Parallel()

	p := progress.New(context.Background(), io.Discard, progress.DefaultMaxBars, time.Millisecond, nil)
	t.Cleanup(p.Stop)

	b := p.NewUnitTotalCounter("msg", "rec", 0)
	require.NotNil(t, b)
	b.Incr(1)
	b.Stop()
}

// TestContext_FromContext covers the context accessors, including the nil and
// missing-value paths.
func TestContext_FromContext(t *testing.T) {
	t.Parallel()

	var nilCtx context.Context
	require.Nil(t, progress.FromContext(nilCtx))
	require.Nil(t, progress.FromContext(context.Background()))

	p := progress.New(context.Background(), io.Discard, progress.DefaultMaxBars, time.Millisecond, nil)
	t.Cleanup(p.Stop)

	ctx := progress.NewContext(nilCtx, p)
	require.Same(t, p, progress.FromContext(ctx))
}

// TestContext_Incr verifies that progress.Incr increments the Bar carried by
// the context, and is safe on a nil context or a context without a Bar.
func TestContext_Incr(t *testing.T) {
	t.Parallel()

	var nilCtx context.Context
	require.NotPanics(t, func() {
		progress.Incr(nilCtx, 1)               // nil context
		progress.Incr(context.Background(), 1) // no bar in context
		progress.Incr(progress.NewBarContext(nilCtx, nil), 1)
	})

	p := progress.New(context.Background(), io.Discard, progress.DefaultMaxBars, time.Hour, nil)
	t.Cleanup(p.Stop)
	// A long render delay keeps the background loop from creating a lifecycle,
	// so the bar's group accounting is only mutated by this test.
	p.SetNotBefore(time.Now().Add(time.Hour))

	bar := p.NewByteCounter("msg", -1)
	ctx := progress.NewBarContext(context.Background(), bar)
	progress.Incr(ctx, 7)

	delta, ok := progress.GroupIncrDelta(bar)
	require.True(t, ok)
	require.Equal(t, 7, delta)
}

// TestColors_EnableColor covers enabling and disabling color, plus the nil
// receiver guard.
func TestColors_EnableColor(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() {
		c := progress.DefaultColors()
		c.EnableColor(true)
		c.EnableColor(false)

		var nilColors *progress.Colors
		nilColors.EnableColor(true)
		nilColors.EnableColor(false)
	})
}

// TestHideOnWriter verifies that writing to the HideOnWriter-wrapped writer
// passes bytes through, kills the live lifecycle (hiding the progress), and
// pushes the next-show checkpoint out by the render delay.
func TestHideOnWriter(t *testing.T) {
	t.Parallel()

	// A long render delay ensures that, once hidden, the background loop won't
	// immediately recreate the lifecycle, keeping the assertions deterministic.
	p := progress.New(context.Background(), io.Discard, progress.DefaultMaxBars, time.Hour, nil)
	t.Cleanup(p.Stop)

	bar := p.NewWaiter("waiter")
	bar.Incr(1)
	require.True(t, p.EnsureLife(), "lifecycle should have been established")
	require.True(t, p.HasLife())

	before := time.Now()
	buf := &bytes.Buffer{}
	w := p.HideOnWriter(buf)

	n, err := w.Write([]byte("main output"))
	require.NoError(t, err)
	require.Equal(t, len("main output"), n)
	require.Equal(t, "main output", buf.String())

	require.False(t, p.HasLife(), "lifecycle should be killed after a write")
	require.True(t, p.NotBefore().After(before),
		"next-show checkpoint should be pushed past the write time")
}

// TestHideOnWriter_nilWriter verifies the guard for a nil underlying writer.
func TestHideOnWriter_nilWriter(t *testing.T) {
	t.Parallel()

	p := progress.New(context.Background(), io.Discard, progress.DefaultMaxBars, time.Millisecond, nil)
	t.Cleanup(p.Stop)
	require.Nil(t, p.HideOnWriter(nil))
}

// TestGroupIncrDelta verifies the group-bar increment accounting: each call
// returns only the delta accumulated since the previous call.
func TestGroupIncrDelta(t *testing.T) {
	t.Parallel()

	p := progress.New(context.Background(), io.Discard, progress.DefaultMaxBars, time.Hour, nil)
	t.Cleanup(p.Stop)
	// Freeze the background loop so it never consumes the bar's deltas.
	p.SetNotBefore(time.Now().Add(time.Hour))

	bar := p.NewUnitCounter("msg", "rec")

	delta, ok := progress.GroupIncrDelta(bar)
	require.True(t, ok)
	require.Equal(t, 0, delta, "no increments yet")

	bar.Incr(10)
	delta, _ = progress.GroupIncrDelta(bar)
	require.Equal(t, 10, delta)

	delta, _ = progress.GroupIncrDelta(bar)
	require.Equal(t, 0, delta, "no new increments since last consume")

	bar.Incr(5)
	delta, _ = progress.GroupIncrDelta(bar)
	require.Equal(t, 5, delta)
}

// TestGroupBar_partition drives the refresh machinery deterministically and
// verifies that bars beyond the group threshold are routed into the invisible
// (grouped) set. With maxBars=3 the group threshold is 2, so of 5 renderable
// bars, 2 are visible and 3 are grouped.
func TestGroupBar_partition(t *testing.T) {
	t.Parallel()

	const maxBars = 3
	p := progress.New(context.Background(), io.Discard, maxBars, time.Millisecond, nil)
	t.Cleanup(p.Stop)

	const numBars = 5
	bars := make([]progress.Bar, numBars)
	for i := range bars {
		bars[i] = p.NewWaiter("waiter")
		// Make every bar immediately renderable.
		require.True(t, progress.SetBarNotBefore(bars[i], time.Now().Add(-time.Hour)))
		bars[i].Incr(1)
	}
	require.Equal(t, numBars, p.BarCount())

	require.True(t, p.EnsureLife(), "lifecycle should have been established")

	// A single deterministic refresh partitions the bars. Repeated background
	// refreshes produce the same stable partition counts.
	p.ForceRefresh(time.Now())

	visible, invisible := p.ActiveBarCounts()
	require.Equal(t, maxBars-1, visible, "visible bars capped at group threshold")
	require.Equal(t, numBars-(maxBars-1), invisible, "overflow bars are grouped")

	// The group bar's aggregate increment calculation must run without error.
	require.GreaterOrEqual(t, p.CalculateGroupIncr(), 0)
}
