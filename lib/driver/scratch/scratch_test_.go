package scratch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateFilename(t *testing.T) {

	filename, path := generateFilename()
	assert.NotEmpty(t, filename)
	assert.NotEmpty(t, path)

}
