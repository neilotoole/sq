package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/testh/proj"
)

func TestFileStore_Nil_Save(t *testing.T) {
	t.Parallel()

	var f *config.YAMLFileStore

	// noinspection GoNilness
	err := f.Save(context.Background(), config.New())
	require.Error(t, err)
}

func TestFileStore_LoadSaveLoad(t *testing.T) {
	t.Parallel()

	// good.01.sq.yml has a bunch of fixtures in it
	fs := &config.YAMLFileStore{Path: "testdata/good.01.sq.yml", HookLoad: hookExpand}
	const expectGood01SrcCount = 34

	cfg, err := fs.Load(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Collection)
	require.Equal(t, expectGood01SrcCount, len(cfg.Collection.Sources()))

	f, err := os.CreateTemp("", "*.sq.yml")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, f.Close()) })

	fs.Path = f.Name()
	t.Logf("writing to tmp file: %s", fs.Path)

	err = fs.Save(context.Background(), cfg)
	require.NoError(t, err)

	cfg2, err := fs.Load(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg2)
	require.Equal(t, expectGood01SrcCount, len(cfg2.Collection.Sources()))
	require.EqualValues(t, cfg, cfg2)
}

// hookExpand expands variables in data, e.g. ${SQ_ROOT}.
var hookExpand = func(data []byte) ([]byte, error) {
	return []byte(proj.Expand(string(data))), nil
}

func TestFileStore_Load(t *testing.T) {
	t.Parallel()

	good, err := filepath.Glob("testdata/good.*")
	require.NoError(t, err)
	bad, err := filepath.Glob("testdata/bad.*")
	require.NoError(t, err)

	t.Logf("%d good fixtures, %d bad fixtures", len(good), len(bad))

	fs := &config.YAMLFileStore{HookLoad: hookExpand}

	for _, match := range good {
		match := match
		t.Run(tutil.Name(match), func(t *testing.T) {
			t.Parallel()

			fs.Path = match
			_, err = fs.Load(context.Background())
			require.NoError(t, err, match)
		})
	}

	for _, match := range bad {
		match := match
		t.Run(tutil.Name(match), func(t *testing.T) {
			fs.Path = match
			_, err = fs.Load(context.Background())
			require.Error(t, err, match)
		})
	}
}
