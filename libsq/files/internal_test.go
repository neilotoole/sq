package files

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz/lockfile"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// newInternalFiles constructs a *Files for white-box tests.
func newInternalFiles(t *testing.T) *Files {
	t.Helper()
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	lockFn := func(context.Context) (unlock func(), err error) {
		return func() {}, nil
	}
	fs, err := New(ctx, nil, lockfile.LockFunc(lockFn), t.TempDir(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })
	return fs
}

// TestFiles_filepath covers the file, SQL, and stdin branches of the
// unexported filepath method.
func TestFiles_filepath(t *testing.T) {
	fs := newInternalFiles(t)

	t.Run("file", func(t *testing.T) {
		src := &source.Source{Handle: "@h", Type: drivertype.CSV, Location: "/tmp/a.csv"}
		fp, err := fs.filepath(src)
		require.NoError(t, err)
		require.Equal(t, "/tmp/a.csv", fp)
	})

	t.Run("sql", func(t *testing.T) {
		src := &source.Source{Handle: "@pg", Type: drivertype.Pg, Location: "postgres://u:p@localhost/db"}
		_, err := fs.filepath(src)
		require.Error(t, err)
	})

	t.Run("stdin", func(t *testing.T) {
		src := &source.Source{Handle: source.StdinHandle, Location: source.StdinHandle}
		_, err := fs.filepath(src)
		require.Error(t, err)
	})
}

// TestFiles_sourceHash_Nil verifies sourceHash(nil) returns "".
func TestFiles_sourceHash_Nil(t *testing.T) {
	fs := newInternalFiles(t)
	require.Empty(t, fs.sourceHash(nil))
}
