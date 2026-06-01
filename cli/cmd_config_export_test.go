package cli_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
)

// TestCmdConfigExport_Portable verifies that without --resolve, the
// output is valid YAML and any ${scheme:path} placeholder is written
// verbatim (no resolution attempt).
func TestCmdConfigExport_Portable(t *testing.T) {
	gokeyring.MockInit()

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@sakila",
		Type:     drivertype.SQLite,
		Location: "sqlite3://${keyring:abc123}",
	}))

	err := tr.Exec("config", "export")
	require.NoError(t, err)

	got := tr.OutString()
	require.Contains(t, got, "config.version:")
	require.Contains(t, got, "@sakila")
	require.Contains(t, got, "${keyring:abc123}",
		"placeholder must be preserved without --resolve")
	require.False(t, strings.Contains(got, "Warning:"),
		"no stderr warning without --resolve")
	require.Equal(t, "", tr.ErrOut.String())
}
