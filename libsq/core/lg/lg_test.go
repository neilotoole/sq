package lg_test

import (
	"context"
	"testing"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
)

func TestContext(t *testing.T) {
	ctx := context.Background()
	log := lgt.New(t)

	ctx = lg.NewContext(ctx, log)
	log = lg.FromContext(ctx)

	log.Info("huzzah")
}
