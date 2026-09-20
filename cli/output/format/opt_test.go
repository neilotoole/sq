package format_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/libsq/core/options"
)

// TestOptGetRawValue verifies that format.Opt.Get converts the raw string form
// that a config file yields. A Format value can only ever reach the config as
// a string, so without this, an unprocessed Options silently yields the
// default format. See #1209.
func TestOptGetRawValue(t *testing.T) {
	opt := format.NewOpt("f", nil, format.Text, nil, "", "")

	raw := options.Options{"f": "json"}
	require.Equal(t, format.JSON, opt.Get(raw),
		"Get must convert the raw string, not fall back to the default")

	reg := &options.Registry{}
	reg.Add(opt)
	processed, err := reg.Process(raw)
	require.NoError(t, err)
	require.Equal(t, format.JSON, opt.Get(processed),
		"Get must agree with itself on processed and unprocessed options")

	require.Equal(t, format.Text, opt.Get(options.Options{"f": "not-a-format"}),
		"an unconvertible value still yields the default")
}
