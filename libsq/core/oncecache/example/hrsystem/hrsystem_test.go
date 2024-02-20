package hrsystem_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/slogt"

	"github.com/neilotoole/sq/libsq/core/oncecache/example/hrsystem"
)

const (
	acmeName    = "Acme Corporation"
	engDeptName = "Engineering"
	qaDeptName  = "QA"
	wileyName   = "Wile E. Coyote"
)

func setup(t *testing.T) (*hrsystem.HRCache, *hrsystem.HRDatabase) {
	t.Helper()
	log := slogt.New(t)

	db, err := hrsystem.NewHRDatabase(log.With("layer", "db"), "testdata/acme.json")
	require.NoError(t, err)
	cache := hrsystem.NewHRCache(log.With("layer", "cache"), db)
	return cache, db
}

func TestHRCache_Basic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cache, db := setup(t)

	require.Equal(t, 0, cache.Stats().GetEmployee())
	require.Equal(t, 0, db.Stats().GetEmployee())

	wiley, err := cache.GetEmployee(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, wileyName, wiley.Name)

	require.Equal(t, 1, cache.Stats().GetEmployee())
	require.Equal(t, 1, db.Stats().GetEmployee())

	wiley, err = cache.GetEmployee(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, wileyName, wiley.Name)

	require.Equal(t, 2, cache.Stats().GetEmployee())
	require.Equal(t, 1, db.Stats().GetEmployee())
}

func TestHRCache_Propagation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cache, db := setup(t)

	// The GetOrg call should trigger cache entry propagation.
	acme, err := cache.GetOrg(ctx, acmeName)
	require.NoError(t, err)
	require.Equal(t, acmeName, acme.Name)
	require.Equal(t, 1, db.Stats().GetOrg())
	require.Equal(t, 1, cache.Stats().GetOrg())

	wiley, err := cache.GetEmployee(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, wileyName, wiley.Name)

	require.Equal(t, 1, cache.Stats().GetEmployee())
	require.Equal(t, 0, db.Stats().GetEmployee())

	engDept, err := cache.GetDepartment(ctx, engDeptName)
	require.NoError(t, err)
	require.Equal(t, engDeptName, engDept.Name)

	require.Equal(t, 1, cache.Stats().GetDepartment())
	require.Equal(t, 0, db.Stats().GetDepartment())
}

func TestLoadJSON(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/acme.json")
	require.NoError(t, err)

	acme := &hrsystem.Org{}
	require.NoError(t, json.Unmarshal(data, acme))

	require.Equal(t, acmeName, acme.Name)
	require.Len(t, acme.Departments, 2)

	require.Equal(t, engDeptName, acme.Departments[0].Name)
	require.Len(t, acme.Departments[0].Staff, 2)

	require.Equal(t, qaDeptName, acme.Departments[1].Name)
	require.Len(t, acme.Departments[1].Staff, 2)

	require.Equal(t, wileyName, acme.Departments[0].Staff[0].Name)
}
