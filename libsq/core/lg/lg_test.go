package lg_test

import (
	"context"
	"os"
	"testing"

	"github.com/neilotoole/sq/libsq/core/lg"

	"golang.org/x/exp/slog"
)

func TestSlg(t *testing.T) {
	_ = t
	// FIXME: delete
	ctx := context.Background()
	_ = ctx

	handler := slog.NewTextHandler(os.Stdout)
	log := slog.New(handler)

	ctx = lg.NewContext(ctx, log)
	log = lg.FromContext(ctx)

	log.Info("huzzah")
}
