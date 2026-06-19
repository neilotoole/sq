package cli

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/options"
)

func TestOptFormatDecimal(t *testing.T) {
	// Default is the precision-safe "string".
	require.Equal(t, "string", OptFormatDecimal.Get(options.Options{}))

	// Valid values pass Process.
	for _, v := range []string{"string", "number"} {
		_, err := OptFormatDecimal.Process(options.Options{OptFormatDecimal.Key(): v})
		require.NoError(t, err, "value %q should be valid", v)
	}

	// Invalid values are rejected.
	_, err := OptFormatDecimal.Process(options.Options{OptFormatDecimal.Key(): "bogus"})
	require.Error(t, err)

	// The CLI flag name equals the option key.
	require.Equal(t, "format.decimal", OptFormatDecimal.Flag().Name)
}
