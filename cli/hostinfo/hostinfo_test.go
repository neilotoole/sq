package hostinfo_test

import (
	"testing"

	"github.com/neilotoole/slogt"

	"github.com/neilotoole/sq/cli/hostinfo"
)

func TestGet(t *testing.T) {
	info := hostinfo.Get()

	log := slogt.New(t)
	log.Debug("Via slog", "sys", info)

	t.Logf("Via string: %s", info.String())
}
