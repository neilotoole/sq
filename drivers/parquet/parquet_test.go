package parquet_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
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
// proper cache/temp dir wiring). Unlike testh.Helper.Open, it exercises the
// provider/driver construction path directly.
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

// TestOpen_TruncatedFileFails pins the eager-DESCRIBE contract of
// createParquetView: a file whose footer is missing (head magic intact, tail
// cut off) must fail at Open time, not at first query.
func TestOpen_TruncatedFileFails(t *testing.T) {
	ctx := context.Background()

	abs, err := filepath.Abs("testdata/truncated.parquet")
	require.NoError(t, err)

	provider := newParquetProviderForTest(t)
	drvr, err := provider.DriverFor(drivertype.Parquet)
	require.NoError(t, err)

	src := &source.Source{
		Type:     drivertype.Parquet,
		Handle:   "@truncated",
		Location: abs,
	}
	_, err = drvr.Open(ctx, src)
	require.Error(t, err)
	require.Contains(t, err.Error(), "truncated.parquet")
}

// TestQueryFixtures runs SLQ queries through the full libsq pipeline (dialect
// rendering, record reader, sink) against the type-edge-case fixtures:
// NULLs, decimals, timestamp granularities, nested types, and an empty
// (zero-row) file.
func TestQueryFixtures(t *testing.T) {
	testCases := []struct {
		name      string
		file      string
		wantCols  []string
		wantCount int
		check     func(t *testing.T, sink *testh.RecordSink)
	}{
		{
			name:      "nulls",
			file:      "nulls.parquet",
			wantCols:  []string{"id", "name", "score"},
			wantCount: 3,
			check: func(t *testing.T, sink *testh.RecordSink) {
				t.Helper()
				// Row 1 is all-NULL in the fixture.
				for i, v := range sink.Recs[1] {
					require.Nil(t, v, "rec 1 col %d should be nil", i)
				}
				require.NotNil(t, sink.Recs[0][0])
				require.Nil(t, sink.Recs[2][2], "rec 2 score is NULL")
			},
		},
		{
			name:      "decimals",
			file:      "decimals.parquet",
			wantCols:  []string{"small", "medium", "big"},
			wantCount: 1,
			check: func(t *testing.T, sink *testh.RecordSink) {
				t.Helper()
				require.Equal(t, "1.23", fmt.Sprint(sink.Recs[0][0]),
					"DECIMAL(5,2) must round-trip exactly")
				require.Equal(t, "1234567.890123", fmt.Sprint(sink.Recs[0][1]),
					"DECIMAL(18,6) must round-trip exactly")
				require.NotNil(t, sink.Recs[0][2], "DECIMAL(38,18)")
			},
		},
		{
			name:      "timestamps",
			file:      "timestamps.parquet",
			wantCols:  []string{"ts_us", "ts_ms", "ts_ns", "ts_tz"},
			wantCount: 1,
			check: func(t *testing.T, sink *testh.RecordSink) {
				t.Helper()
				for i := range sink.Recs[0] {
					require.NotNil(t, sink.Recs[0][i], "col %d", i)
				}
			},
		},
		{
			name:      "nested",
			file:      "nested.parquet",
			wantCols:  []string{"id", "person", "scores", "counts"},
			wantCount: 1,
			check: func(t *testing.T, sink *testh.RecordSink) {
				t.Helper()
				// STRUCT, LIST and MAP columns must survive the record
				// pipeline without crashing; exact representation is up to
				// the DuckDB driver.
				for i := range sink.Recs[0] {
					require.NotNil(t, sink.Recs[0][i], "col %d", i)
				}
			},
		},
		{
			name:      "empty",
			file:      "empty.parquet",
			wantCols:  []string{"id", "name"},
			wantCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			th := testh.New(t)
			src := th.Add(&source.Source{
				Handle:   "@pq_" + tc.name,
				Type:     drivertype.Parquet,
				Location: filepath.Join("testdata", tc.file),
			})

			sink, err := th.QuerySLQ(src.Handle+".data", nil)
			require.NoError(t, err)
			require.Equal(t, tc.wantCols, sink.RecMeta.MungedNames())
			require.Equal(t, tc.wantCount, len(sink.Recs))
			if tc.check != nil {
				tc.check(t, sink)
			}
		})
	}
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

// TestPing_NonHTTPRemoteVerifiesReachability verifies that Ping performs a
// real reachability check for s3-style remotes (by opening the grip, whose
// eager DESCRIBE reads the parquet footer via DuckDB httpfs) rather than
// returning unconditional success: pinging a nonexistent bucket must fail.
func TestPing_NonHTTPRemoteVerifiesReachability(t *testing.T) {
	tu.SkipShort(t, true)

	provider := newParquetProviderForTest(t)
	drvr, err := provider.DriverFor(drivertype.Parquet)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	src := &source.Source{
		Type:     drivertype.Parquet,
		Handle:   "@remote_missing",
		Location: "s3://sq-test-nonexistent-bucket-d41d8cd9/k.parquet",
	}
	require.Error(t, drvr.Ping(ctx, src))
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

	// The "data" view's columns must match the parquet schema.
	require.Len(t, md.Tables, 1)
	require.Equal(t, source.MonotableName, md.Tables[0].Name)
	gotCols := make([]string, len(md.Tables[0].Columns))
	for i, col := range md.Tables[0].Columns {
		gotCols[i] = col.Name
	}
	require.Equal(t, sakila.TblActorCols(), gotCols)

	tblMd, err := g.TableMetadata(th.Context, source.MonotableName)
	require.NoError(t, err)
	require.Equal(t, source.MonotableName, tblMd.Name)
	require.Equal(t, int64(sakila.TblActorCount), tblMd.RowCount)
}

// TestQuerySLQ_SakilaParquet runs SLQ queries through the libsq pipeline
// against the registered sakila parquet handles, covering the
// dialect-selection and record-writer path that sq itself uses.
func TestQuerySLQ_SakilaParquet(t *testing.T) {
	testCases := []struct {
		handle    string
		wantCols  []string
		wantCount int
	}{
		{
			handle:    sakila.ParquetActor,
			wantCols:  sakila.TblActorCols(),
			wantCount: sakila.TblActorCount,
		},
		{
			handle:    sakila.ParquetAddress,
			wantCols:  sakila.TblAddressCols(),
			wantCount: sakila.TblAddressCount,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.handle, func(t *testing.T) {
			th := testh.New(t)
			sink, err := th.QuerySLQ(tc.handle+".data", nil)
			require.NoError(t, err)
			require.Equal(t, tc.wantCols, sink.RecMeta.MungedNames())
			require.Equal(t, tc.wantCount, len(sink.Recs))
		})
	}
}

// TestConcurrentQueries verifies that multiple connections from the grip's
// pool see the "data" view. The view is created on one pooled connection of
// an in-memory DuckDB; a regression to per-connection :memory: semantics
// would pass single-query tests and fail on the second connection.
func TestConcurrentQueries(t *testing.T) {
	th := testh.New(t)
	g := th.Open(th.Source(sakila.ParquetActor))

	db, err := g.DB(th.Context)
	require.NoError(t, err)

	const goroutines = 4
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	counts := make([]int, goroutines)
	for i := range goroutines {
		wg.Go(func() {
			errs[i] = db.QueryRowContext(th.Context,
				`SELECT count(*) FROM "data"`).Scan(&counts[i])
		})
	}
	wg.Wait()

	for i := range goroutines {
		require.NoError(t, errs[i])
		require.Equal(t, sakila.TblActorCount, counts[i])
	}
}

// TestTableMetadata_RejectsNonMonotableName verifies that requesting
// metadata for any table other than the "data" monotable fails with a clear
// sq-level error, rather than falling through to a DuckDB "table not found"
// error that references internals the user never created.
func TestTableMetadata_RejectsNonMonotableName(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.ParquetActor)
	g := th.Open(src)

	_, err := g.TableMetadata(th.Context, "not_data")
	require.Error(t, err)
	require.Contains(t, err.Error(), `table name should be "data"`)

	md, err := g.TableMetadata(th.Context, source.MonotableName)
	require.NoError(t, err)
	require.Equal(t, source.MonotableName, md.Name)
}

// TestSakilaParquetMatchesCSV asserts that the parquet sakila actor fixture
// (generated from the CSV fixture by generate-sakila.sh) returns identical
// records to the CSV source through the libsq record pipeline: every row,
// every column, including the last_update timestamp.
func TestSakilaParquetMatchesCSV(t *testing.T) {
	th := testh.New(t)

	pqSink, err := th.QuerySLQ(sakila.ParquetActor+".data", nil)
	require.NoError(t, err)
	csvSink, err := th.QuerySLQ(sakila.CSVActor+".data", nil)
	require.NoError(t, err)

	require.Equal(t, csvSink.RecMeta.MungedNames(), pqSink.RecMeta.MungedNames(),
		"column names differ between csv and parquet sakila actor")
	require.Equal(t, len(csvSink.Recs), len(pqSink.Recs),
		"row count differs between csv and parquet sakila actor")
	for i := range csvSink.Recs {
		require.EqualValues(t, csvSink.Recs[i], pqSink.Recs[i],
			"row %d differs between csv and parquet sakila actor", i)
	}
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
	require.Contains(t, err.Error(), "missing.parquet",
		"error should reference the requested path")
	require.Contains(t, err.Error(), "404",
		"error should surface the HTTP status")
}
