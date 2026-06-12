package driver_test

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// openRecord captures what fakeDriver.Open observed for one invocation.
type openRecord struct {
	loc      string
	readOnly bool
	explicit bool
}

// fakeDriver implements driver.Driver, recording each Open invocation
// and the read-only hint carried by its context.
type fakeDriver struct {
	mu    sync.Mutex
	opens []openRecord
	grips []*fakeGrip
}

func (d *fakeDriver) Open(ctx context.Context, src *source.Source) (driver.Grip, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.opens = append(d.opens, openRecord{
		loc:      src.Location,
		readOnly: driver.IsReadOnly(ctx),
		explicit: driver.IsReadOnlyExplicit(ctx),
	})
	g := &fakeGrip{src: src}
	d.grips = append(d.grips, g)
	return g, nil
}

func (d *fakeDriver) Ping(_ context.Context, _ *source.Source) error { return nil }

func (d *fakeDriver) DriverMetadata() driver.Metadata { return driver.Metadata{} }

func (d *fakeDriver) ValidateSource(src *source.Source) (*source.Source, error) { return src, nil }

// openCount returns the number of Open invocations so far.
func (d *fakeDriver) openCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.opens)
}

// fakeProvider maps every driver type to the single fake driver.
type fakeProvider struct {
	drvr *fakeDriver
}

func (p *fakeProvider) DriverFor(_ drivertype.Type) (driver.Driver, error) {
	return p.drvr, nil
}

// fakeGrip is a minimal driver.Grip.
type fakeGrip struct {
	src    *source.Source
	mu     sync.Mutex
	closed bool
}

// errFakeGrip is returned by the fakeGrip methods the tests never invoke.
var errFakeGrip = errors.New("fakeGrip: not implemented")

func (g *fakeGrip) DB(_ context.Context) (*sql.DB, error) { return nil, errFakeGrip }

func (g *fakeGrip) SQLDriver() driver.SQLDriver { return nil }

func (g *fakeGrip) Source() *source.Source { return g.src }

func (g *fakeGrip) SourceMetadata(_ context.Context, _ bool) (*metadata.Source, error) {
	return nil, errFakeGrip
}

func (g *fakeGrip) TableMetadata(_ context.Context, _ string) (*metadata.Table, error) {
	return nil, errFakeGrip
}

func (g *fakeGrip) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closed = true
	return nil
}

func (g *fakeGrip) isClosed() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.closed
}

func newFakeGrips() (*driver.Grips, *fakeDriver) {
	drvr := &fakeDriver{}
	return driver.NewGrips(&fakeProvider{drvr: drvr}, nil, nil), drvr
}

// TestGrips_Open_ReadOnlyModeKeysCache verifies that the Grips cache
// distinguishes read-write, implicit read-only, and explicit read-only
// opens of the same source (gh #779). Previously the cache was keyed on
// handle alone, so whichever mode opened first was silently returned to
// every subsequent caller, and correctness depended on call-site
// ordering discipline (e.g. cli/cmd_sql.go opening the RW destination
// grip before the RO query grips).
func TestGrips_Open_ReadOnlyModeKeysCache(t *testing.T) {
	newSrc := func() *source.Source {
		return &source.Source{
			Handle:   "@fake",
			Type:     drivertype.Pg,
			Location: "postgres://alice:pw@db/sakila",
		}
	}

	t.Run("ro then rw", func(t *testing.T) {
		gs, drvr := newFakeGrips()
		ctx := context.Background()

		gripRO, err := gs.Open(driver.WithReadOnly(ctx), newSrc())
		require.NoError(t, err)
		gripRW, err := gs.Open(ctx, newSrc())
		require.NoError(t, err)

		require.NotSame(t, gripRO, gripRW,
			"RW open after RO open must not return the RO grip")
		require.Equal(t, 2, drvr.openCount())
		require.True(t, drvr.opens[0].readOnly)
		require.False(t, drvr.opens[1].readOnly,
			"second open must reach the driver without the read-only hint")

		// Repeat opens in each mode hit the per-mode cache entries.
		again, err := gs.Open(driver.WithReadOnly(ctx), newSrc())
		require.NoError(t, err)
		require.Same(t, gripRO, again)
		again, err = gs.Open(ctx, newSrc())
		require.NoError(t, err)
		require.Same(t, gripRW, again)
		require.Equal(t, 2, drvr.openCount())

		// Close must release every coexisting grip.
		require.NoError(t, gs.Close())
		for i, g := range drvr.grips {
			require.True(t, g.isClosed(), "grip %d not closed", i)
		}
	})

	t.Run("rw then ro", func(t *testing.T) {
		gs, drvr := newFakeGrips()
		ctx := context.Background()

		gripRW, err := gs.Open(ctx, newSrc())
		require.NoError(t, err)
		gripRO, err := gs.Open(driver.WithReadOnly(ctx), newSrc())
		require.NoError(t, err)

		require.NotSame(t, gripRW, gripRO,
			"RO open after RW open must not return the RW grip")
		require.Equal(t, 2, drvr.openCount())
		require.False(t, drvr.opens[0].readOnly)
		require.True(t, drvr.opens[1].readOnly,
			"second open must reach the driver with the read-only hint")

		require.NoError(t, gs.Close())
		for i, g := range drvr.grips {
			require.True(t, g.isClosed(), "grip %d not closed", i)
		}
	})

	t.Run("explicit ro distinct from implicit ro", func(t *testing.T) {
		// DuckDB treats an explicit --readonly hint more forcefully than
		// the implicit hint (it overrides access_mode=AUTOMATIC on the
		// location, see gh #803), so the two hints can produce different
		// connections and must not share a cache entry.
		gs, drvr := newFakeGrips()
		ctx := context.Background()

		gripImplicit, err := gs.Open(driver.WithReadOnly(ctx), newSrc())
		require.NoError(t, err)
		gripExplicit, err := gs.Open(driver.WithReadOnlyExplicit(ctx), newSrc())
		require.NoError(t, err)

		require.NotSame(t, gripImplicit, gripExplicit)
		require.Equal(t, 2, drvr.openCount())
		require.False(t, drvr.opens[0].explicit)
		require.True(t, drvr.opens[1].explicit)

		require.NoError(t, gs.Close())
	})
}

// TestGrips_Open_CacheHitSkipsSecretResolution verifies that a Grips
// cache hit returns before secret resolution runs (gh #779). Previously
// doOpen resolved secrets before consulting the cache, so every Open of
// an already-open source (e.g. one call per table during inspect) paid
// for resolution again. The second Open below carries a fresh
// secret.Registry whose resolver must never be invoked: a cache hit
// needs only the handle and the read-only hint, not the resolved
// location.
func TestGrips_Open_CacheHitSkipsSecretResolution(t *testing.T) {
	gs, drvr := newFakeGrips()
	src := &source.Source{
		Handle:   "@fake",
		Type:     drivertype.Pg,
		Location: "postgres://alice:${keyring:pw}@db/sakila",
	}

	resolverA := &captureResolver{value: "hunter2"}
	regA := secret.NewRegistry()
	regA.Register("keyring", resolverA)
	ctxA := secret.NewContext(context.Background(), regA)

	grip1, err := gs.Open(ctxA, src)
	require.NoError(t, err)
	require.Equal(t, []string{"pw"}, resolverA.calls)
	require.Equal(t, 1, drvr.openCount())
	require.Equal(t, "postgres://alice:hunter2@db/sakila", drvr.opens[0].loc,
		"driver must receive the resolved location")

	resolverB := &captureResolver{value: "hunter2"}
	regB := secret.NewRegistry()
	regB.Register("keyring", resolverB)
	ctxB := secret.NewContext(context.Background(), regB)

	grip2, err := gs.Open(ctxB, src)
	require.NoError(t, err)
	require.Same(t, grip1, grip2)
	require.Empty(t, resolverB.calls,
		"cache hit must not invoke secret resolution")
	require.Equal(t, 1, drvr.openCount())

	require.NoError(t, gs.Close())
}
