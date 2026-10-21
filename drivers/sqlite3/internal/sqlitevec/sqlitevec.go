// Package sqlitevec bundles the sqlite-vec extension (vlasky/sqlite-vec fork)
// as vendored C source and registers it with SQLite via
// sqlite3_auto_extension, so every connection opened by mattn/go-sqlite3
// afterward has the vec0 virtual table and the vec_* functions.
//
// The extension is compiled statically against the SQLite amalgamation that
// mattn/go-sqlite3 links. See README.md for the vendored version.
package sqlitevec

/*
#cgo CFLAGS: -DSQLITE_CORE -DSQLITE_VEC_STATIC -DSQLITE_VEC_OMIT_FS
#cgo darwin,arm64 CFLAGS: -DSQLITE_VEC_ENABLE_NEON
#cgo linux LDFLAGS: -lm

#include "sqlite-vec.h"
*/
import "C"

import (
	"sync"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// sqliteOK is SQLITE_OK.
const sqliteOK = 0

// register is memoized so that sqlite3_auto_extension is called only once.
var register = sync.OnceValue(func() error {
	rc := C.sqlite3_auto_extension((*[0]byte)(C.sqlite3_vec_init))
	if rc != sqliteOK {
		return errz.Errorf("register sqlite-vec: sqlite rc %d", int(rc))
	}
	return nil
})

// Register registers sqlite-vec with SQLite for all subsequently opened
// connections in this process. It is idempotent and safe for concurrent use.
func Register() error {
	return register()
}

// HeaderVersion returns SQLITE_VERSION from the vendored sqlite3.h. It must
// equal the runtime sqlite_version() of the linked SQLite.
func HeaderVersion() string {
	return C.SQLITE_VERSION
}

// Version returns SQLITE_VEC_VERSION of the vendored sqlite-vec.
func Version() string {
	return C.SQLITE_VEC_VERSION
}
