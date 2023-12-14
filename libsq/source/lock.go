package source

import (
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz/lockfile"
)

// NewLock returns a new source.Lock instance.
//
// REVISIT: We may not actually use source.Lock at all, and
// instead stick with ioz/lockfile.Lockfile.
func NewLock(src *Source, pidfile string) (Lock, error) {
	lf, err := lockfile.New(pidfile)
	if err != nil {
		return Lock{}, errz.Err(err)
	}

	return Lock{
		Lockfile: lf,
		src:      src,
	}, nil
}

type Lock struct {
	lockfile.Lockfile
	src *Source
}

func (l Lock) Source() *Source {
	return l.src
}

func (l Lock) String() string {
	return l.src.Handle + ": " + string(l.Lockfile)
}
