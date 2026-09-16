package cli_test

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/libsq/core/options"
)

// TestLogLevelOptGetRawValue verifies that LogLevelOpt.Get converts the raw
// forms that a config file yields. A slog.Level can only reach the config as
// a string or an int, so without this, an unprocessed Options silently yields
// the default level. See #1209.
func TestLogLevelOptGetRawValue(t *testing.T) {
	opt := cli.NewLogLevelOpt("log.level", slog.LevelInfo, "", "")

	raw := options.Options{"log.level": "debug"}
	require.Equal(t, slog.LevelDebug, opt.Get(raw),
		"Get must convert the raw string, not fall back to the default")

	reg := &options.Registry{}
	reg.Add(opt)
	processed, err := reg.Process(raw)
	require.NoError(t, err)
	require.Equal(t, slog.LevelDebug, opt.Get(processed),
		"Get must agree with itself on processed and unprocessed options")

	require.Equal(t, slog.LevelInfo, opt.Get(options.Options{"log.level": "not-a-level"}),
		"an unconvertible value still yields the default")
}
