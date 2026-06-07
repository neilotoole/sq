package parquet_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/duckdb"
	"github.com/neilotoole/sq/drivers/parquet"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
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
	require.Contains(t, err.Error(), "/nonexistent/path/to.parquet")
}

// TestPing_NonHTTPRemoteReturnsNil verifies that Ping does not try to stat
// an s3:// (or similar) URL. files.Ping would treat it as a local file path
// and fail with an os.Stat error; the parquet driver instead defers to the
// first Open/query, which goes through DuckDB's httpfs.
func TestPing_NonHTTPRemoteReturnsNil(t *testing.T) {
	ctx := context.Background()
	provider := newParquetProviderForTest(t)
	drvr, err := provider.DriverFor(drivertype.Parquet)
	require.NoError(t, err)

	for _, loc := range []string{
		"s3://bucket/k.parquet",
		"gs://bucket/k.parquet",
		"r2://bucket/k.parquet",
	} {
		t.Run(loc, func(t *testing.T) {
			src := &source.Source{
				Type:     drivertype.Parquet,
				Handle:   "@remote",
				Location: loc,
			}
			require.NoError(t, drvr.Ping(ctx, src))
		})
	}
}

func TestOpen_WithConnOptions(t *testing.T) {
	ctx := context.Background()

	abs, err := filepath.Abs("testdata/actor.parquet")
	require.NoError(t, err)

	provider := newParquetProviderForTest(t)
	drvr, err := provider.DriverFor(drivertype.Parquet)
	require.NoError(t, err)

	// Setting threads=1 via the location query is the documented way to
	// forward DuckDB connection options through a parquet source. We verify
	// it took effect by checking DuckDB's current_setting after the view is
	// created.
	src := &source.Source{
		Type:     drivertype.Parquet,
		Handle:   "@actor_opts",
		Location: abs + "?threads=1",
	}
	g, err := drvr.Open(ctx, src)
	require.NoError(t, err)
	t.Cleanup(func() { _ = g.Close() })

	db, err := g.DB(ctx)
	require.NoError(t, err)

	var threads string
	err = db.QueryRowContext(ctx, `SELECT current_setting('threads')`).Scan(&threads)
	require.NoError(t, err)
	require.Equal(t, "1", threads)
}

func TestSakilaParquetActor(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.ParquetActor)
	require.Equal(t, drivertype.Parquet, src.Type)

	g := th.Open(src)

	db, err := g.DB(th.Context)
	require.NoError(t, err)

	var n int
	err = db.QueryRowContext(th.Context, `SELECT count(*) FROM "data"`).Scan(&n)
	require.NoError(t, err)
	require.Equal(t, 200, n) // Sakila actor table has 200 rows.
}

func TestInspectSakilaParquet(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.ParquetActor)
	g := th.Open(src)

	md, err := g.SourceMetadata(th.Context, false)
	require.NoError(t, err)
	require.Equal(t, drivertype.Parquet, md.Driver)
	require.Equal(t, src.Handle, md.Handle)
	require.NotZero(t, md.Size)
	require.Equal(t, "actor.parquet", md.Name)
}

func TestSakilaParquetMatchesCSV(t *testing.T) {
	th := testh.New(t)

	pq := th.Open(th.Source(sakila.ParquetActor))
	csv := th.Open(th.Source(sakila.CSVActor))

	// Row count must match.
	require.Equal(t, sakilaActorRowCount(t, csv), sakilaActorRowCount(t, pq),
		"row count differs between csv and parquet sakila actor")

	// Same ordered rows when both columns are cast to VARCHAR.
	// Casting normalizes type-shape differences (CSV INT vs Parquet INT32,
	// timestamp formatting, decimal padding, etc.).
	const q = `SELECT CAST(actor_id AS VARCHAR), CAST(first_name AS VARCHAR),
	                  CAST(last_name AS VARCHAR)
	           FROM "data"
	           ORDER BY CAST(actor_id AS INTEGER)
	           LIMIT 5`

	pqRows := readThreeColRows(t, pq, q)
	csvRows := readThreeColRows(t, csv, q)
	require.Equal(t, csvRows, pqRows,
		"first 5 rows differ between csv and parquet sakila actor")
}

func sakilaActorRowCount(t *testing.T, g driver.Grip) int {
	t.Helper()
	db, err := g.DB(context.Background())
	require.NoError(t, err)
	var n int
	require.NoError(t,
		db.QueryRowContext(context.Background(), `SELECT count(*) FROM "data"`).Scan(&n))
	return n
}

func readThreeColRows(t *testing.T, g driver.Grip, query string) [][3]string {
	t.Helper()
	db, err := g.DB(context.Background())
	require.NoError(t, err)
	rows, err := db.QueryContext(context.Background(), query)
	require.NoError(t, err)
	defer rows.Close()

	var out [][3]string
	for rows.Next() {
		var a, b, c string
		require.NoError(t, rows.Scan(&a, &b, &c))
		out = append(out, [3]string{a, b, c})
	}
	require.NoError(t, rows.Err())
	return out
}

func TestOpen_HTTPS(t *testing.T) {
	tu.SkipShort(t, true)

	body, err := os.ReadFile("testdata/actor.parquet")
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apache.parquet")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		// httpfs uses Range requests; http.ServeContent handles them correctly.
		http.ServeContent(w, r, "actor.parquet", time.Time{}, bytes.NewReader(body))
	}))
	t.Cleanup(server.Close)

	th := testh.New(t)
	src := &source.Source{
		Type:     drivertype.Parquet,
		Handle:   "@remote_actor",
		Location: server.URL + "/actor.parquet",
	}
	drvr := th.DriverFor(src)
	g, err := drvr.Open(th.Context, src)
	require.NoError(t, err)
	t.Cleanup(func() { _ = g.Close() })

	db, err := g.DB(th.Context)
	require.NoError(t, err)
	var n int
	err = db.QueryRowContext(th.Context, `SELECT count(*) FROM "data"`).Scan(&n)
	require.NoError(t, err)
	require.Equal(t, 3, n)
}

func TestOpen_HTTPS_404(t *testing.T) {
	tu.SkipShort(t, true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)

	th := testh.New(t)
	src := &source.Source{
		Type:     drivertype.Parquet,
		Handle:   "@remote_missing",
		Location: server.URL + "/missing.parquet",
	}
	drvr := th.DriverFor(src)
	_, err := drvr.Open(th.Context, src)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parquet")
}
