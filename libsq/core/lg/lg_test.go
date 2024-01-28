package lg_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
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

func TestDepth(t *testing.T) {
	// I'm not motivated enough to write a test that actually checks output
	// (e.g. by setting the log output to a bytes.Buffer), so this
	// is just a visual check.
	log := lgt.New(t)
	log = log.With("name", "alice", "age", 42)
	err := errors.New("TestSourceAttr error")

	log.Warn("TestSourceAttr - NO DEPTH")
	lg.Depth(log, slog.LevelWarn, 0, "TestSourceAttr log depth")
	lg.WarnIfError(log, "TestSourceAttr", err)
	lg.WarnIfFuncError(log, "TestSourceAttr", errorFunc)
	lg.WarnIfCloseError(log, "TestSourceAttr", errorCloser{})
	lg.Unexpected(log, err)

	nest1(log)
}

func nest1(log *slog.Logger) {
	err := errors.New("nest1 error")
	log.Warn("nest1 - NO DEPTH")
	lg.Depth(log, slog.LevelWarn, 0, "nest1 log depth")
	lg.WarnIfError(log, "nest1", err)
	lg.WarnIfFuncError(log, "nest1", errorFunc)
	lg.WarnIfCloseError(log, "nest1", errorCloser{})
	lg.Unexpected(log, err)

	nest2(log)
}

func nest2(log *slog.Logger) {
	err := errors.New("nest2 error")
	log.Warn("nest2 - NO DEPTH")
	lg.Depth(log, slog.LevelWarn, 0, "nest2 log depth")
	lg.WarnIfError(log, "nest2", err)
	lg.WarnIfFuncError(log, "nest2", errorFunc)
	lg.WarnIfCloseError(log, "nest2", errorCloser{})
	lg.Unexpected(log, err)
}

func errorFunc() error {
	return errors.New("errorFunc went bad")
}

var _ io.Closer = errorCloser{}

type errorCloser struct{}

func (e errorCloser) Close() error {
	return errors.New("errorCloser.Close went bad")
}
