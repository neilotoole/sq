package duckdb_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
)

// TestKindMapping creates a fresh DB from type_test.ddl and verifies each
// column maps to the expected kind.Kind.
func TestKindMapping(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "type_test.duckdb")

	ddl, err := os.ReadFile("testdata/type_test.ddl")
	require.NoError(t, err)

	// Build the DB out-of-band via the raw go-duckdb driver before handing
	// the file to sq via testh. (sq's testh would expect the file to exist
	// and be registered in sources.sq.yml, but type_test.ddl is too schema-
	// only-volatile to register globally.)
	rawDB, err := sql.Open("duckdb", dbPath)
	require.NoError(t, err)
	_, err = rawDB.ExecContext(context.Background(), string(ddl))
	require.NoError(t, err)
	require.NoError(t, rawDB.Close())

	// Now wrap as an sq Source and inspect via the driver's metadata path.
	th := testh.New(t)
	src := &source.Source{
		Handle:   "@type_test",
		Type:     drivertype.DuckDB,
		Location: "duckdb://" + dbPath,
	}
	th.Add(src)

	tblMeta, err := th.TableMetadata(src, "type_test")
	require.NoError(t, err)

	want := map[string]kind.Kind{
		"col_bool":        kind.Bool,
		"col_tinyint":     kind.Int,
		"col_smallint":    kind.Int,
		"col_int":         kind.Int,
		"col_bigint":      kind.Int,
		"col_hugeint":     kind.Decimal,
		"col_uhugeint":    kind.Decimal,
		"col_utinyint":    kind.Int,
		"col_usmallint":   kind.Int,
		"col_uint":        kind.Int,
		"col_ubigint":     kind.Decimal,
		"col_float":       kind.Float,
		"col_double":      kind.Float,
		"col_decimal":     kind.Decimal,
		"col_varchar":     kind.Text,
		"col_blob":        kind.Bytes,
		"col_date":        kind.Date,
		"col_time":        kind.Time,
		"col_timestamp":   kind.Datetime,
		"col_timestamptz": kind.Datetime,
		"col_interval":    kind.Text,
		"col_uuid":        kind.Text,
		"col_json":        kind.Text,
		"col_list":        kind.Text,
		"col_struct":      kind.Text,
		"col_map":         kind.Text,
		"col_enum":        kind.Text,
	}

	got := map[string]kind.Kind{}
	for _, c := range tblMeta.Columns {
		got[c.Name] = c.Kind
	}

	for name, k := range want {
		require.Equal(t, k, got[name], "col %s: expected kind %s, got %s (data_type=%q)",
			name, k, got[name], tblMeta.Columns)
	}
}
