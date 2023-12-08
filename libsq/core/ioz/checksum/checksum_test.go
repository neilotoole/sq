package checksum_test

import (
	"testing"

	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/stretchr/testify/assert"
)

func TestHash(t *testing.T) {
	got := checksum.Hash([]byte("hello world"))
	t.Log(got)
	assert.Equal(t, "d4a1185", got)
}
