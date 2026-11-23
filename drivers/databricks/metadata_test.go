package databricks_test

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/databricks"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

func TestKindFromDBTypeName(t *testing.T) {
	testCases := []struct {
		dbType string
		want   kind.Kind
	}{
		// Names as reported for result-set columns by databricks-sql-go.
		{"BOOLEAN", kind.Bool},
		{"TINYINT", kind.Int},
		{"SMALLINT", kind.Int},
		{"INT", kind.Int},
		{"BIGINT", kind.Int},
		{"FLOAT", kind.Float},
		{"DOUBLE", kind.Float},
		{"DECIMAL", kind.Decimal},
		{"STRING", kind.Text},
		{"VARCHAR", kind.Text},
		{"CHAR", kind.Text},
		{"BINARY", kind.Bytes},
		{"DATE", kind.Date},
		{"TIMESTAMP", kind.Datetime},
		{"TIMESTAMP_NTZ", kind.Datetime},
		{"ARRAY", kind.Text},
		{"MAP", kind.Text},
		{"STRUCT", kind.Text},
		{"UNION", kind.Text},
		{"USER_DEFINED", kind.Text},
		{"INTERVAL_YEAR_MONTH", kind.Text},
		{"INTERVAL_DAY_TIME", kind.Text},
		{"NULL", kind.Null},

		// Names as reported by information_schema.
		{"LONG", kind.Int},
		{"SHORT", kind.Int},
		{"BYTE", kind.Int},
		{"VARIANT", kind.Text},
		{"INTERVAL", kind.Text},
		{"VOID", kind.Null},

		// Full type names, with parameters.
		{"decimal(10,2)", kind.Decimal},
		{"DECIMAL(38, 18)", kind.Decimal},
		{"varchar(20)", kind.Text},
		{"array<int>", kind.Text},
		{"map<string,array<bigint>>", kind.Text},
		{"struct<a:int,b:string>", kind.Text},
		{"interval day to second", kind.Text},
		{"timestamp_ntz", kind.Datetime},

		// Case and whitespace.
		{"  bigint ", kind.Int},
		{"Boolean", kind.Bool},

		// Unrecognized.
		{"", kind.Unknown},
		{"GEOMETRYCOLLECTION", kind.Unknown},
		{"NOT_A_TYPE", kind.Unknown},
	}

	log := lgt.New(t)
	for _, tc := range testCases {
		t.Run(tc.dbType, func(t *testing.T) {
			require.Equal(t, tc.want, databricks.KindFromDBTypeName(log, "col", tc.dbType))
		})
	}

	// A nil logger must be tolerated.
	require.Equal(t, kind.Unknown, databricks.KindFromDBTypeName((*slog.Logger)(nil), "col", "NOT_A_TYPE"))
}

func TestParseSemver(t *testing.T) {
	testCases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"3.5.0 a1b2c3d4e5f6", "v3.5.0", false},
		{"4.0.0", "v4.0.0", false},
		{"3.5", "v3.5.0", false},
		{"v3.5.1", "v3.5.1", false},
		{"  3.5.0  ", "v3.5.0", false},
		{"", "", true},
		{"unknown", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := databricks.ParseSemver(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestTableTypeFromDBType(t *testing.T) {
	testCases := map[string]string{
		"MANAGED":           sqlz.TableTypeTable,
		"EXTERNAL":          sqlz.TableTypeTable,
		"FOREIGN":           sqlz.TableTypeTable,
		"STREAMING_TABLE":   sqlz.TableTypeTable,
		"VIEW":              sqlz.TableTypeView,
		"view":              sqlz.TableTypeView,
		"METRIC_VIEW":       sqlz.TableTypeView,
		"MATERIALIZED_VIEW": sqlz.TableTypeMaterializedView,
		"":                  sqlz.TableTypeTable,
	}

	for in, want := range testCases {
		t.Run(in, func(t *testing.T) {
			require.Equal(t, want, databricks.TableTypeFromDBType(in))
		})
	}
}

func TestRequireInfoSchema(t *testing.T) {
	require.NoError(t, databricks.RequireInfoSchema("main"))
	require.NoError(t, databricks.RequireInfoSchema("samples"))
	require.Error(t, databricks.RequireInfoSchema("hive_metastore"))
	require.Error(t, databricks.RequireInfoSchema("HIVE_METASTORE"))
	require.Error(t, databricks.RequireInfoSchema("spark_catalog"))
}

func TestIsNotExist(t *testing.T) {
	testCases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("something else"), false},
		{errors.New("[TABLE_OR_VIEW_NOT_FOUND] The table or view `x` cannot be found"), true},
		{errors.New("[SCHEMA_NOT_FOUND] The schema `x` cannot be found"), true},
		{errors.New("[CATALOG_NOT_FOUND] The catalog `x` cannot be found"), true},
		{errors.New("[NO_SUCH_CATALOG_EXCEPTION] Catalog 'x' was not found"), true},
		{errors.New("[PARSE_SYNTAX_ERROR] Syntax error at or near 'FORM'"), false},
	}

	for _, tc := range testCases {
		name := "nil"
		if tc.err != nil {
			name = tc.err.Error()
		}
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, databricks.IsNotExist(tc.err))
		})
	}
}
