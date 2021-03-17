package sqlite3

// This file contains code supporting the SQLite SpatiaLite extensions.
// See:
// - https://www.gaia-gis.it/gaia-sins/spatialite-tutorial-2.3.1.html
// - https://www.gaia-gis.it/fossil/libspatialite/wiki?name=mod_spatialite
// - https://en.wikipedia.org/wiki/Well-known_text_representation_of_geometry
// - https://docs.geotools.org/stable/javadocs/org/opengis/referencing/doc-files/WKT.html

// Acknowledgement: some of the code in this file derives
// from: https://github.com/shaxbee/go-spatialite

import (
	"database/sql"
	"errors"
	"sync"

	"github.com/mattn/go-sqlite3"
)

// errMsgNoSpatialite is returned by sqlite when the
// spatialite extension is not loaded.
const errMsgNoSpatialite = "no such module: VirtualSpatialIndex"

const spatialite = "spatialite"

type libEntrypoint struct {
	lib  string
	proc string
}

var spatialiteLibs = []libEntrypoint{
	{"mod_spatialite", "sqlite3_modspatialite_init"},
	{"mod_spatialite.dylib", "sqlite3_modspatialite_init"},
	{"libspatialite.so", "sqlite3_modspatialite_init"},
	{"libspatialite.so.5", "spatialite_init_ex"},
	{"libspatialite.so", "spatialite_init_ex"},
}

// ErrSpatialiteNotFound is returned if the spatialite extension
// can not be loaded.
var ErrSpatialiteNotFound = errors.New("spatialite extension not found")

var spatialiteOnce sync.Once

func registerSpatialite() {
	spatialiteOnce.Do(func() {
		sql.Register(spatialite, &sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				for _, v := range spatialiteLibs {
					if err := conn.LoadExtension(v.lib, v.proc); err == nil {
						return nil
					}
				}
				return ErrSpatialiteNotFound
			},
		})
	})
}
