package checksum_test

import (
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
)

func TestHash(t *testing.T) {
	got := checksum.Sum(nil)
	require.Equal(t, "", got)
	got = checksum.Sum([]byte{})
	require.Equal(t, "", got)
	got = checksum.Sum([]byte("hello world"))
	assert.Equal(t, "d4a1185", got)
}
