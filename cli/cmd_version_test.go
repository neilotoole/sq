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

func TestGetVersionFromBrewFormula_URLBased(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		wantVer string
		wantErr bool
	}{
		{
			name: "homebrew-core_tar.gz_format",
			input: `class Sq < Formula
  desc "swiss-army knife for data"
  homepage "https://sq.io"
  url "https://github.com/neilotoole/sq/archive/refs/tags/v0.48.3.tar.gz"
  sha256 "abc123"
  license "MIT"

  bottle do
`,
			wantVer: "0.48.3",
		},
		{
			name: "homebrew-core_zip_format",
			input: `class Sq < Formula
  url "https://github.com/neilotoole/sq/archive/refs/tags/v1.2.3.zip"
  sha256 "abc123"

  bottle do
`,
			wantVer: "1.2.3",
		},
		{
			name: "invalid_semver_in_URL",
			input: `class Sq < Formula
  url "https://github.com/neilotoole/sq/archive/refs/tags/vnotvalid.tar.gz"

  bottle do
`,
			wantErr: true,
		},
		{
			name: "explicit_version_takes_precedence",
			input: `class Sq < Formula
  version "0.50.0"
  url "https://github.com/neilotoole/sq/archive/refs/tags/v0.48.3.tar.gz"

  bottle do
`,
			wantVer: "0.50.0",
		},
		{
			name: "unrecognized_extension_falls_through_to_explicit_version",
			input: `class Sq < Formula
  url "https://github.com/neilotoole/sq/archive/refs/tags/v1.0.0.tar.xz"
  version "0.48.11"

  bottle do
`,
			wantVer: "0.48.11",
		},
		{
			name: "no_version_found",
			input: `class Sq < Formula
  desc "swiss-army knife for data"

  bottle do
`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vers, err := cli.GetVersionFromBrewFormula([]byte(tc.input))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantVer, vers)
		})
	}
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
