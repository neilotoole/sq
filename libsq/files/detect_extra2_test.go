package files_test

import (
	"context"
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
