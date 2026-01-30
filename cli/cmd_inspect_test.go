package cli_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// TestCmdInspect_json_yaml tests "sq inspect" for
// the JSON and YAML formats.
func TestCmdInspect_json_yaml(t *testing.T) { //nolint:tparallel
	tu.SkipShort(t, true)

	possibleTbls := append(sakila.AllTbls(), source.MonotableName)
	testCases := []struct {
		handle   string
		wantTbls []string
	}{
		{sakila.CSVActor, []string{source.MonotableName}},
		{sakila.TSVActor, []string{source.MonotableName}},
		{sakila.XLSX, sakila.AllTbls()},
		{sakila.SL3, sakila.AllTbls()},
		{sakila.Pg, lo.Without(sakila.AllTbls(), sakila.TblFilmText)}, // pg doesn't have film_text
		{sakila.My, sakila.AllTbls()},
		{sakila.MS, sakila.AllTbls()},
	}

	testFormats := []struct {
		format      format.Format
		unmarshalFn func(data []byte, v any) error
	}{
		{format.JSON, json.Unmarshal},
		{format.YAML, ioz.UnmarshallYAML},
	}

	for _, tf := range testFormats {
		tf := tf
		t.Run(tf.format.String(), func(t *testing.T) {
			t.Parallel()

			for _, tc := range testCases {
				tc := tc

				t.Run(tc.handle, func(t *testing.T) {
					t.Parallel()
					tu.SkipWindowsIf(t, tc.handle == sakila.XLSX, "XLSX too slow on windows workflow")

					th := testh.New(t)
					src := th.Source(tc.handle)

					tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
					err := tr.Exec("inspect", fmt.Sprintf("--%s", tf.format))
					require.NoError(t, err)

					srcMeta := &metadata.Source{}
					require.NoError(t, tf.unmarshalFn(tr.Out.Bytes(), srcMeta))
					require.Equal(t, src.Type, srcMeta.Driver)
					require.Equal(t, src.Handle, srcMeta.Handle)
					require.Equal(t, location.Redact(src.Location), srcMeta.Location)

					gotTableNames := srcMeta.TableNames()
					gotTableNames = lo.Intersect(gotTableNames, possibleTbls)

					for _, wantTblName := range tc.wantTbls {
						if src.Type == drivertype.Pg && wantTblName == sakila.TblFilmText {
							// Postgres sakila DB doesn't have film_text for some reason
							continue
						}
						require.Contains(t, gotTableNames, wantTblName)
					}

					t.Run("inspect_table", func(t *testing.T) {
						for _, tblName := range gotTableNames {
							tblName := tblName
							t.Run(tblName, func(t *testing.T) {
								tu.SkipShort(t, true)
								tr2 := testrun.New(lg.NewContext(th.Context, lgt.New(t)), t, tr)
								err := tr2.Exec("inspect", "."+tblName, fmt.Sprintf("--%s", tf.format))
								require.NoError(t, err)
								tblMeta := &metadata.Table{}
								require.NoError(t, tf.unmarshalFn(tr2.Out.Bytes(), tblMeta))
								require.Equal(t, tblName, tblMeta.Name)
								require.True(t, len(tblMeta.Columns) > 0)
							})
						}
					})

					t.Run("inspect_overview", func(t *testing.T) {
						t.Logf("Test: sq inspect @src --overview")
						tr2 := testrun.New(lg.NewContext(th.Context, lgt.New(t)), t, tr)
						err := tr2.Exec(
							"inspect",
							tc.handle,
							"--"+flag.InspectOverview,
							fmt.Sprintf("--%s", tf.format),
						)
						require.NoError(t, err)

						srcMeta := &metadata.Source{}
						require.NoError(t, tf.unmarshalFn(tr2.Out.Bytes(), srcMeta))
						require.Equal(t, src.Type, srcMeta.Driver)
						require.Equal(t, src.Handle, srcMeta.Handle)
						require.Nil(t, srcMeta.Tables)
						require.Zero(t, srcMeta.TableCount)
						require.Zero(t, srcMeta.ViewCount)
						require.NotEmpty(t, srcMeta.Name)
						require.NotEmpty(t, srcMeta.Schema)
						require.NotEmpty(t, srcMeta.FQName)
						require.NotEmpty(t, srcMeta.DBDriver)
						require.NotEmpty(t, srcMeta.DBProduct)
						require.NotEmpty(t, srcMeta.DBVersion)
						require.NotZero(t, srcMeta.Size)
					})

					t.Run("inspect_dbprops", func(t *testing.T) {
						t.Logf("Test: sq inspect @src --dbprops")
						tr2 := testrun.New(lg.NewContext(th.Context, lgt.New(t)), t, tr)
						err := tr2.Exec(
							"inspect",
							tc.handle,
							"--"+flag.InspectDBProps,
							"--"+tf.format.String(),
						)
						require.NoError(t, err)

						props := map[string]any{}
						require.NoError(t, tf.unmarshalFn(tr2.Out.Bytes(), &props))
						require.NotEmpty(t, props)
					})
				})
			}
		})
	}
}

// TestCmdInspect_text tests "sq inspect" for
// the text format.
func TestCmdInspect_text(t *testing.T) { //nolint:tparallel
	testCases := []struct {
		handle   string
		wantTbls []string
	}{
		{sakila.CSVActor, []string{source.MonotableName}},
		{sakila.TSVActor, []string{source.MonotableName}},
		{sakila.XLSX, sakila.AllTbls()},
		{sakila.SL3, sakila.AllTbls()},
		{sakila.Pg, lo.Without(sakila.AllTbls(), sakila.TblFilmText)}, // pg doesn't have film_text
		{sakila.My, sakila.AllTbls()},
		{sakila.MS, sakila.AllTbls()},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.handle, func(t *testing.T) {
			t.Parallel()

			tu.SkipWindowsIf(t, tc.handle == sakila.XLSX, "XLSX too slow on windows workflow")

			th := testh.New(t)
			src := th.Source(tc.handle)

			tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
			err := tr.Exec("inspect", fmt.Sprintf("--%s", format.Text))
			require.NoError(t, err)

			output := tr.Out.String()
			require.Contains(t, output, src.Type)
			require.Contains(t, output, src.Handle)
			require.Contains(t, output, location.Redact(src.Location))

			for _, wantTblName := range tc.wantTbls {
				if src.Type == drivertype.Pg && wantTblName == "film_text" {
					// Postgres sakila DB doesn't have film_text for some reason
					continue
				}
				require.Contains(t, output, wantTblName)
			}

			t.Run("inspect_table", func(t *testing.T) {
				for _, tblName := range tc.wantTbls {
					tblName := tblName
					t.Run(tblName, func(t *testing.T) {
						tu.SkipShort(t, true)
						t.Logf("Test: sq inspect .tbl")
						tr2 := testrun.New(th.Context, t, tr)
						err := tr2.Exec("inspect", "."+tblName, fmt.Sprintf("--%s", format.Text))
						require.NoError(t, err)

						output := tr2.Out.String()
						require.Contains(t, output, tblName)
					})
				}
			})

			t.Run("inspect_overview", func(t *testing.T) {
				t.Logf("Test: sq inspect @src --overview")
				tr2 := testrun.New(th.Context, t, tr)
				err := tr2.Exec(
					"inspect",
					tc.handle,
					"--"+flag.InspectOverview,
					"--"+format.Text.String(),
				)
				require.NoError(t, err)
				output := tr2.Out.String()
				require.Contains(t, output, src.Type)
				require.Contains(t, output, src.Handle)
				require.Contains(t, output, location.Redact(src.Location))
			})

			t.Run("inspect_dbprops", func(t *testing.T) {
				t.Logf("Test: sq inspect @src --dbprops")
				tr2 := testrun.New(th.Context, t, tr)
				err := tr2.Exec(
					"inspect",
					tc.handle,
					"--"+flag.InspectDBProps,
					"--"+format.Text.String(),
				)
				require.NoError(t, err)
				output := tr2.Out.String()
				require.NotEmpty(t, output)
			})
		})
	}
}

func TestCmdInspect_smoke(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	tr := testrun.New(th.Context, t, nil)
	err := tr.Exec("inspect")
	require.Error(t, err, "should fail because no active src")

	tr = testrun.New(th.Context, t, nil)
	tr.Add(*src) // now have an active src

	err = tr.Exec("inspect", "--json")
	require.NoError(t, err, "should pass because there is an active src")

	md := &metadata.Source{}
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), md))
	require.Equal(t, drivertype.SQLite, md.Driver)
	require.Equal(t, sakila.SL3, md.Handle)
	require.Equal(t, src.RedactedLocation(), md.Location)
	require.Equal(t, sakila.AllTblsViews(), md.TableNames())

	// Try one more source for good measure
	tr = testrun.New(th.Context, t, nil)
	src = th.Source(sakila.CSVActor)
	tr.Add(*src)

	err = tr.Exec("inspect", "--json", src.Handle)
	require.NoError(t, err)

	md = &metadata.Source{}
	require.NoError(t, json.Unmarshal(tr.Out.Bytes(), md))
	require.Equal(t, drivertype.CSV, md.Driver)
	require.Equal(t, sakila.CSVActor, md.Handle)
	require.Equal(t, src.Location, md.Location)
	require.Equal(t, []string{source.MonotableName}, md.TableNames())
}

func TestCmdInspect_stdin(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		fpath    string
		wantErr  bool
		wantType drivertype.Type
		wantTbls []string
	}{
		{
			fpath:    proj.Abs(sakila.PathCSVActor),
			wantType: drivertype.CSV,
			wantTbls: []string{source.MonotableName},
		},
		{
			fpath:    proj.Abs(sakila.PathTSVActor),
			wantType: drivertype.TSV,
			wantTbls: []string{source.MonotableName},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tu.Name(tc.fpath), func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			f, err := os.Open(tc.fpath) // No need to close f
			require.NoError(t, err)

			tr := testrun.New(ctx, t, nil)
			tr.Run.Stdin = f

			err = tr.Exec("inspect", "--json")
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err, "should read from stdin")

			md := &metadata.Source{}
			require.NoError(t, json.Unmarshal(tr.Out.Bytes(), md))
			require.Equal(t, tc.wantType, md.Driver)
			require.Equal(t, source.StdinHandle, md.Handle)
			require.Equal(t, source.StdinHandle, md.Location)
			require.Equal(t, tc.wantTbls, md.TableNames())
		})
	}
}

func TestCmdInspect_mode_schemata(t *testing.T) {
	active := lo.ToPtr(true)

	type schema struct {
		Name    string `json:"schema" yaml:"schema"`
		Catalog string `json:"catalog" yaml:"catalog"`
		Owner   string `json:"owner,omitempty" yaml:"owner,omitempty"`
		Active  *bool  `json:"active" yaml:"active"`
	}

	testCases := []struct {
		handle       string
		wantSchemata []schema
	}{
		{
			handle: sakila.SL3,
			wantSchemata: []schema{
				{Name: "main", Catalog: "default", Active: active},
			},
		},
		{
			handle: sakila.Pg,
			wantSchemata: []schema{
				{Name: "information_schema", Catalog: "sakila", Owner: "sakila"},
				{Name: "pg_catalog", Catalog: "sakila", Owner: "sakila"},
				{Name: "public", Catalog: "sakila", Owner: "sakila", Active: active},
			},
		},
		{
			handle: sakila.MS,
			wantSchemata: []schema{
				{Name: "INFORMATION_SCHEMA", Catalog: "sakila", Owner: "INFORMATION_SCHEMA"},
				{Name: "dbo", Catalog: "sakila", Owner: "dbo", Active: active},
				{Name: "sys", Catalog: "sakila", Owner: "sys"},
			},
		},
		{
			handle: sakila.My,
			wantSchemata: []schema{
				{Name: "information_schema", Catalog: "def", Owner: ""},
				{Name: "mysql", Catalog: "def", Owner: ""},
				{Name: "sakila", Catalog: "def", Owner: "", Active: active},
				{Name: "sys", Catalog: "def", Owner: ""},
			},
		},
	}

	for _, fm := range []format.Format{format.JSON, format.YAML, format.Text} {
		fm := fm
		t.Run(fm.String(), func(t *testing.T) {
			for _, tc := range testCases {
				tc := tc

				t.Run(tc.handle, func(t *testing.T) {
					th := testh.New(t)
					src := th.Source(tc.handle)

					tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
					err := tr.Exec("inspect", "--"+flag.InspectSchemata, "--"+fm.String())
					require.NoError(t, err)
					var gotSchemata []schema

					switch fm { //nolint:exhaustive
					case format.JSON:
						tr.Bind(&gotSchemata)
					case format.YAML:
						tr.BindYAML(&gotSchemata)
					case format.Text:
						t.Logf("\n%s", tr.OutString())
						// Return early because we can't be bothered to parse text output
						return
					}

					for i, s := range tc.wantSchemata {
						require.Contains(t, gotSchemata, s, "wantSchemata[%d]", i)
					}
				})
			}
		})
	}
}

func TestCmdInspect_mode_catalogs(t *testing.T) {
	active := lo.ToPtr(true)
	type catalog struct {
		Catalog string `json:"catalog" yaml:"catalog"`
		Active  *bool  `json:"active,omitempty" yaml:"active,omitempty"`
	}

	testCases := []struct {
		handle       string
		wantCatalogs []catalog
	}{
		// Note that SQLite doesn't support catalogs
		{
			handle: sakila.Pg,
			wantCatalogs: []catalog{
				{Catalog: "postgres"},
				{Catalog: "sakila", Active: active},
			},
		},
		{
			handle: sakila.MS,
			wantCatalogs: []catalog{
				{Catalog: "master"},
				{Catalog: "model"},
				{Catalog: "msdb"},
				{Catalog: "sakila", Active: active},
				{Catalog: "tempdb"},
			},
		},
		{
			handle: sakila.My,
			wantCatalogs: []catalog{
				{Catalog: "def", Active: active},
			},
		},
	}

	for _, fm := range []format.Format{format.JSON, format.YAML, format.Text} {
		fm := fm
		t.Run(fm.String(), func(t *testing.T) {
			for _, tc := range testCases {
				tc := tc

				t.Run(tc.handle, func(t *testing.T) {
					th := testh.New(t)
					src := th.Source(tc.handle)

					tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
					err := tr.Exec("inspect", "--"+flag.InspectCatalogs, "--"+fm.String())
					require.NoError(t, err)
					var gotCatalogs []catalog

					switch fm { //nolint:exhaustive
					case format.JSON:
						tr.Bind(&gotCatalogs)
					case format.YAML:
						tr.BindYAML(&gotCatalogs)
					case format.Text:
						t.Logf("\n%s", tr.OutString())
						// Return early because we can't be bothered to parse text output
						return
					}

					for i, c := range tc.wantCatalogs {
						require.Contains(t, gotCatalogs, c, "wantCatalogs[%d]", i)
					}
				})
			}
		})
	}
}

// TestCmdInspect_NumericSchema tests "sq inspect --src.schema" with numeric
// and numeric-prefixed schema names. This validates the grammar fix for issue #470.
// See: https://github.com/neilotoole/sq/issues/470
func TestCmdInspect_NumericSchema(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	testCases := []struct {
		name   string
		schema string
	}{
		{"pure_numeric", "98765"},
		{"numeric_prefixed", "123test"},
		{"numeric_with_underscore", "456_inspect"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			ctx := th.Context
			src := th.Source(sakila.Pg)

			// Create a unique schema name for this test.
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
			tblName := "inspect_test_tbl"
			_, err = db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE %q.%q (id serial PRIMARY KEY, name text)`,
				schemaName, tblName))
			require.NoError(t, err)

			// Insert some test data.
			_, err = db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %q.%q (name) VALUES ('row1'), ('row2')`,
				schemaName, tblName))
			require.NoError(t, err)

			// Run sq inspect with --src.schema pointing to the numeric schema.
			tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
			err = tr.Exec(
				"inspect",
				"--src.schema", schemaName,
				"--json",
			)
			require.NoError(t, err, "sq inspect --src.schema %q should succeed", schemaName)

			// Parse the output.
			srcMeta := &metadata.Source{}
			require.NoError(t, json.Unmarshal(tr.Out.Bytes(), srcMeta))

			// Verify the schema matches our numeric schema.
			require.Equal(t, schemaName, srcMeta.Schema,
				"inspect output should show the numeric schema")

			// Verify our test table appears in the output.
			tblNames := srcMeta.TableNames()
			require.Contains(t, tblNames, tblName,
				"inspect output should contain our test table")

			// Also test inspect of the specific table in the numeric schema.
			tr2 := testrun.New(th.Context, t, nil).Hush().Add(*src)
			err = tr2.Exec(
				"inspect",
				"."+tblName,
				"--src.schema", schemaName,
				"--json",
			)
			require.NoError(t, err, "sq inspect .%s --src.schema %q should succeed", tblName, schemaName)

			tblMeta := &metadata.Table{}
			require.NoError(t, json.Unmarshal(tr2.Out.Bytes(), tblMeta))
			require.Equal(t, tblName, tblMeta.Name,
				"table inspect should return correct table name")
			require.Equal(t, int64(2), tblMeta.RowCount,
				"table inspect should show correct row count")
		})
	}
}

// TestCmdInspect_NumericCatalogSchema tests "sq inspect --src.schema CATALOG.SCHEMA"
// with numeric and numeric-prefixed identifiers in both catalog and schema positions.
// This validates the full CATALOG.SCHEMA parsing flow for issue #470.
// See: https://github.com/neilotoole/sq/issues/470
func TestCmdInspect_NumericCatalogSchema(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	// Test cases with numeric catalog.schema combinations.
	// Note: For PostgreSQL, "catalog" is the database name. Creating databases
	// with numeric names requires special permissions, so we use the existing
	// sakila database as the catalog and test numeric schemas within it.
	testCases := []struct {
		name   string
		schema string
	}{
		{"pure_numeric", "77777"},
		{"numeric_prefixed", "888xyz"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			ctx := th.Context
			src := th.Source(sakila.Pg)

			// Create a unique schema name for this test.
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
			tblName := "catschema_test_tbl"
			_, err = db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE %q.%q (id serial PRIMARY KEY, val text)`,
				schemaName, tblName))
			require.NoError(t, err)

			// Insert some test data.
			_, err = db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %q.%q (val) VALUES ('a'), ('b'), ('c')`,
				schemaName, tblName))
			require.NoError(t, err)

			// The catalog for PostgreSQL is the database name.
			// For the sakila test database, this is "sakila".
			catalogSchema := "sakila." + schemaName

			// Run sq inspect with --src.schema in CATALOG.SCHEMA format.
			tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
			err = tr.Exec(
				"inspect",
				"--src.schema", catalogSchema,
				"--json",
			)
			require.NoError(t, err, "sq inspect --src.schema %q should succeed", catalogSchema)

			// Parse the output.
			srcMeta := &metadata.Source{}
			require.NoError(t, json.Unmarshal(tr.Out.Bytes(), srcMeta))

			// Verify the schema matches our numeric schema.
			require.Equal(t, schemaName, srcMeta.Schema,
				"inspect output should show the numeric schema")

			// Verify our test table appears in the output.
			tblNames := srcMeta.TableNames()
			require.Contains(t, tblNames, tblName,
				"inspect output should contain our test table")

			// Also test with --src.schema using just the schema name (no catalog prefix)
			// to confirm both formats work.
			tr2 := testrun.New(th.Context, t, nil).Hush().Add(*src)
			err = tr2.Exec(
				"inspect",
				"."+tblName,
				"--src.schema", schemaName, // schema only, no catalog
				"--json",
			)
			require.NoError(t, err, "sq inspect .%s --src.schema %q should succeed", tblName, schemaName)

			tblMeta := &metadata.Table{}
			require.NoError(t, json.Unmarshal(tr2.Out.Bytes(), tblMeta))
			require.Equal(t, tblName, tblMeta.Name,
				"table inspect should return correct table name")
			require.Equal(t, int64(3), tblMeta.RowCount,
				"table inspect should show correct row count")
		})
	}
}
