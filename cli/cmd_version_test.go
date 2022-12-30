package cli

import (
	"context"
	"os"
	"testing"

	"golang.org/x/mod/semver"

	"github.com/stretchr/testify/require"
)

func TestGetVersionFromBrewFormula(t *testing.T) {
	f, err := os.ReadFile("testdata/sq-0.20.0.rb")
	require.NoError(t, err)

	vers, err := getVersionFromBrewFormula(f)
	require.NoError(t, err)
	require.Equal(t, "0.20.0", vers)
}

func TestFetchBrewVersion(t *testing.T) {
	latest, err := fetchBrewVersion(context.Background())
	require.NoError(t, err)
	require.True(t, semver.IsValid("v"+latest))
}
