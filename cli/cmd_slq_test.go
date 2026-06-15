package cli_test

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	goccy "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
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

// TestCmdSLQ_Insert_MultipleSchemas is an end-to-end regression test for
// https://github.com/neilotoole/sq/issues/484. Inserting into a table that
// also exists in another schema of the destination must succeed. Before the
// fix, the MySQL/Postgres TableExists check counted the table name across all
// schemas, so the create-if-not-exists hook (libsq.DBWriterCreateTableIfNotExistsHook)
// misjudged existence: with the name present in two schemas it reported the
// table as missing and tried to CREATE the already-existing table, failing
// with "table already exists" — exactly the symptom reported in #484.
//
// This drives the full --insert path (the hook -> TableExists -> CreateTable),
// which the unit test TestDriver_TableExists_MultipleSchemas does not exercise.
// Only MySQL and Postgres were affected, so only those are tested here.
func TestCmdSLQ_Insert_MultipleSchemas(t *testing.T) {
	for _, dest := range []string{sakila.Pg, sakila.My} {
		t.Run(dest, func(t *testing.T) {
			th, destSrc, drvr, _, db := testh.NewWith(t, dest)
			ctx := th.Context

			conn, err := db.Conn(ctx)
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, conn.Close()) })

			// destTbl is an empty actor copy in the current schema; it stands in
			// for the user's pre-existing destination table. th.CopyTable
			// registers its own cleanup.
			destTbl := th.CopyTable(true, destSrc, tablefq.From(sakila.TblActor), tablefq.T{}, false)

			// Create a same-named table in another schema to trigger #484: the
			// name now exists in two schemas at once.
			otherSchema := "test_schema_" + stringz.Uniq8()
			require.NoError(t, drvr.CreateSchema(ctx, conn, otherSchema))
			t.Cleanup(func() { assert.NoError(t, drvr.DropSchema(ctx, conn, otherSchema)) })
			_, err = drvr.CopyTable(ctx, conn, tablefq.From(sakila.TblActor),
				tablefq.T{Schema: otherSchema, Table: destTbl}, false)
			require.NoError(t, err)

			// Insert actor rows into the current-schema table. Before the fix
			// this failed with "table already exists" because TableExists saw
			// the name in both schemas and the hook tried to re-CREATE it.
			tr := testrun.New(ctx, t, nil).Add(*destSrc)
			insertTo := fmt.Sprintf("%s.%s", destSrc.Handle, destTbl)
			cols := stringz.PrefixSlice(sakila.TblActorCols(), ".")
			query := fmt.Sprintf("%s.%s | %s", destSrc.Handle, sakila.TblActor, strings.Join(cols, ", "))

			err = tr.Exec("slq", "--insert="+insertTo, query)
			require.NoError(t, err)

			sink, err := th.QuerySQL(destSrc, nil, "select * from "+destTbl)
			require.NoError(t, err)
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
		})
	}
}

// TestCmdSLQ_Insert tests the "sq slq QUERY --insert=@dest.tbl" functionality,
// which executes an SLQ query against one source and inserts the results into
// a table in another (or the same) source. This is similar to TestCmdSQL_Insert
// but uses SLQ (sq's query language) instead of raw SQL.
//
// The test performs the following for each origin/destination database combination:
//  1. Creates an empty copy of the actor table in the destination database
//  2. Executes an SLQ query against the origin database's actor table
//  3. Uses --insert to pipe results directly into the destination table
//  4. Verifies all 200 actor rows were successfully transferred
//
// This exercises:
//   - Cross-database querying via SLQ and insertion (e.g., SQLite → PostgreSQL)
//   - Same-database query-to-insert (e.g., PostgreSQL → PostgreSQL)
//   - The batch insert mechanism for efficiently transferring multiple rows
//   - SLQ query parsing and execution across different database backends
//   - The CLI's ability to manage multiple source connections simultaneously
//
// The test matrix covers all combinations of supported SQL databases as both
// origin (data source) and destination (insert target).
func TestCmdSLQ_Insert(t *testing.T) {
	for _, origin := range sakila.SQLLatest() {
		t.Run("origin_"+origin, func(t *testing.T) {
			for _, dest := range sakila.SQLLatest() {
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
	err := tr.Exec("slq", "--header=false", "--csv", src.Handle+".data")
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
		t.Run("origin_"+h1, func(t *testing.T) {
			for _, h2 := range handles {
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
		{
			handle: sakila.Duck,
			skipReason: `DuckDB excludes information_schema from its referenceable schema
list, so --src.schema=information_schema is not supported.
See: https://github.com/neilotoole/sq/issues/437`,
		},
	}

	for _, tc := range testCases {
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

// TestCmdSLQ_RenderSQL_Text verifies that --render-sql with the default
// (text) format prints the rendered SQL without executing it.
func TestCmdSLQ_RenderSQL_Text(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	tr := testrun.New(th.Context, t, nil).Add(*src).Hush()
	require.NoError(t, tr.Exec("slq", "--render-sql", "--monochrome", src.Handle+".actor"))

	out := strings.TrimSpace(tr.OutString())
	require.True(t, strings.HasPrefix(strings.ToUpper(out), "SELECT"),
		"expected SQL starting with SELECT, got: %s", out)
	require.Contains(t, strings.ToLower(out), "actor")
}

// TestCmdSLQ_RenderSQL_JSON verifies that --render-sql --format=json
// emits the structured payload.
func TestCmdSLQ_RenderSQL_JSON(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	tr := testrun.New(th.Context, t, nil).Add(*src).Hush()
	require.NoError(t, tr.Exec("slq", "--render-sql", "--format=json", src.Handle+".actor"))

	var got output.SQLPayload
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &got))
	require.Equal(t, src.Handle+".actor", got.SLQ)
	require.Contains(t, strings.ToUpper(got.SQL), "SELECT")
	require.Contains(t, strings.ToLower(got.SQL), "actor")
	require.Equal(t, "sqlite3", got.Dialect)
	require.Equal(t, src.Handle, got.Sources.Target)
	require.Equal(t, []string{src.Handle}, got.Sources.Inputs)
}

// TestCmdSLQ_RenderSQL_JSONL verifies that --render-sql --format=jsonl
// emits a single-line JSON payload.
func TestCmdSLQ_RenderSQL_JSONL(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	tr := testrun.New(th.Context, t, nil).Add(*src).Hush()
	require.NoError(t, tr.Exec("slq", "--render-sql", "--format=jsonl", src.Handle+".actor"))

	// tr.Out.String() retains the trailing newline written by the JSONL writer;
	// tr.OutString() would strip it. Use the raw buffer to verify exactly one
	// newline (the trailing one), confirming compact single-line output.
	out := tr.Out.String()
	require.Equal(t, 1, strings.Count(out, "\n"))
	require.True(t, strings.HasSuffix(out, "\n"))

	var got output.SQLPayload
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &got))
	require.Equal(t, "sqlite3", got.Dialect)
}

// TestCmdSLQ_RenderSQL_YAML verifies that --render-sql --format=yaml
// emits the payload as YAML.
func TestCmdSLQ_RenderSQL_YAML(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	tr := testrun.New(th.Context, t, nil).Add(*src).Hush()
	require.NoError(t, tr.Exec("slq", "--render-sql", "--format=yaml", src.Handle+".actor"))

	var got output.SQLPayload
	require.NoError(t, goccy.Unmarshal(tr.Out.Bytes(), &got))
	require.Equal(t, "sqlite3", got.Dialect)
	require.Equal(t, src.Handle, got.Sources.Target)
}

// TestCmdSLQ_RenderSQL_CSV_FallsBackToText verifies that requesting a
// format with no SQL writer of its own (e.g. csv) falls back to the
// text writer, per the existing OptFormat fallback convention.
// execSLQRenderSQL logs a Warn when this happens; the test only
// asserts the user-visible output (plain SQL), since the log goes
// through the slog handler and is not surfaced on stderr.
func TestCmdSLQ_RenderSQL_CSV_FallsBackToText(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	tr := testrun.New(th.Context, t, nil).Add(*src).Hush()
	require.NoError(t, tr.Exec("slq", "--render-sql", "--format=csv", "--monochrome", src.Handle+".actor"))

	out := strings.TrimSpace(tr.OutString())
	require.True(t, strings.HasPrefix(strings.ToUpper(out), "SELECT"))
}

// TestCmdSLQ_RenderSQL_InsertConflict verifies that combining
// --render-sql with --insert is rejected.
func TestCmdSLQ_RenderSQL_InsertConflict(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	tr := testrun.New(th.Context, t, nil).Add(*src).Hush()
	err := tr.Exec("slq", "--render-sql", "--insert="+src.Handle+".foo", src.Handle+".actor")
	require.Error(t, err)
	require.Contains(t, err.Error(), "render-sql")
}

// TestCmdSLQ_RenderSQL_ArgSubstitution verifies that --arg values
// appear in both the rendered SQL and the JSON payload's args field.
func TestCmdSLQ_RenderSQL_ArgSubstitution(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	tr := testrun.New(th.Context, t, nil).Add(*src).Hush()
	require.NoError(t, tr.Exec("slq",
		"--render-sql", "--format=json",
		"--arg", "name:TOM",
		src.Handle+".actor | .first_name == $name"))

	var got output.SQLPayload
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &got))
	require.Equal(t, "TOM", got.Args["name"])
	require.Contains(t, got.SQL, "TOM")
}

// TestCmdSLQ_RenderSQL_CrossSource verifies that a cross-source SLQ
// (SQLite + CSV) renders against the synthetic join DB. Sources.Target
// should be the @join_<uniq> handle, Inputs should list both user-named
// handles, and Dialect should be the scratch SQLite dialect.
func TestCmdSLQ_RenderSQL_CrossSource(t *testing.T) {
	th := testh.New(t)
	sl3 := th.Source(sakila.SL3)
	csvSrc := th.Source(sakila.CSVActor)

	tr := testrun.New(th.Context, t, nil).Add(*sl3, *csvSrc).Hush()
	require.NoError(t, tr.Exec("slq",
		"--render-sql", "--format=json",
		"@sakila_sl3.actor | join(@sakila_csv_actor.data, .actor_id)"))

	var got output.SQLPayload
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), &got))
	require.True(t, strings.HasPrefix(got.Sources.Target, "@join_"),
		"expected Sources.Target to start with @join_, got: %s", got.Sources.Target)
	require.ElementsMatch(t,
		[]string{sakila.SL3, sakila.CSVActor},
		got.Sources.Inputs)
	require.Equal(t, "sqlite3", got.Dialect,
		"join DB dialect should be the scratch sqlite3")
}

// TestCmdSLQ_RenderSQL_InvalidSLQ verifies that an unparseable SLQ
// surfaces an error and does not print partial SQL to stdout.
func TestCmdSLQ_RenderSQL_InvalidSLQ(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	tr := testrun.New(th.Context, t, nil).Add(*src).Hush()
	err := tr.Exec("slq", "--render-sql", "@no_such_handle.foo")
	require.Error(t, err)
	require.Empty(t, strings.TrimSpace(tr.OutString()),
		"stdout should be empty on error, got: %s", tr.OutString())
}

// TestRenderSQLSupportsFormat is a parity test pinning the
// renderSQLSupportsFormat allow-list to format.All(). Adding a new
// format.Format to format.All() will fail this test until the new
// format is added to the case table (and, importantly, until a
// corresponding writer dispatch is added to newWriters or the
// new format is explicitly listed as falling back to text).
func TestRenderSQLSupportsFormat(t *testing.T) {
	cases := []struct {
		fm        format.Format
		supported bool
	}{
		{format.Text, true},
		{format.Raw, true},
		{format.JSON, true},
		{format.JSONL, true},
		{format.YAML, true},
		{format.JSONA, false},
		{format.CSV, false},
		{format.TSV, false},
		{format.HTML, false},
		{format.Markdown, false},
		{format.XML, false},
		{format.XLSX, false},
	}

	seen := make(map[format.Format]bool, len(cases))
	for _, tc := range cases {
		t.Run(string(tc.fm), func(t *testing.T) {
			seen[tc.fm] = true
			require.Equal(t, tc.supported, cli.RenderSQLSupportsFormat(tc.fm))
		})
	}

	for _, fm := range format.All() {
		require.True(t, seen[fm],
			"format %q missing from TestRenderSQLSupportsFormat cases; "+
				"add it (and a writer dispatch in newWriters if appropriate)", fm)
	}
}

// TestCmdSLQ_RenderSQL_ExplicitFalse verifies that an explicit
// --render-sql=false does not trigger render-only mode; the query
// should execute normally and emit records, not SQL text.
func TestCmdSLQ_RenderSQL_ExplicitFalse(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	tr := testrun.New(th.Context, t, nil).Add(*src).Hush()
	require.NoError(t, tr.Exec("slq", "--render-sql=false", "--format=json", src.Handle+".actor"))

	// A real query result is a JSON array of records; a render-sql JSON
	// payload would be a single object starting with '{'.
	out := strings.TrimSpace(tr.OutString())
	require.True(t, strings.HasPrefix(out, "["),
		"expected query results (JSON array), got: %s", out)
}

// TestCmdSLQ_RenderSQL_NotOnSQLCmd verifies that --render-sql is not
// available on the sq sql command (different command, different flag
// set).
func TestCmdSLQ_RenderSQL_NotOnSQLCmd(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	tr := testrun.New(th.Context, t, nil).Add(*src).Hush()
	err := tr.Exec("sql", "--render-sql", "SELECT 1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown flag")
}

// TestCmdSLQ_NumericSchema tests SLQ query execution against tables in schemas
// with numeric or numeric-prefixed names. This validates the grammar fix for
// issue #470 in the actual query execution path.
// See: https://github.com/neilotoole/sq/issues/470
func TestCmdSLQ_NumericSchema(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	testCases := []struct {
		name   string
		schema string
	}{
		{"pure_numeric", "99999"},
		{"numeric_prefixed", "123query"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			ctx := th.Context
			src := th.Source(sakila.Pg)

			// Create unique schema name for this test.
			schemaName := tc.schema + "_" + stringz.Uniq8()

			// Create the numeric schema directly via SQL.
			db := th.OpenDB(src)
			_, err := db.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schemaName))
			require.NoError(t, err)
			_, err = db.ExecContext(ctx, fmt.Sprintf(`CREATE SCHEMA %q`, schemaName))
			require.NoError(t, err)
			t.Cleanup(func() {
				_, _ = db.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schemaName))
			})

			// Create a test table in the numeric schema.
			tblName := "query_test_tbl"
			_, err = db.ExecContext(ctx, fmt.Sprintf(
				`CREATE TABLE %q.%q (id serial PRIMARY KEY, name text, value int)`,
				schemaName, tblName))
			require.NoError(t, err)

			// Insert test data.
			_, err = db.ExecContext(ctx, fmt.Sprintf(
				`INSERT INTO %q.%q (name, value) VALUES ('alice', 10), ('bob', 20), ('charlie', 30)`,
				schemaName, tblName))
			require.NoError(t, err)

			// Execute SLQ query with --src.schema pointing to numeric schema.
			tr := testrun.New(ctx, t, nil).Hush().Add(*src)
			err = tr.Exec(
				"--csv", "-H",
				"--src.schema", schemaName,
				"."+tblName+" | .name, .value",
			)
			require.NoError(t, err, "sq query with --src.schema %q should succeed", schemaName)

			// Verify query results.
			got := tr.BindCSV()
			want := [][]string{
				{"alice", "10"},
				{"bob", "20"},
				{"charlie", "30"},
			}
			require.Equal(t, want, got, "query results should match expected data")

			// Also test with a filter to exercise more of the query path.
			tr2 := testrun.New(ctx, t, nil).Hush().Add(*src)
			err = tr2.Exec(
				"--csv", "-H",
				"--src.schema", schemaName,
				"."+tblName+" | .name, .value | where(.value > 15)",
			)
			require.NoError(t, err, "sq query with filter should succeed")

			got2 := tr2.BindCSV()
			want2 := [][]string{
				{"bob", "20"},
				{"charlie", "30"},
			}
			require.Equal(t, want2, got2, "filtered results should match")
		})
	}
}

// TestSLQ_DuckDB_DoesNotModifyMtime verifies that running an SLQ
// query against a DuckDB source does not touch the file's mtime.
func TestSLQ_DuckDB_DoesNotModifyMtime(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Duck)
	path := strings.TrimPrefix(src.Location, "duckdb://")

	statBefore, err := os.Stat(path)
	require.NoError(t, err)

	tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
	require.NoError(t, tr.Exec("slq", src.Handle+" | .actor | .first_name | .[0:5]"))

	statAfter, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, statBefore.ModTime(), statAfter.ModTime(),
		"DuckDB file mtime must not change after sq slq")
}

// TestSLQ_DuckDB_SrcSchema_DoesNotModifyMtime verifies that --src.schema's
// catalog/schema validation (which pre-opens the source via Grips.Open)
// happens under the read-only context, not the implicit RW. Without the
// RO hint reaching the pre-open, verifySourceCatalogSchema caches a RW
// grip that later RO opens silently inherit.
func TestSLQ_DuckDB_SrcSchema_DoesNotModifyMtime(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Duck)
	path := strings.TrimPrefix(src.Location, "duckdb://")

	statBefore, err := os.Stat(path)
	require.NoError(t, err)

	tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
	require.NoError(t, tr.Exec("slq", "--src.schema=main",
		src.Handle+" | .actor | .first_name | .[0:3]"))

	statAfter, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, statBefore.ModTime(), statAfter.ModTime(),
		"DuckDB file mtime must not change after sq slq --src.schema")
}

// TestSLQ_DuckDB_RenderSQL_DoesNotModifyMtime verifies that --render-sql
// (which still opens sources to resolve dialect) does so under the
// read-only context. Without the RO hint, render-sql produces only SQL
// output but mutates the file mtime as a side effect.
func TestSLQ_DuckDB_RenderSQL_DoesNotModifyMtime(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Duck)
	path := strings.TrimPrefix(src.Location, "duckdb://")

	statBefore, err := os.Stat(path)
	require.NoError(t, err)

	tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
	require.NoError(t, tr.Exec("slq", "--render-sql", src.Handle+" | .actor"))

	statAfter, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, statBefore.ModTime(), statAfter.ModTime(),
		"DuckDB file mtime must not change after sq slq --render-sql")
}

// TestCmdSLQ_Print_ReadOnly guards that the plain `sq <slq>` print path opens
// sources read-only. The DuckDB file is made read-only on disk (0444), so a
// read-write open would fail; read-only succeeds. Mirrors TestDiff_Data_ReadOnly.
func TestCmdSLQ_Print_ReadOnly(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.Duck)
	path := strings.TrimPrefix(src.Location, "duckdb://")

	require.NoError(t, os.Chmod(path, 0o444))
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) }) // let TempDir cleanup remove it

	tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
	require.NoError(t, tr.Exec("slq", src.Handle+".actor"),
		"sq <slq> must open the source read-only (no write lock)")
}

// TestCmdSLQ_Insert_FromReadOnlySource guards that an --insert can READ from a
// source that is read-only on disk (0444). The destination opens read-write;
// the source, having a different handle, must open read-only (gh #779 per-source
// mode via QueryContext.WriteHandle). A regression here forces the source RW and
// fails with "permission denied" on the 0444 DuckDB file. The destination is a
// (writable) SQLite source; the point is the read-only DuckDB source.
func TestCmdSLQ_Insert_FromReadOnlySource(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.Duck)
	dest := th.Source(sakila.SL3)

	srcPath := strings.TrimPrefix(src.Location, "duckdb://")
	require.NoError(t, os.Chmod(srcPath, 0o444))
	t.Cleanup(func() { _ = os.Chmod(srcPath, 0o644) })

	destTbl := "actor_ro_copy_" + stringz.Uniq8()
	tr := testrun.New(th.Context, t, nil).Hush().Add(*src, *dest)
	require.NoError(t, tr.Exec("slq", "--insert="+dest.Handle+"."+destTbl, src.Handle+".actor"),
		"insert from a read-only source must open the source read-only and succeed")
}
