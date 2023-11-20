package ioz_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
)

func TestMarshalYAML(t *testing.T) {
	m := map[string]any{
		"hello": `sqlserver://sakila:p_ss"**W0rd@222.75.174.219?database=sakila`,
	}

	b, err := ioz.MarshalYAML(m)
	require.NoError(t, err)
	require.NotNil(t, b)
}
