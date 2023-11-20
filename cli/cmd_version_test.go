package cli_test

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"testing"

	"github.com/ecnepsnai/osquery"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/ioz"
)

func TestGetVersionFromBrewFormula(t *testing.T) {
	f, err := os.ReadFile("testdata/sq-0.20.0.rb")
	require.NoError(t, err)

	vers, err := cli.GetVersionFromBrewFormula(f)
	require.NoError(t, err)
	require.Equal(t, "0.20.0", vers)
}

func TestFetchBrewVersion(t *testing.T) {
	latest, err := cli.FetchBrewVersion(context.Background())
	require.NoError(t, err)
	require.True(t, semver.IsValid("v"+latest))
}

func TestOSQuery(t *testing.T) {
	info, err := osquery.Get()
	require.NoError(t, err)

	t.Logf("%+v", info)
}

func TestCmdVersion(t *testing.T) {
	bi := buildinfo.Get()
	ctx := context.Background()
	tr := testrun.New(ctx, t, nil)

	// --text
	err := tr.Exec("version", "--text")
	require.NoError(t, err)
	text := tr.Out.String()
	require.Contains(t, text, bi.Version)

	tr = testrun.New(ctx, t, nil)
	err = tr.Exec("version", "--text", "--verbose")
	require.NoError(t, err)
	text = tr.Out.String()

	checkStringsFn := func(text string) {
		require.Contains(t, text, bi.Version)
		require.Contains(t, text, runtime.GOOS)
		require.Contains(t, text, runtime.GOARCH)
	}
	checkStringsFn(text)

	// --json
	tr = testrun.New(ctx, t, nil)
	err = tr.Exec("version", "--json")
	require.NoError(t, err)
	text = tr.Out.String()
	checkStringsFn(text)

	m := map[string]any{}
	err = json.Unmarshal(tr.Out.Bytes(), &m)
	require.NoError(t, err)
	require.Equal(t, runtime.GOOS, m["host"].(map[string]any)["platform"])

	// --yaml
	tr = testrun.New(ctx, t, nil)
	err = tr.Exec("version", "--yaml")
	require.NoError(t, err)
	text = tr.Out.String()
	checkStringsFn(text)

	m = map[string]any{}
	err = ioz.UnmarshallYAML(tr.Out.Bytes(), &m)
	require.NoError(t, err)
	require.Equal(t, runtime.GOOS, m["host"].(map[string]any)["platform"])
}
