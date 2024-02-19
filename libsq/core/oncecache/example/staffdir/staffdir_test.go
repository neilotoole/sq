package staffdir_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/neilotoole/sq/libsq/core/oncecache/example/staffdir"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) (*slog.Logger, *staffdir.InMemDB, *staffdir.DirCache) {
	log := slogt.New(t)

	db, err := staffdir.NewInMemDB(log.With("layer", "db"), "testdata/acme.json")
	require.NoError(t, err)
	cache := staffdir.NewDirCache(log.With("layer", "cache"), db)
	return log, db, cache
}

func TestApp(t *testing.T) {
	const wileyName = "Wile E. Coyote"

	ctx := context.Background()

	_, db, cache := setup(t)
	_ = db

	wiley, err := cache.GetEmployee(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, wileyName, wiley.Name)

	require.Equal(t, 1, db.Stats().GetEmployee())
	require.Equal(t, 1, cache.Stats().GetEmployee())

	wiley, err = cache.GetEmployee(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, wileyName, wiley.Name)

	require.Equal(t, 1, db.Stats().GetEmployee())
	require.Equal(t, 2, cache.Stats().GetEmployee())
}

func TestLoadJSON(t *testing.T) {
	data, err := os.ReadFile("testdata/acme.json")
	require.NoError(t, err)

	acme := &staffdir.Company{}
	require.NoError(t, json.Unmarshal(data, acme))

	require.Equal(t, "Acme Corporation", acme.Name)
	require.Len(t, acme.Departments, 2)

	require.Equal(t, "Engineering", acme.Departments[0].Name)
	require.Len(t, acme.Departments[0].Staff, 2)

	require.Equal(t, "Testing", acme.Departments[1].Name)
	require.Len(t, acme.Departments[1].Staff, 2)

	require.Equal(t, "Wile E. Coyote", acme.Departments[0].Staff[0].Name)
}
