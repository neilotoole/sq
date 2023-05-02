package lg_test

import (
	"context"
	"testing"

	"github.com/neilotoole/slogt"

	"github.com/neilotoole/sq/libsq/core/lg"
)

func TestContext(t *testing.T) {
	ctx := context.Background()
	log := slogt.New(t)

	ctx = lg.NewContext(ctx, log)
	log = lg.FromContext(ctx)

	log.Info("huzzah")
}
