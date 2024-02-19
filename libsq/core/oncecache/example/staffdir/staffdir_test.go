package staffdir_test

import (
	"context"
	"encoding/json"
	"github.com/neilotoole/slogt"
	"github.com/neilotoole/sq/libsq/core/oncecache/example/staffdir"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestApp(t *testing.T) {
	ctx := context.Background()
	log := slogt.New(t)

	db, err := staffdir.NewInMemDB(log.With("layer", "db"), "testdata/acme.json")
	require.NoError(t, err)

	wileyDB, err := db.GetEmployee(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "Wile E. Coyote", wileyDB.Name)

	cache := staffdir.NewDirCache(log.With("layer", "cache"), db)
	require.NotNil(t, cache)

	wiley, err := cache.GetEmployee(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "Wile E. Coyote", wiley.Name)

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
