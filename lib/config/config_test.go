package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeout(t *testing.T) {

	text := "5s"

	d, err := time.ParseDuration(text)
	require.Nil(t, err)

	assert.Equal(t, time.Second*5, d)

}
