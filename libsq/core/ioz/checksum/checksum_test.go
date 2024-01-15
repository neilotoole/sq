package checksum_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
)

func TestSum(t *testing.T) {
	got := checksum.Sum(nil)
	require.Equal(t, "", got)
	got = checksum.Sum([]byte{})
	require.Equal(t, "", got)
	got = checksum.Sum([]byte("hello world"))
	assert.Equal(t, "d4a1185", got)
}
