package cli_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/drivers/csv"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
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
					require.Equal(t, source.RedactLocation(src.Location), srcMeta.Location)

					gotTableNames := srcMeta.TableNames()
					gotTableNames = lo.Intersect(gotTableNames, possibleTbls)

					for _, wantTblName := range tc.wantTbls {
						if src.Type == postgres.Type && wantTblName == sakila.TblFilmText {
							// Postgres sakila DB doesn't have film_text for some reason
							continue
						}
						require.Contains(t, gotTableNames, wantTblName)
					}

					t.Run("inspect_table", func(t *testing.T) {
						for _, tblName := range gotTableNames {
							tblName := tblName
							t.Run(tblName, func(t *testing.T) {
								// t.Parallel()
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
							fmt.Sprintf("--%s", flag.InspectOverview),
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
							fmt.Sprintf("--%s", flag.InspectDBProps),
							fmt.Sprintf("--%s", tf.format),
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
			require.Contains(t, output, source.RedactLocation(src.Location))

			for _, wantTblName := range tc.wantTbls {
				if src.Type == postgres.Type && wantTblName == "film_text" {
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
					fmt.Sprintf("--%s", flag.InspectOverview),
					fmt.Sprintf("--%s", format.Text),
				)
				require.NoError(t, err)
				output := tr2.Out.String()
				require.Contains(t, output, src.Type)
				require.Contains(t, output, src.Handle)
				require.Contains(t, output, source.RedactLocation(src.Location))
			})

			t.Run("inspect_dbprops", func(t *testing.T) {
				t.Logf("Test: sq inspect @src --dbprops")
				tr2 := testrun.New(th.Context, t, tr)
				err := tr2.Exec(
					"inspect",
					tc.handle,
					fmt.Sprintf("--%s", flag.InspectDBProps),
					fmt.Sprintf("--%s", format.Text),
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
	require.Equal(t, sqlite3.Type, md.Driver)
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
	require.Equal(t, csv.TypeCSV, md.Driver)
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
			wantType: csv.TypeCSV,
			wantTbls: []string{source.MonotableName},
		},
		{
			fpath:    proj.Abs(sakila.PathTSVActor),
			wantType: csv.TypeTSV,
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
