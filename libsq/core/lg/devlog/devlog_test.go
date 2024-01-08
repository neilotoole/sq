package devlog_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
)

func TestDevlog(t *testing.T) {
	log := lgt.New(t)
	err := errz.New("oh noes")
	log.Error("bah", lga.Err, err)
}

func TestDevlogTextHandler(t *testing.T) {
	o := &slog.HandlerOptions{}

	h := slog.NewTextHandler(os.Stdout, o)
	log := slog.New(h)
	err := errz.New("oh noes")
	log.Error("bah", lga.Err, err)
}
