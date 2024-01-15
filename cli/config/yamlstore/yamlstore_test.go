package yamlstore_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/config/yamlstore"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/tu"
)

func TestFileStore_Nil_Save(t *testing.T) {
	var f *yamlstore.Store

	err := f.Save(context.Background(), config.New())
	require.Error(t, err)
}

func TestFileStore_LoadSaveLoad(t *testing.T) {
	ctx := context.Background()

	const wantVers = `v0.34.0`

	// good.01.sq.yml has a bunch of fixtures in it
	fs := &yamlstore.Store{
		Path:            "testdata/good.01.sq.yml",
		HookLoad:        hookExpand,
		OptionsRegistry: &options.Registry{},
	}
	cli.RegisterDefaultOpts(fs.OptionsRegistry)
	const expectGood01SrcCount = 34

	cfg, err := fs.Load(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Collection)
	require.Equal(t, wantVers, cfg.Version)
	require.Equal(t, expectGood01SrcCount, len(cfg.Collection.Sources()))

	f, err := os.CreateTemp("", "*.sq.yml")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, f.Close()) })

	fs.Path = f.Name()
	t.Logf("writing to tmp file: %s", fs.Path)

	err = fs.Save(ctx, cfg)
	require.NoError(t, err)

	cfg2, err := fs.Load(ctx)
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
	optsReg := &options.Registry{}
	cli.RegisterDefaultOpts(optsReg)

	good, err := filepath.Glob("testdata/good.*")
	require.NoError(t, err)
	bad, err := filepath.Glob("testdata/bad.*")
	require.NoError(t, err)

	t.Logf("%d good fixtures, %d bad fixtures", len(good), len(bad))

	fs := &yamlstore.Store{
		HookLoad:        hookExpand,
		OptionsRegistry: optsReg,
	}

	for _, match := range good {
		match := match
		t.Run(tu.Name(match), func(t *testing.T) {
			fs.Path = match
			cfg, err := fs.Load(context.Background())
			require.NoError(t, err, match)
			require.NotNil(t, cfg)
		})
	}

	for _, match := range bad {
		match := match
		t.Run(tu.Name(match), func(t *testing.T) {
			fs.Path = match
			cfg, err := fs.Load(context.Background())
			t.Log(err)
			require.Error(t, err, match)
			require.Nil(t, cfg)
		})
	}
}
