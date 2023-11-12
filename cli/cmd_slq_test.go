package cli_test

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/neilotoole/sq/libsq/core/tablefq"

	"github.com/neilotoole/sq/testh/proj"

	"github.com/neilotoole/sq/cli"

	"github.com/neilotoole/sq/cli/testrun"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// TestCmdSLQ_Insert_Create tests "sq QUERY --insert=@src.tbl".
func TestCmdSLQ_Insert_Create(t *testing.T) {
	th := testh.New(t)
	originSrc, destSrc := th.Source(sakila.SL3), th.Source(sakila.SL3)
	srcTbl := sakila.TblActor
	if th.IsMonotable(originSrc) {
		srcTbl = source.MonotableName
	}

	destTbl := stringz.UniqSuffix(sakila.TblActor + "_copy")

	tr := testrun.New(th.Context, t, nil).Add(*originSrc)
	if destSrc.Handle != originSrc.Handle {
		tr.Add(*destSrc)
	}

	insertTo := fmt.Sprintf("%s.%s", destSrc.Handle, destTbl)
	cols := stringz.PrefixSlice(sakila.TblActorCols(), ".")
	query := fmt.Sprintf("%s.%s | %s", originSrc.Handle, srcTbl, strings.Join(cols, ", "))

	err := tr.Exec("slq", "--insert="+insertTo, query)
	require.NoError(t, err)

	sink, err := th.QuerySQL(destSrc, nil, "select * from "+destTbl)
	require.NoError(t, err)
	require.Equal(t, sakila.TblActorCount, len(sink.Recs))
}

// TestCmdSLQ_Insert tests "sq slq QUERY --insert=dest.tbl".
func TestCmdSLQ_Insert(t *testing.T) {
	for _, origin := range sakila.SQLLatest() {
		origin := origin

		t.Run("origin_"+origin, func(t *testing.T) {
			for _, dest := range sakila.SQLLatest() {
				dest := dest

				t.Run("dest_"+dest, func(t *testing.T) {
					t.Parallel()

					th := testh.New(t)
					originSrc, destSrc := th.Source(origin), th.Source(dest)
					srcTbl := sakila.TblActor
					if th.IsMonotable(originSrc) {
						srcTbl = source.MonotableName
					}

					// To avoid dirtying the destination table, we make a copy
					// of it (without data).
					tblName := th.CopyTable(
						true,
						destSrc,
						tablefq.From(sakila.TblActor),
						tablefq.T{},
						false,
					)

					tr := testrun.New(th.Context, t, nil).Add(*originSrc)
					if destSrc.Handle != originSrc.Handle {
						tr.Add(*destSrc)
					}

					insertTo := fmt.Sprintf("%s.%s", destSrc.Handle, tblName)
					cols := stringz.PrefixSlice(sakila.TblActorCols(), ".")
					query := fmt.Sprintf("%s.%s | %s", originSrc.Handle, srcTbl, strings.Join(cols, ", "))

					err := tr.Exec("slq", "--insert="+insertTo, query)
					require.NoError(t, err)

					sink, err := th.QuerySQL(destSrc, nil, "select * from "+tblName)
					require.NoError(t, err)
					require.Equal(t, sakila.TblActorCount, len(sink.Recs))
				})
			}
		})
	}
}

func TestCmdSLQ_CSV(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.CSVActor)
	tr := testrun.New(th.Context, t, nil).Add(*src)
	err := tr.Exec("slq", "--header=false", "--csv", fmt.Sprintf("%s.data", src.Handle))
	require.NoError(t, err)

	recs := tr.BindCSV()
	require.Equal(t, sakila.TblActorCount, len(recs))
}

// TestCmdSLQ_OutputFlag verifies that flag --output=<file> works.
func TestCmdSLQ_OutputFlag(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.SL3)
	tr := testrun.New(th.Context, t, nil).Add(*src)
	outputFile, err := os.CreateTemp("", t.Name())
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, outputFile.Close())
		assert.NoError(t, os.Remove(outputFile.Name()))
	})

	err = tr.Exec("slq",
		"--header=false", "--csv", fmt.Sprintf("%s.%s", src.Handle, sakila.TblActor),
		"--output", outputFile.Name())
	require.NoError(t, err)

	recs, err := csv.NewReader(outputFile).ReadAll()
	require.NoError(t, err)
	require.Equal(t, sakila.TblActorCount, len(recs))
}

func TestCmdSLQ_Join_cross_source(t *testing.T) {
	const queryTpl = `%s.customer | join(%s.address, .address_id) | where(.customer_id == %d) | .[0] | .customer_id, .email, .city_id` //nolint:lll
	handles := sakila.SQLLatest()

	// Attempt to join every SQL test source against every SQL test source.
	for _, h1 := range handles {
		h1 := h1

		t.Run("origin_"+h1, func(t *testing.T) {
			for _, h2 := range handles {
				h2 := h2

				t.Run("dest_"+h2, func(t *testing.T) {
					t.Parallel()

					th := testh.New(t)
					src1, src2 := th.Source(h1), th.Source(h2)

					tr := testrun.New(th.Context, t, nil).Add(*src1)
					if src2.Handle != src1.Handle {
						tr.Add(*src2)
					}

					query := fmt.Sprintf(queryTpl, src1.Handle, src2.Handle, sakila.MillerCustID)

					err := tr.Exec("slq", "--header=false", "--csv", query)
					require.NoError(t, err)

					recs := tr.BindCSV()
					require.Equal(t, 1, len(recs), "should only be one matching record")
					require.Equal(t, 3, len(recs[0]), "should have three fields")
					require.Equal(t, strconv.Itoa(sakila.MillerCustID), recs[0][0])
					require.Equal(t, sakila.MillerEmail, recs[0][1])
					require.Equal(t, strconv.Itoa(sakila.MillerCityID), recs[0][2])
				})
			}
		})
	}
}

// TestCmdSLQ_ActiveSrcHandle verifies that source.ActiveHandle is
// interpreted as the active src in a SLQ query.
func TestCmdSLQ_ActiveSrcHandle(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	// 1. Verify that the query works as expected using the actual src handle
	tr := testrun.New(th.Context, t, nil).Add(*src).Hush()

	require.Equal(t, src.Handle, tr.Run.Config.Collection.Active().Handle)
	err := tr.Exec("slq", "--header=false", "--csv", "@sakila_sl3.actor")
	require.NoError(t, err)
	recs := tr.BindCSV()
	require.Equal(t, sakila.TblActorCount, len(recs))

	// 2. Verify that it works using source.ActiveHandle as the src handle
	tr = testrun.New(th.Context, t, nil).Add(*src).Hush()
	require.Equal(t, src.Handle, tr.Run.Config.Collection.Active().Handle)
	err = tr.Exec("slq", "--header=false", "--csv", source.ActiveHandle+".actor")
	require.NoError(t, err)
	recs = tr.BindCSV()
	require.Equal(t, sakila.TblActorCount, len(recs))
}

func TestCmdSLQ_PreprocessFlagArgVars(t *testing.T) {
	testCases := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{
			name: "empty",
			in:   []string{},
			want: []string{},
		},
		{
			name: "no flags",
			in:   []string{".actor"},
			want: []string{".actor"},
		},
		{
			name: "non-arg flag",
			in:   []string{"--json", ".actor"},
			want: []string{"--json", ".actor"},
		},
		{
			name: "non-arg flag with value",
			in:   []string{"--json", "true", ".actor"},
			want: []string{"--json", "true", ".actor"},
		},
		{
			name: "single arg flag",
			in:   []string{"--arg", "name", "TOM", ".actor"},
			want: []string{"--arg", "name:TOM", ".actor"},
		},
		{
			name:    "invalid arg name",
			in:      []string{"--arg", "na me", "TOM", ".actor"},
			wantErr: true,
		},
		{
			name:    "invalid arg name (with colon)",
			in:      []string{"--arg", "na:me", "TOM", ".actor"},
			wantErr: true,
		},
		{
			name: "colon in value",
			in:   []string{"--arg", "name", "T:OM", ".actor"},
			want: []string{"--arg", "name:T:OM", ".actor"},
		},
		{
			name: "single arg flag with whitespace",
			in:   []string{"--arg", "name", "TOM DOWD", ".actor"},
			want: []string{"--arg", "name:TOM DOWD", ".actor"},
		},
		{
			name: "two arg flags",
			in:   []string{"--arg", "name", "TOM", "--arg", "eyes", "blue", ".actor"},
			want: []string{"--arg", "name:TOM", "--arg", "eyes:blue", ".actor"},
		},
		{
			name: "two arg flags with interspersed flag",
			in:   []string{"--arg", "name", "TOM", "--json", "true", "--arg", "eyes", "blue", ".actor"},
			want: []string{"--arg", "name:TOM", "--json", "true", "--arg", "eyes:blue", ".actor"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, gotErr := cli.PreprocessFlagArgVars(tc.in)
			if tc.wantErr {
				t.Log(gotErr.Error())
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.EqualValues(t, tc.want, got)
		})
	}
}

func TestCmdSLQ_FlagActiveSource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tr := testrun.New(ctx, t, nil)

	// @sqlite will be the active source
	require.NoError(t, tr.Exec("add", proj.Abs(sakila.PathSL3), "--handle", "@sqlite"))

	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("add", proj.Abs(sakila.PathCSVActor), "--handle", "@csv"))

	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec(
		"--csv",
		"--no-header",
		".actor",
	))
	require.Len(t, tr.BindCSV(), sakila.TblActorCount)

	// Now, use flag.ActiveSrc to switch the source.
	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec(
		"--csv",
		"--no-header",
		"--src", "@csv",
		".data",
	))
	require.Len(t, tr.BindCSV(), sakila.TblActorCount)

	// Double check that we didn't change the persisted active source
	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("src", "--json"))
	require.Equal(t, "@sqlite", tr.BindMap()["handle"])
}

func TestCmdSLQ_FlagActiveSchema(t *testing.T) {
	testCases := []struct {
		handle string

		// skipReason is the reason to skip the test case.
		skipReason string

		// defaultCatalog is the default catalog for the source.
		defaultCatalog string

		// defaultSchema is the default schema for the source,
		// e.g. "public" for Pg, or "dbo" for SQL Server.
		defaultSchema string

		// altCatalog is the name of a second catalog that
		// we know to exist in the source. For example, "model"
		// for SQL Server, or "postgres" for Postgres.
		altCatalog string

		// expectSchemaFuncValue is the value we expect
		// the SLQ "schema()" func to return. Generally one would
		// expect this to be the same as the value supplied
		// to --src.schema, but for SQL Server, the SLQ "schema()"
		// func does not honor --src.schema. This is a limitation in
		// SQL Server itself; it's not possible to change the default
		// schema for a connection.
		expectSchemaFuncValue string
	}{
		{
			handle:                sakila.Pg,
			defaultCatalog:        "sakila",
			defaultSchema:         "public",
			altCatalog:            "postgres",
			expectSchemaFuncValue: "information_schema",
		},
		{
			handle:                sakila.MS,
			defaultCatalog:        "sakila",
			defaultSchema:         "dbo",
			altCatalog:            "model",
			expectSchemaFuncValue: "dbo",
		},
		{
			handle:                sakila.My,
			defaultCatalog:        "def",
			defaultSchema:         "sakila",
			altCatalog:            "model",
			expectSchemaFuncValue: "information_schema",
		},
		{
			handle: sakila.SL3,
			skipReason: `SQLite 'schema' support requires implementing 'ATTACH DATABASE'.
See: https://github.com/neilotoole/sq/issues/324`,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.handle, func(t *testing.T) {
			if tc.skipReason != "" {
				t.Skip(tc.skipReason)
				return
			}
			th := testh.New(t)
			src := th.Source(tc.handle)

			tr := testrun.New(th.Context, t, nil).
				Add(*th.Source(sakila.CSVActor), *src)

			// Confirm that sakila.CSVActor is the active source.
			require.NoError(t, tr.Exec("src"))
			require.Equal(t, sakila.CSVActor, tr.OutString())

			// Test combination of --src and --src.schema
			const qInfoSchemaActor = `.tables | .table_catalog, .table_schema, .table_name, .table_type | where(.table_name == "actor")` //nolint:lll
			require.NoError(t, tr.Reset().Exec("--csv", "-H",
				"--src", tc.handle,
				"--src.schema", "information_schema",
				qInfoSchemaActor,
			))

			want := [][]string{{tc.defaultCatalog, tc.defaultSchema, "actor", "BASE TABLE"}}
			got := tr.BindCSV()
			require.Equal(t, want, got)

			require.NoError(t, tr.Reset().Exec("src", tc.handle))

			require.NoError(t, tr.Reset().Exec("-H", "schema()"))
			require.Equal(t, tc.defaultSchema, tr.OutString())

			// Test just --src.schema (schema part only)
			require.NoError(t, tr.Reset().Exec("--csv", "-H",
				"--src.schema", "information_schema",
				qInfoSchemaActor,
			))
			got = tr.BindCSV()
			require.Equal(t, want, got)

			if th.SQLDriverFor(src).Dialect().Catalog {
				// Test --src.schema (catalog and schema parts)
				require.NoError(t, tr.Reset().Exec("--csv", "-H",
					"--src.schema", tc.altCatalog+".information_schema",
					`.schemata | .catalog_name | unique`,
				))

				got = tr.BindCSV()
				require.Equal(t, tc.altCatalog, got[0][0])
			}

			// Note that for SQL Server, the SLQ "schema()"
			// func does not honor --src.schema. This is a limitation in
			// SQL Server itself; it's not possible to change the default
			// schema for a connection.
			require.NoError(t, tr.Reset().Exec("-H",
				"--src.schema", "information_schema",
				"schema()"))
			require.Equal(t, tc.expectSchemaFuncValue, tr.OutString())
		})
	}
}
