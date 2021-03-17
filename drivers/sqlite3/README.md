# sqlite3

This package provides support for [SQLite](https://sqlite.org).

The underlying Golang SQL driver is [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3).



## SpatiaLite

Support is provided for the [SpatiaLite](https://www.gaia-gis.it/fossil/libspatialite/index)
geometry extension.

The SpatiaLite libraries must be installed separately.

For macOS, use `brew install spatialite`.

The code to load the extension is copied from: https://github.com/shaxbee/go-spatialite


The spatialite test DBs come from:

- https://github.com/dpmcmlxxvi/SpatiaLiteCpp/blob/master/examples/spatialite/test-2.3.sqlite
- https://git.asi.ru/solutions/world-ai-and-data-challenge/trase/-/blob/fd8f090fb6f934890d4b5e95df4976c65f183292/aequilibrae/reference_files/spatialite.sqlite
- https://osf.io/2ym95/
