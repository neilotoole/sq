package duckdb_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
)

// TestRecordMeta_TypeSpectrum exercises every branch in
// newRecordFuncForDuckDB by scanning a row of testdata/type_test.ddl through
// the full RecordMeta + munge pipeline and asserting the canonical Go type
// produced for each column.
//
// TestKindMapping (db_type_test.go) covers the static type-name → kind.Kind
// mapping; this test covers the runtime value-munging path.
func TestRecordMeta_TypeSpectrum(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "type_test.duckdb")

	ddl, err := os.ReadFile("testdata/type_test.ddl")
	require.NoError(t, err)

	rawDB, err := sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(context.Background(), string(ddl))
	require.NoError(t, err)
	require.NoError(t, rawDB.Close())

	th := testh.New(t)
	src := &source.Source{
		Handle:   "@type_spectrum",
		Type:     drivertype.DuckDB,
		Location: "duckdb://" + dbPath,
	}
	th.Add(src)

	grip := th.Open(src)
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	rows, err := db.QueryContext(th.Context, `SELECT * FROM type_test`)
	require.NoError(t, err)
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	require.NoError(t, err)

	recMeta, newRecFn, err := grip.SQLDriver().RecordMeta(th.Context, colTypes)
	require.NoError(t, err)

	require.True(t, rows.Next(), "type_test should have at least one row")
	scanRow := recMeta.NewScanRow()
	require.NoError(t, rows.Scan(scanRow...))
	rec, err := newRecFn(scanRow)
	require.NoError(t, err)

	// Map column name → Go type the munge should produce.
	wantTypes := map[string]string{
		"col_bool":        "bool",
		"col_tinyint":     "int64",
		"col_smallint":    "int64",
		"col_int":         "int64",
		"col_bigint":      "int64",
		"col_hugeint":     "decimal.Decimal",
		"col_uhugeint":    "decimal.Decimal",
		"col_utinyint":    "int64",
		"col_usmallint":   "int64",
		"col_uint":        "int64",
		"col_ubigint":     "decimal.Decimal",
		"col_float":       "float64",
		"col_double":      "float64",
		"col_decimal":     "decimal.Decimal",
		"col_varchar":     "string",
		"col_blob":        "[]byte",
		"col_date":        "time.Time",
		"col_time":        "time.Time",
		"col_timestamp":   "time.Time",
		"col_timestamptz": "time.Time",
		"col_interval":    "string",
		"col_uuid":        "string",
		"col_json":        "string",
		"col_list":        "string",
		"col_struct":      "string",
		"col_map":         "string",
		"col_enum":        "string",
	}

	gotColumns := make(map[string]bool, len(recMeta))
	for i, fm := range recMeta {
		name := fm.Name()
		gotColumns[name] = true
		wantType, ok := wantTypes[name]
		if !ok {
			t.Errorf("unexpected column in type_test: %s", name)
			continue
		}
		gotType := goType(rec[i])
		require.Equal(t, wantType, gotType,
			"col %s: want Go type %s, got %s (value=%v)", name, wantType, gotType, rec[i])
	}

	// Catch the reverse direction too: if type_test.ddl drops a column that
	// wantTypes still names, the loop above would silently miss it.
	for name := range wantTypes {
		require.True(t, gotColumns[name],
			"col %s in wantTypes but missing from type_test result", name)
	}

	// col_interval renders in DuckDB's native canonical text form
	// (see FormatInterval); the fixture value is
	// "1 year 2 months 3 days 4 hours 5 minutes 6.789 seconds".
	for i, fm := range recMeta {
		if fm.Name() == "col_interval" {
			require.Equal(t, "1 year 2 months 3 days 04:05:06.789", rec[i])
		}
	}
}

func goType(v any) string {
	switch v.(type) {
	case bool:
		return "bool"
	case int64:
		return "int64"
	case float64:
		return "float64"
	case decimal.Decimal:
		return "decimal.Decimal"
	case string:
		return "string"
	case []byte:
		return "[]byte"
	case time.Time:
		return "time.Time"
	case nil:
		return "nil"
	default:
		return "unknown"
	}
}

// TestRecordMeta_HugeIntPrecision asserts that HUGEINT, UHUGEINT, and
// UBIGINT values that exceed int64 range round-trip through the type munge
// without precision loss — they are promoted to decimal.Decimal rather
// than truncated to int64.
func TestRecordMeta_HugeIntPrecision(t *testing.T) {
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@hugeint_test",
		Type:     drivertype.DuckDB,
		Location: "duckdb://:memory:",
	}
	th.Add(src)
	grip := th.Open(src)
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	const (
		hugeIntMax  = "170141183460469231731687303715884105727" // 2^127 - 1
		uHugeIntMax = "340282366920938463463374607431768211455" // 2^128 - 1
		uBigIntMax  = "18446744073709551615"                    // 2^64 - 1
	)
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"hugeint_max", "SELECT " + hugeIntMax + "::HUGEINT", hugeIntMax},
		// HUGEINT min + 1. The literal -2^127 can't be expressed as
		// "-2^127::HUGEINT" — DuckDB parses that as unary negation of a
		// positive 2^127 literal that doesn't fit in INT128. min + 1 is
		// sufficient to exercise the large-negative path.
		{"hugeint_min_plus_one", "SELECT -" + hugeIntMax + "::HUGEINT", "-" + hugeIntMax},
		{"uhugeint_max", "SELECT " + uHugeIntMax + "::UHUGEINT", uHugeIntMax},
		{"ubigint_max", "SELECT " + uBigIntMax + "::UBIGINT", uBigIntMax},
		// HUGEINT value within int64 range — still promoted to decimal for
		// column-type consistency.
		{"hugeint_small", "SELECT 42::HUGEINT", "42"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := db.QueryContext(th.Context, tc.query)
			require.NoError(t, err)
			defer rows.Close()

			colTypes, err := rows.ColumnTypes()
			require.NoError(t, err)
			recMeta, newRecFn, err := grip.SQLDriver().RecordMeta(th.Context, colTypes)
			require.NoError(t, err)

			require.True(t, rows.Next())
			scanRow := recMeta.NewScanRow()
			require.NoError(t, rows.Scan(scanRow...))
			rec, err := newRecFn(scanRow)
			require.NoError(t, err)

			d, ok := rec[0].(decimal.Decimal)
			require.True(t, ok, "expected decimal.Decimal, got %T (value=%v)", rec[0], rec[0])
			require.Equal(t, tc.want, d.String())
		})
	}
}

// TestOpen_Memory verifies that an in-memory DuckDB source ("duckdb://:memory:")
// can be opened through driveri.Open and used for DDL + DML + DQL end-to-end.
func TestOpen_Memory(t *testing.T) {
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@mem_e2e",
		Type:     drivertype.DuckDB,
		Location: "duckdb://:memory:",
	}
	th.Add(src)

	grip := th.Open(src)
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context, `CREATE TABLE t (id INTEGER, name VARCHAR)`)
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context, `INSERT INTO t VALUES (1, 'a'), (2, 'b'), (3, 'c')`)
	require.NoError(t, err)

	var cnt int
	require.NoError(t, db.QueryRowContext(th.Context, `SELECT count(*) FROM t`).Scan(&cnt))
	require.Equal(t, 3, cnt)
}

// TestConcurrentOpen exercises the connector init fn (extension INSTALL +
// LOAD + SET) under concurrent open. Regression coverage for the "INSTALL
// once per process / LOAD per connection" contract. Without process-level
// memoization of INSTALL, parallel opens against fresh DBs race on the
// on-disk extension cache (manifests as "Could not move file: Access is
// denied" on Windows). The installExtensions mutex+bool pattern in
// pragma.go is deliberately NOT sync.Once, so that a transient install
// failure (disk full, antivirus) does not permanently poison the process.
//
// Note: by the time this test runs, earlier tests in the package have
// already flipped installComplete=true, so the 8 goroutines below mainly
// exercise concurrent LOAD + SET via connInitFn rather than concurrent
// INSTALL. Coverage for the once-on-failure retry contract requires a
// mocked driver.ExecerContext and is tracked as a follow-up.
func TestConcurrentOpen(t *testing.T) {
	dir := t.TempDir()
	th := testh.New(t)
	probe := &source.Source{
		Handle:   "@conc_probe",
		Type:     drivertype.DuckDB,
		Location: "duckdb://" + filepath.Join(dir, "probe.duckdb"),
	}
	th.Add(probe)
	drv := th.DriverFor(probe)

	const n = 8
	var g errgroup.Group
	for i := range n {
		g.Go(func() error {
			// Each goroutine uses a distinct file so we don't trip DuckDB's
			// process-exclusive file lock; the point here is parallel INSTALL +
			// LOAD + SET via the connector init fn.
			src := &source.Source{
				Handle:   "@conc",
				Type:     drivertype.DuckDB,
				Location: "duckdb://" + filepath.Join(dir, "conc_"+strconv.Itoa(i)+".duckdb"),
			}
			grip, err := drv.Open(th.Context, src, driver.ModeReadWrite)
			if err != nil {
				return err
			}
			defer grip.Close()
			db, err := grip.DB(th.Context)
			if err != nil {
				return err
			}
			var got int
			return db.QueryRowContext(th.Context, "SELECT 1").Scan(&got)
		})
	}
	require.NoError(t, g.Wait())
}
