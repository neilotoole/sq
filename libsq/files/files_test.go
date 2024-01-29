package files_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/testsrc"
	"github.com/neilotoole/sq/testh/tu"
)

func TestFiles_DetectType(t *testing.T) {
	testCases := []struct {
		loc      string
		wantType drivertype.Type
		wantErr  bool
	}{
		{loc: proj.Abs(sakila.PathSL3), wantType: drivertype.SQLite},
		{loc: proj.Abs("drivers/sqlite3/testdata/sakila_db"), wantType: drivertype.SQLite},
		{loc: proj.Abs(testsrc.PathXLSXTestHeader), wantType: drivertype.XLSX},
		{loc: proj.Abs("drivers/xlsx/testdata/test_header_xlsx"), wantType: drivertype.XLSX},
		{loc: proj.Abs("drivers/xlsx/testdata/test_noheader.xlsx"), wantType: drivertype.XLSX},
		{loc: proj.Abs("drivers/csv/testdata/person.csv"), wantType: drivertype.CSV},
		{loc: proj.Abs("drivers/csv/testdata/person_noheader.csv"), wantType: drivertype.CSV},
		{loc: proj.Abs("drivers/csv/testdata/person_csv"), wantType: drivertype.CSV},
		{loc: proj.Abs("drivers/csv/testdata/person.tsv"), wantType: drivertype.TSV},
		{loc: proj.Abs("drivers/csv/testdata/person_noheader.tsv"), wantType: drivertype.TSV},
		{loc: proj.Abs("drivers/csv/testdata/person_tsv"), wantType: drivertype.TSV},
		{loc: proj.Abs("drivers/csv/testdata/person_tsv"), wantType: drivertype.TSV},
		{loc: proj.Abs("drivers/json/testdata/actor.json"), wantType: drivertype.JSON},
		{loc: proj.Abs("drivers/json/testdata/actor.jsona"), wantType: drivertype.JSONA},
		{loc: proj.Abs("drivers/json/testdata/actor.jsonl"), wantType: drivertype.JSONL},
		{loc: proj.Abs("README.md"), wantType: drivertype.None, wantErr: true},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(filepath.Base(tc.loc), func(t *testing.T) {
			ctx := lg.NewContext(context.Background(), lgt.New(t))
			fs, err := files.New(ctx, nil, testh.TempLockFunc(t), tu.TempDir(t, true), tu.CacheDir(t, true))
			require.NoError(t, err)
			fs.AddDriverDetectors(testh.DriverDetectors()...)

			typ, err := fs.DetectType(ctx, "@test_"+stringz.Uniq8(), tc.loc)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.wantType, typ)
		})
	}
}

func TestFiles_DriverType(t *testing.T) {
	testCases := []struct {
		loc      string
		wantType drivertype.Type
		wantErr  bool
	}{
		{loc: proj.Expand("sqlite3://${SQ_ROOT}/drivers/sqlite3/testdata/sakila.db"), wantType: drivertype.SQLite},
		{loc: proj.Abs(sakila.PathSL3), wantType: drivertype.SQLite},
		{loc: proj.Abs("drivers/sqlite3/testdata/sakila_db"), wantType: drivertype.SQLite},
		{loc: "sqlserver://sakila:p_ssW0rd@localhost?database=sakila", wantType: drivertype.MSSQL},
		{loc: "postgres://sakila:p_ssW0rd@localhost/sakila", wantType: drivertype.Pg},
		{loc: "mysql://sakila:p_ssW0rd@localhost/sakila", wantType: drivertype.MySQL},
		{loc: proj.Abs(testsrc.PathXLSXTestHeader), wantType: drivertype.XLSX},
		{loc: proj.Abs("drivers/xlsx/testdata/test_header_xlsx"), wantType: drivertype.XLSX},
		{loc: sakila.ExcelSubsetURL, wantType: drivertype.XLSX},
		{loc: proj.Abs(sakila.PathCSVActor), wantType: drivertype.CSV},
		{loc: proj.Abs("drivers/csv/testdata/person_csv"), wantType: drivertype.CSV},
		{loc: sakila.ActorCSVURL, wantType: drivertype.CSV},
		{loc: proj.Abs(sakila.PathTSVActor), wantType: drivertype.TSV},
		{loc: proj.Abs("drivers/csv/testdata/person_tsv"), wantType: drivertype.TSV},
		{loc: proj.Abs("drivers/json/testdata/actor.json"), wantType: drivertype.JSON},
		{loc: proj.Abs("drivers/json/testdata/actor.jsona"), wantType: drivertype.JSONA},
		{loc: proj.Abs("drivers/json/testdata/actor.jsonl"), wantType: drivertype.JSONL},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tu.Name(location.Redact(tc.loc)), func(t *testing.T) {
			ctx := lg.NewContext(context.Background(), lgt.New(t))

			fs, err := files.New(ctx, nil, testh.TempLockFunc(t), tu.TempDir(t, true), tu.CacheDir(t, true))
			require.NoError(t, err)
			fs.AddDriverDetectors(testh.DriverDetectors()...)

			gotType, gotErr := fs.DetectType(context.Background(), "@test_"+stringz.Uniq8(), tc.loc)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tc.wantType, gotType)
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
		{loc: proj.Abs(sakila.PathSL3), wantType: drivertype.SQLite, wantScore: 1.0},
		{loc: proj.Abs("drivers/sqlite3/testdata/sakila_db"), wantType: drivertype.SQLite, wantScore: 1.0},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(filepath.Base(tc.loc), func(t *testing.T) {
			rFn := func(ctx context.Context) (io.ReadCloser, error) { return os.Open(tc.loc) }

			ctx := lg.NewContext(context.Background(), lgt.New(t))

			typ, score, err := files.DetectMagicNumber(ctx, rFn)
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
		Type:     drivertype.CSV,
		Location: proj.Abs(fpath),
	}

	fs, err := files.New(ctx, nil, testh.TempLockFunc(t), tu.TempDir(t, true), tu.CacheDir(t, true))
	require.NoError(t, err)

	g := &errgroup.Group{}

	for i := 0; i < 1000; i++ {
		g.Go(func() error {
			r, gErr := fs.NewReader(ctx, src, false)
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
		{fpath: proj.Abs(sakila.PathCSVActor), wantType: drivertype.CSV},
		{fpath: proj.Abs(sakila.PathTSVActor), wantType: drivertype.TSV},
		{fpath: proj.Abs(sakila.PathXLSX), wantType: drivertype.XLSX},
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
	require.Equal(t, drivertype.CSV, typ)
}

func TestFiles_Filesize(t *testing.T) {
	f, err := os.Open(proj.Abs(sakila.PathCSVActor))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, f.Close()) })

	fi, err := os.Stat(f.Name())
	require.NoError(t, err)
	wantSize := fi.Size()

	th := testh.New(t)
	var cancelFn context.CancelFunc
	th.Context, cancelFn = context.WithTimeout(th.Context, time.Second)
	defer cancelFn()
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

	stdinSrc := &source.Source{Handle: "@stdin", Location: "@stdin"}

	// Files.Filesize will block until the stream is fully read.
	r, err := fs.NewReader(th.Context, stdinSrc, false)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, r)
	require.NoError(t, err)

	gotSize2, err := fs.Filesize(th.Context, stdinSrc)
	require.NoError(t, err)
	require.Equal(t, wantSize, gotSize2)
}
