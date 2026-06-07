package parquet_test

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/duckdb"
	"github.com/neilotoole/sq/drivers/parquet"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
)

func nopLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestDriverMetadata(t *testing.T) {
	d, err := (&parquet.Provider{Log: nopLogger()}).DriverFor(drivertype.Parquet)
	require.NoError(t, err)
	md := d.DriverMetadata()
	require.Equal(t, drivertype.Parquet, md.Type)
	require.True(t, md.Monotable)
	require.Equal(t, "Apache Parquet", md.Description)
}

func TestDriverFor_WrongType(t *testing.T) {
	_, err := (&parquet.Provider{Log: nopLogger()}).DriverFor(drivertype.CSV)
	require.Error(t, err)
}

func TestValidateSource_RejectsWrongType(t *testing.T) {
	d, err := (&parquet.Provider{Log: nopLogger()}).DriverFor(drivertype.Parquet)
	require.NoError(t, err)
	_, err = d.ValidateSource(&source.Source{Type: drivertype.CSV})
	require.Error(t, err)
}

func TestValidateSource_AcceptsParquet(t *testing.T) {
	d, err := (&parquet.Provider{Log: nopLogger()}).DriverFor(drivertype.Parquet)
	require.NoError(t, err)
	src := &source.Source{Type: drivertype.Parquet, Handle: "@p", Location: "/tmp/x.parquet"}
	got, err := d.ValidateSource(src)
	require.NoError(t, err)
	require.Same(t, src, got)
}

// newParquetProviderForTest builds a parquet.Provider whose Registry knows
// DuckDB and whose Files comes from a fresh testh.Helper (so we get the
// proper cache/temp dir wiring without depending on testh having parquet
// registered yet — that happens in Task 8).
func newParquetProviderForTest(t *testing.T) *parquet.Provider {
	t.Helper()
	log := nopLogger()
	reg := driver.NewRegistry(log)
	reg.AddProvider(drivertype.DuckDB, &duckdb.Provider{Log: log})
	th := testh.New(t)
	return &parquet.Provider{Log: log, Registry: reg, Files: th.Files()}
}

func TestOpen_LocalActorFixture(t *testing.T) {
	ctx := context.Background()

	abs, err := filepath.Abs("testdata/actor.parquet")
	require.NoError(t, err)

	provider := newParquetProviderForTest(t)
	drvr, err := provider.DriverFor(drivertype.Parquet)
	require.NoError(t, err)

	src := &source.Source{
		Type:     drivertype.Parquet,
		Handle:   "@actor",
		Location: abs,
	}
	g, err := drvr.Open(ctx, src)
	require.NoError(t, err)
	t.Cleanup(func() { _ = g.Close() })

	db, err := g.DB(ctx)
	require.NoError(t, err)

	var n int
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM "data"`).Scan(&n)
	require.NoError(t, err)
	require.Equal(t, 3, n)
}

func TestOpen_MissingFileFails(t *testing.T) {
	ctx := context.Background()
	provider := newParquetProviderForTest(t)
	drvr, err := provider.DriverFor(drivertype.Parquet)
	require.NoError(t, err)

	src := &source.Source{
		Type:     drivertype.Parquet,
		Handle:   "@missing",
		Location: "/nonexistent/path/to.parquet",
	}
	_, err = drvr.Open(ctx, src)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parquet")
}
