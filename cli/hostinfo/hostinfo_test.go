package hostinfo_test

import (
	"testing"

	"github.com/neilotoole/sq/cli/hostinfo"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
)

func TestGet(t *testing.T) {
	info := hostinfo.Get()

	log := lgt.New(t)
	log.Debug("Via slog", "sys", info)

	t.Logf("Via string: %s", info.String())
}
