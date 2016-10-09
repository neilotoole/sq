package scratch

import (
	"testing"

	"os"

	"github.com/stretchr/testify/assert"
)

func TestGenerateFilename(t *testing.T) {

	filename, path := generateFilename(os.TempDir())
	assert.NotEmpty(t, filename)
	assert.NotEmpty(t, path)

}
