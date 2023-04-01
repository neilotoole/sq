package slg_test

import (
	"context"
	"os"
	"testing"

	"github.com/neilotoole/sq/libsq/core/slg"

	"golang.org/x/exp/slog"
)

func TestSlg(t *testing.T) {
	_ = t
	// FIXME: delete
	ctx := context.Background()
	_ = ctx

	handler := slog.NewTextHandler(os.Stdout)
	log := slog.New(handler)

	ctx = slg.NewContext(ctx, log)
	log = slg.FromContext(ctx)

	log.Info("huzzah")
}
