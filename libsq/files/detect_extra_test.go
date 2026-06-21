package files_test

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
)

// erroringDetector is a TypeDetectFunc that always returns an error, used to
// drive the errgroup error path in detectType.
func erroringDetector(_ context.Context, _ files.NewReaderFunc) (drivertype.Type, float32, error) {
	return drivertype.None, 0, errz.New("boom")
}

// errReader returns wantErr after returning some bytes, to drive the
// io.ReadFull error branch of DetectMagicNumber.
type errReader struct {
	wantErr error
}

func (e errReader) Read(_ []byte) (int, error) { return 0, e.wantErr }
func (errReader) Close() error                 { return nil }

// TestDetectMagicNumber_Errors covers the error and no-match branches of
// DetectMagicNumber.
func TestDetectMagicNumber_Errors(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))

	t.Run("reader_error", func(t *testing.T) {
		wantErr := errors.New("boom")
		rFn := func(_ context.Context) (io.ReadCloser, error) { return nil, wantErr }
		typ, score, err := files.DetectMagicNumber(ctx, rFn)
		require.Error(t, err)
		require.Equal(t, drivertype.None, typ)
		require.Zero(t, score)
	})

	t.Run("no_match_csv", func(t *testing.T) {
		rFn := func(_ context.Context) (io.ReadCloser, error) {
			return io.NopCloser(newStringReader("a,b,c\n1,2,3\n")), nil
		}
		typ, score, err := files.DetectMagicNumber(ctx, rFn)
		require.NoError(t, err)
		require.Equal(t, drivertype.None, typ)
		require.Zero(t, score)
	})

	t.Run("duckdb_magic", func(t *testing.T) {
		rFn := func(_ context.Context) (io.ReadCloser, error) {
			return io.NopCloser(newDuckHeaderReader()), nil
		}
		typ, score, err := files.DetectMagicNumber(ctx, rFn)
		require.NoError(t, err)
		require.Equal(t, drivertype.DuckDB, typ)
		require.Equal(t, float32(1.0), score)
	})
}

// TestFiles_DetectType_DuckDBExt verifies driverFromFileExt via DetectType
// for .duckdb and .ddb extensions (case-insensitive), without opening files.
func TestFiles_DetectType_DuckDBExt(t *testing.T) {
	testCases := []struct {
		loc      string
		wantType drivertype.Type
	}{
		{loc: "/no/such/x.duckdb", wantType: drivertype.DuckDB},
		{loc: "/no/such/x.ddb", wantType: drivertype.DuckDB},
		{loc: "/no/such/x.DDB", wantType: drivertype.DuckDB},
	}

	for _, tc := range testCases {
		t.Run(tc.loc, func(t *testing.T) {
			ctx, fs := newTestFiles(t)
			t.Cleanup(func() { assert.NoError(t, fs.Close()) })
			typ, err := fs.DetectType(ctx, "@h"+stringz.Uniq8(), tc.loc)
			require.NoError(t, err)
			require.Equal(t, tc.wantType, typ)
		})
	}
}

// TestFiles_DetectType_NoDetectors verifies that DetectType returns an error
// when no detectors are registered and the type can't be determined by
// extension/MIME.
func TestFiles_DetectType_NoDetectors(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	// Use a file with no extension so the ext/MIME shortcuts don't resolve a
	// type before the (absent) byte detectors are consulted.
	_, err := fs.DetectType(ctx, "@h"+stringz.Uniq8(), proj.Abs("drivers/csv/testdata/person_csv"))
	require.Error(t, err, "no detectors registered -> unable to determine type")
}

// TestFiles_DetectType_ParseError verifies the location.Parse error path.
func TestFiles_DetectType_ParseError(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	_, err := fs.DetectType(ctx, "@h"+stringz.Uniq8(), "postgres://user:@@@:not a valid url")
	require.Error(t, err)
}

// TestFiles_DetectType_ContextCancelled verifies the cancelled-context path
// in detectType.
func TestFiles_DetectType_ContextCancelled(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	fs := testh.New(t).Files()

	cctx, cancel := context.WithCancel(ctx)
	cancel()

	// A no-extension file forces the byte-detector path (which checks
	// ctx.Done()), rather than an early ext/MIME resolution.
	_, err := fs.DetectType(cctx, "@h"+stringz.Uniq8(), proj.Abs("drivers/csv/testdata/person_csv"))
	require.Error(t, err)
}

// TestFiles_DetectType_Undetectable verifies that a file the detectors can't
// classify returns an error (None, "unable to determine").
func TestFiles_DetectType_Undetectable(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	fs := testh.New(t).Files()

	typ, err := fs.DetectType(ctx, "@h"+stringz.Uniq8(), proj.Abs("README.md"))
	require.Error(t, err)
	require.Equal(t, drivertype.None, typ)
}

// TestFiles_DetectStdinType_Undetectable covers the path where stdin data
// can't be classified: DetectStdinType returns (None, nil).
func TestFiles_DetectStdinType_Undetectable(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	fs := testh.New(t).Files()

	f, err := os.Open(proj.Abs("README.md"))
	require.NoError(t, err)
	require.NoError(t, fs.AddStdin(ctx, f)) // AddStdin closes f

	typ, err := fs.DetectStdinType(ctx)
	require.NoError(t, err)
	require.Equal(t, drivertype.None, typ)
}

// TestDetectMagicNumber_ReadFullError covers the io.ReadFull error branch of
// DetectMagicNumber (a read error that is not io.ErrUnexpectedEOF).
func TestDetectMagicNumber_ReadFullError(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	rFn := func(_ context.Context) (io.ReadCloser, error) {
		return errReader{wantErr: errz.New("read boom")}, nil
	}
	typ, score, err := files.DetectMagicNumber(ctx, rFn)
	require.Error(t, err)
	require.Equal(t, drivertype.None, typ)
	require.Zero(t, score)
}

// TestFiles_DetectType_DetectorError verifies that a detector returning an
// error propagates out of detectType via g.Wait().
func TestFiles_DetectType_DetectorError(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })
	fs.AddDriverDetectors(erroringDetector)

	// A no-extension file forces the byte-detector path (rather than an early
	// ext/MIME resolution), so the erroring detector is invoked.
	_, err := fs.DetectType(ctx, "@h"+stringz.Uniq8(), proj.Abs("drivers/csv/testdata/person_csv"))
	require.Error(t, err, "erroring detector -> error from g.Wait()")
}

// TestFiles_DetectStdinType_DetectorError verifies that a detector error
// propagates out of DetectStdinType.
func TestFiles_DetectStdinType_DetectorError(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	_, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })
	fs.AddDriverDetectors(erroringDetector)

	f, err := os.Open(proj.Abs("drivers/csv/testdata/person_csv"))
	require.NoError(t, err)
	require.NoError(t, fs.AddStdin(ctx, f)) // AddStdin closes f

	_, err = fs.DetectStdinType(ctx)
	require.Error(t, err, "erroring detector -> error from DetectStdinType")
}
