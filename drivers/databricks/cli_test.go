package databricks_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/drivers/databricks/internal/thriftfake"
)

// TestCLI exercises the driver through the sq commands, against the fake:
// "sq add", "sq ping", "sq inspect", SLQ and native SQL queries, and the
// commands that the read-only driver refuses.
func TestCLI(t *testing.T) {
	routes := append(metadataRoutes(),
		route{
			match: stmtContains("SELECT * FROM `actor` LIMIT 2 OFFSET 0"),
			res: result(thriftfake.Result{
				Columns: []thriftfake.Column{
					{Name: "actor_id", Type: thriftfake.TypeBigInt},
					{Name: "first_name", Type: thriftfake.TypeString},
				},
				Rows: [][]any{{int64(1), "PENELOPE"}, {int64(2), nil}},
			}),
		},
		route{match: stmtContains("CREATE TABLE"), res: result(thriftfake.Result{})},
		route{match: stmtContains("SHOW CATALOGS"), res: result(namesResult("catalog", "main", "samples"))},
		route{match: stmtContains("SHOW SCHEMAS"), res: result(namesResult("databaseName", "default", "nyctaxi"))},
	)
	srv := newFake(t, routes...)
	loc := fakeLocation(srv, "catalog=main&schema=sakila")

	ctx := context.Background()
	tr := testrun.New(ctx, t, nil)

	// "sq add" pings the source, and derives the handle's name from the
	// catalog param.
	require.NoError(t, tr.Exec("add", loc))
	require.Contains(t, tr.OutString(), "@main")
	requireNoSecret(t, tr)
	require.Contains(t, srv.Statements(), "select 1")

	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("driver", "ls"))
	require.Regexp(t, `(?m)^databricks\s`, tr.OutString())

	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("ping", "@main"))
	requireNoSecret(t, tr)

	tr = testrun.New(ctx, t, tr).Hush()
	require.NoError(t, tr.Exec("--json", "inspect", "@main"))
	requireNoSecret(t, tr)
	require.Equal(t, "databricks", tr.JQ(".driver"))
	require.Equal(t, "main", tr.JQ(".catalog"))
	require.Equal(t, "sakila", tr.JQ(".schema"))
	require.Len(t, tr.JQ(".tables").([]any), 4)

	tr = testrun.New(ctx, t, tr).Hush()
	require.NoError(t, tr.Exec("--csv", "--no-header", "@main.actor | .[:2]"))
	require.Equal(t, [][]string{{"1", "PENELOPE"}, {"2", ""}}, tr.BindCSV())
	requireNoSecret(t, tr)

	// Native SQL is not restricted, so DDL works. The rows affected count is
	// not reported by this driver.
	tr = testrun.New(ctx, t, tr).Hush()
	require.NoError(t, tr.Exec("--json", "sql", "CREATE TABLE sakila.t (id BIGINT)"))
	require.Equal(t, float64(-1), tr.JQ(".rows_affected"))
	require.Contains(t, srv.Statements(), "CREATE TABLE sakila.t (id BIGINT)")

	tr = testrun.New(ctx, t, tr).Hush()
	require.NoError(t, tr.Exec("--csv", "--no-header", "sql", "SELECT * FROM `actor` LIMIT 2 OFFSET 0"))
	require.Equal(t, [][]string{{"1", "PENELOPE"}, {"2", ""}}, tr.BindCSV())

	// --src.schema is validated against the warehouse, and becomes the
	// initial namespace of the session that runs the query.
	tr = testrun.New(ctx, t, tr).Hush()
	require.NoError(t, tr.Exec("--json", "inspect", "@main", "--src.schema", "samples.nyctaxi"))
	sessions := srv.Sessions()
	require.Equal(t, thriftfake.Namespace{Catalog: "samples", Schema: "nyctaxi"}, sessions[len(sessions)-1])

	tr = testrun.New(ctx, t, tr)
	err := tr.Exec("inspect", "@main", "--src.schema", "nope.nyctaxi")
	require.Error(t, err)
	require.ErrorContains(t, err, "catalog {nope} doesn't exist")

	// Inserting into a Databricks source is not supported. The insert fails
	// at the first step, which is to open a transaction on the destination,
	// as Databricks has none, so it must not have sent any DDL or DML.
	tr = testrun.New(ctx, t, tr)
	before := len(srv.Statements())
	err = tr.Exec("--insert", "@main.actor_copy", "@main.actor | .[:2]")
	require.Error(t, err)
	require.ErrorContains(t, err, "@main.actor_copy")
	for _, stmt := range srv.Statements()[before:] {
		require.NotContains(t, stmt, "CREATE")
		require.NotContains(t, stmt, "INSERT")
	}

	// The write commands report that the driver is read-only.
	tr = testrun.New(ctx, t, tr)
	err = tr.Exec("tbl", "truncate", "@main.actor")
	require.Error(t, err)
	require.ErrorContains(t, err, "read-only")
	requireNoSecret(t, tr)
}

// requireNoSecret requires that the output of tr doesn't contain the
// access token.
func requireNoSecret(t *testing.T, tr *testrun.TestRun) {
	t.Helper()
	require.NotContains(t, tr.Out.String(), fakePAT)
	require.NotContains(t, tr.ErrOut.String(), fakePAT)
}
