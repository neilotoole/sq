package lg_test

import (
	"bytes"
	"strings"
	"testing"

	"os"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/go-lg/test/filter/pkg1"
	"github.com/neilotoole/go-lg/test/filter/pkg2"
	"github.com/neilotoole/go-lg/test/filter/pkg3"
	"github.com/stretchr/testify/assert"
)

func TestFilters(t *testing.T) {

	buf := useNewLgBuf()
	logPackages()
	assert.Equal(t, 9, countLines(buf))

	buf = useNewLgBuf()
	lg.Levels(lg.LevelDebug)
	logPackages()
	assert.Equal(t, 5, countLines(buf))

	buf = useNewLgBuf()
	lg.Levels()
	logPackages()
	assert.Equal(t, 0, countLines(buf))

	buf = useNewLgBuf()
	lg.Levels(lg.LevelAll)
	logPackages()
	assert.Equal(t, 9, countLines(buf), "levels should be reset to all")

	buf = useNewLgBuf()
	lg.Exclude("github.com/neilotoole/go-lg/test/filter/pkg1")
	logPackages()
	assert.Equal(t, 6, countLines(buf))

	buf = useNewLgBuf()
	lg.Exclude("github.com/neilotoole/go-lg/test/filter/pkg1", "github.com/neilotoole/go-lg/test/filter/pkg2")
	logPackages()
	assert.Equal(t, 3, countLines(buf))

	buf = useNewLgBuf()
	lg.Exclude("github.com/neilotoole/go-lg/test/filter/pkg1", "github.com/neilotoole/go-lg/test/filter/pkg2", "github.com/neilotoole/go-lg/test/filter/pkg3")
	logPackages()
	assert.Equal(t, 0, countLines(buf))

	buf = useNewLgBuf()
	lg.Exclude("github.com/neilotoole/go-lg/test/filter")
	logPackages()
	assert.Equal(t, 0, countLines(buf), "all sub-packages should have been excluded")

	buf = useNewLgBuf()
	lg.Excluded = nil
	logPackages()
	assert.Equal(t, 9, countLines(buf), "should have reset all pkg filters")

	buf = useNewLgBuf()
	lg.Disable()
	logPackages()
	assert.Equal(t, 0, countLines(buf), "logging should be entirely disabled")

	buf = useNewLgBuf()
	lg.Enable()
	logPackages()
	assert.Equal(t, 9, countLines(buf), "logging should be re-enabled")
}

func countLines(buf *bytes.Buffer) int {
	return strings.Count(buf.String(), "\n")
}

func useNewLgBuf() *bytes.Buffer {
	buf := &bytes.Buffer{}
	lg.Use(buf)
	return buf
}

func logPackages() {

	pkg1.LogDebug()
	pkg1.LogWarn()
	pkg1.LogError()

	pkg2.LogDebug()
	pkg2.LogWarn()
	pkg2.LogError()

	pkg3.LogDebug()
	pkg3.LogWarn()
	pkg3.LogError()
}

func resetLg() {

	lg.Enable()
	lg.Excluded = nil
	lg.Levels(lg.LevelAll)
	lg.Use(os.Stdout)
}

func TestDepth(t *testing.T) {
	resetLg()
	buf := useNewLgBuf()
	lg.Debugf("regular debug")
	assert.True(t, strings.Contains(buf.String(), ":lg_test.TestDepth] regular debug"))

	buf = useNewLgBuf()
	lg.Depth(1).Debugf("debug with depth")
	assert.True(t, strings.Contains(buf.String(), ":testing.tRunner] debug with depth"), "should be calling on behalf of ancestor function (testing.tRunner)")

}
