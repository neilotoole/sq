package source_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/drivers/csv"
	"github.com/neilotoole/sq/drivers/mysql"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/drivers/sqlserver"
	"github.com/neilotoole/sq/drivers/xlsx"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/testsrc"
	"github.com/neilotoole/sq/testh/tu"
)

func TestFiles_Type(t *testing.T) {
	testCases := []struct {
		loc      string
		wantType drivertype.Type
		wantErr  bool
	}{
		{loc: proj.Expand("sqlite3://${SQ_ROOT}/drivers/sqlite3/testdata/sakila.db"), wantType: sqlite3.Type},
		{loc: proj.Abs(sakila.PathSL3), wantType: sqlite3.Type},
		{loc: proj.Abs("drivers/sqlite3/testdata/sakila_db"), wantType: sqlite3.Type},
		{loc: "sqlserver://sakila:p_ssW0rd@localhost?database=sakila", wantType: sqlserver.Type},
		{loc: "postgres://sakila:p_ssW0rd@localhost/sakila?sslmode=disable", wantType: postgres.Type},
		{loc: "mysql://sakila:p_ssW0rd@localhost/sakila", wantType: mysql.Type},
		{loc: proj.Abs(testsrc.PathXLSXTestHeader), wantType: xlsx.Type},
		{loc: proj.Abs("drivers/xlsx/testdata/test_header_xlsx"), wantType: xlsx.Type},
		{loc: sakila.URLSubsetXLSX, wantType: xlsx.Type},
		{loc: proj.Abs(sakila.PathCSVActor), wantType: csv.TypeCSV},
		{loc: proj.Abs("drivers/csv/testdata/person_csv"), wantType: csv.TypeCSV},
		{loc: sakila.URLActorCSV, wantType: csv.TypeCSV},
		{loc: proj.Abs("drivers/csv/testdata/person_tsv"), wantType: csv.TypeTSV},
		{loc: proj.Abs(sakila.PathTSVActor), wantType: csv.TypeTSV},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tu.Name(source.RedactLocation(tc.loc)), func(t *testing.T) {
			ctx := lg.NewContext(context.Background(), lgt.New(t))

			fs, err := source.NewFiles(ctx, nil, tu.TempDir(t), tu.CacheDir(t), true)
			require.NoError(t, err)
			fs.AddDriverDetectors(testh.DriverDetectors()...)

			gotType, gotErr := fs.DriverType(context.Background(), "@test_"+stringz.Uniq8(), tc.loc)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tc.wantType, gotType)
		})
	}
}

func TestFiles_DetectType(t *testing.T) {
	testCases := []struct {
		loc      string
		wantType drivertype.Type
		wantOK   bool
		wantErr  bool
	}{
		{loc: proj.Abs(sakila.PathSL3), wantType: sqlite3.Type, wantOK: true},
		{loc: proj.Abs("drivers/sqlite3/testdata/sakila_db"), wantType: sqlite3.Type, wantOK: true},
		{loc: proj.Abs(testsrc.PathXLSXTestHeader), wantType: xlsx.Type, wantOK: true},
		{loc: proj.Abs("drivers/xlsx/testdata/test_header_xlsx"), wantType: xlsx.Type, wantOK: true},
		{loc: proj.Abs("drivers/xlsx/testdata/test_noheader.xlsx"), wantType: xlsx.Type, wantOK: true},
		{loc: proj.Abs("drivers/csv/testdata/person.csv"), wantType: csv.TypeCSV, wantOK: true},
		{loc: proj.Abs("drivers/csv/testdata/person_noheader.csv"), wantType: csv.TypeCSV, wantOK: true},
		{loc: proj.Abs("drivers/csv/testdata/person_csv"), wantType: csv.TypeCSV, wantOK: true},
		{loc: proj.Abs("drivers/csv/testdata/person.tsv"), wantType: csv.TypeTSV, wantOK: true},
		{loc: proj.Abs("drivers/csv/testdata/person_noheader.tsv"), wantType: csv.TypeTSV, wantOK: true},
		{loc: proj.Abs("drivers/csv/testdata/person_tsv"), wantType: csv.TypeTSV, wantOK: true},
		{loc: proj.Abs("README.md"), wantType: drivertype.None, wantOK: false},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(filepath.Base(tc.loc), func(t *testing.T) {
			ctx := lg.NewContext(context.Background(), lgt.New(t))
			fs, err := source.NewFiles(ctx, nil, tu.TempDir(t), tu.CacheDir(t), true)
			require.NoError(t, err)
			fs.AddDriverDetectors(testh.DriverDetectors()...)

			typ, ok, err := source.FilesDetectTypeFn(fs, ctx, tc.loc)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.wantOK, ok)
			require.Equal(t, tc.wantType, typ)
		})
	}
}

func TestDetectMagicNumber(t *testing.T) {
	testCases := []struct {
		loc       string
		wantType  drivertype.Type
		wantScore float32
		wantErr   bool
	}{
		{loc: proj.Abs(sakila.PathSL3), wantType: sqlite3.Type, wantScore: 1.0},
		{loc: proj.Abs("drivers/sqlite3/testdata/sakila_db"), wantType: sqlite3.Type, wantScore: 1.0},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(filepath.Base(tc.loc), func(t *testing.T) {
			rFn := func(ctx context.Context) (io.ReadCloser, error) { return os.Open(tc.loc) }

			ctx := lg.NewContext(context.Background(), lgt.New(t))

			typ, score, err := source.DetectMagicNumber(ctx, rFn)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantType, typ)
			require.Equal(t, tc.wantScore, score)
		})
	}
}

func TestFiles_NewReader(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	fpath := sakila.PathCSVActor
	wantBytes := proj.ReadFile(fpath)

	src := &source.Source{
		Handle:   "@test_" + stringz.Uniq8(),
		Type:     csv.TypeCSV,
		Location: proj.Abs(fpath),
	}

	fs, err := source.NewFiles(ctx, nil, tu.TempDir(t), tu.CacheDir(t), true)
	require.NoError(t, err)

	g := &errgroup.Group{}

	for i := 0; i < 1000; i++ {
		g.Go(func() error {
			r, gErr := fs.Open(ctx, src)
			require.NoError(t, gErr)

			b, gErr := io.ReadAll(r)
			require.NoError(t, gErr)

			require.Equal(t, wantBytes, b)
			return nil
		})
	}

	err = g.Wait()
	require.NoError(t, err)
}

func TestFiles_Stdin(t *testing.T) {
	testCases := []struct {
		fpath    string
		wantType drivertype.Type
		wantErr  bool
	}{
		{fpath: proj.Abs(sakila.PathCSVActor), wantType: csv.TypeCSV},
		{fpath: proj.Abs(sakila.PathTSVActor), wantType: csv.TypeTSV},
		{fpath: proj.Abs(sakila.PathXLSX), wantType: xlsx.Type},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tu.Name(tc.fpath), func(t *testing.T) {
			th := testh.New(t)
			fs := th.Files()

			f, err := os.Open(tc.fpath)
			require.NoError(t, err)

			err = fs.AddStdin(th.Context, f) // f is closed by AddStdin
			require.NoError(t, err)

			typ, err := fs.DetectStdinType(th.Context)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.Equal(t, tc.wantType, typ)
		})
	}
}

func TestFiles_Stdin_ErrorWrongOrder(t *testing.T) {
	th := testh.New(t)
	fs := th.Files()

	typ, err := fs.DetectStdinType(th.Context)
	require.Error(t, err, "should error because AddStdin not yet invoked")
	require.Equal(t, drivertype.None, typ)

	f, err := os.Open(proj.Abs(sakila.PathCSVActor))
	require.NoError(t, err)

	require.NoError(t, fs.AddStdin(th.Context, f)) // AddStdin closes f
	typ, err = fs.DetectStdinType(th.Context)
	require.NoError(t, err)
	require.Equal(t, csv.TypeCSV, typ)
}

func TestFiles_Size(t *testing.T) {
	f, err := os.Open(proj.Abs(sakila.PathCSVActor))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, f.Close()) })

	fi, err := os.Stat(f.Name())
	require.NoError(t, err)
	wantSize := fi.Size()

	th := testh.New(t)
	fs := th.Files()

	gotSize, err := fs.Filesize(th.Context, &source.Source{
		Handle:   stringz.UniqSuffix("@h"),
		Location: f.Name(),
	})
	require.NoError(t, err)
	require.Equal(t, wantSize, gotSize)

	f2, err := os.Open(proj.Abs(sakila.PathCSVActor))
	require.NoError(t, err)
	// Verify that this works with @stdin as well
	require.NoError(t, fs.AddStdin(th.Context, f2))

	gotSize2, err := fs.Filesize(th.Context, &source.Source{
		Handle:   "@stdin",
		Location: "@stdin",
	})
	require.NoError(t, err)
	require.Equal(t, wantSize, gotSize2)
}
