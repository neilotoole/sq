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
	// log.Debug("huzzah")
	err := errz.New("oh noes")
	// stack := errs.Stacks(err)
	//  lga.Stack, errz.Stacks(err)
	log.Error("bah", lga.Err, err)
}

func TestDevlogTextHandler(t *testing.T) {
	o := &slog.HandlerOptions{
		ReplaceAttr: ReplaceAttr,
	}

	h := slog.NewTextHandler(os.Stdout, o)
	log := slog.New(h)
	// log := lgt.New(t)
	// log.Debug("huzzah")
	err := errz.New("oh noes")
	// stack := errs.Stacks(err)
	//  lga.Stack, errz.Stacks(err)
	log.Error("bah", lga.Err, err)
}

func ReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case "pid":
		return slog.Attr{}
	case "error":
		if _, ok := a.Value.Any().(error); ok {
			a.Key = "e"
		}
		a.Key = "wussah"
		return a
	default:
		return a
	}
}
