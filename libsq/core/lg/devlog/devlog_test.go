package devlog_test

import (
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"testing"
)

func TestDevlog(t *testing.T) {
	log := lgt.New(t)
	//log.Debug("huzzah")
	err := errz.New("oh noes")
	log.Error("bah", lga.Err, err, lga.Stack, errz.Stacks(err))

}
