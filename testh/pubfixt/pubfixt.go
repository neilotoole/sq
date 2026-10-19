// Package pubfixt declares which Sakila fixtures are published at
// sq.io/testdata and which canonical in-repo fixture each one is copied from.
//
// It is the single source of truth for three consumers: the gentestdata
// command, which writes the published copies; the guard test in test/fixtures,
// which verifies they have not drifted; and testh/fixtsrv, which serves them
// over 127.0.0.1 for tests that need a real HTTP fetch.
//
// It lives under testh/ rather than test/fixtures/internal/ so that testh can
// import it. An internal package there would be importable only from under
// test/fixtures, which would leave fixtsrv duplicating PublishedDir.
package pubfixt

// PublishedDir is the project-relative dir served at sq.io/testdata.
const PublishedDir = "/site/static/testdata"

// Files maps each published file to the project-relative path of the
// canonical fixture it is copied from.
//
// Files in PublishedDir that are not Sakila data (person.csv, demo_person.csv,
// xl_demo.xlsx) have no canonical counterpart and are deliberately absent.
var Files = map[string]string{
	"actor.csv":          "/drivers/csv/testdata/sakila-csv/actor.csv",
	"film.csv":           "/drivers/csv/testdata/sakila-csv/film.csv",
	"sakila.db":          "/drivers/sqlite3/testdata/sakila.db",
	"sakila.xlsx":        "/drivers/xlsx/testdata/sakila.xlsx",
	"sakila_subset.xlsx": "/drivers/xlsx/testdata/sakila_subset.xlsx",
}

// Tarballs maps each published tarball to the project-relative dir whose
// files it packs. The archive holds one root entry named for the dir, then
// that dir's files.
var Tarballs = map[string]string{
	"sakila-csv.tar.gz": "/drivers/csv/testdata/sakila-csv",
	"sakila-tsv.tar.gz": "/drivers/csv/testdata/sakila-tsv",
}

// GenCmd is the command that regenerates the published fixtures. It appears
// in the guard test's failure messages.
const GenCmd = "go run ./test/fixtures/internal/gentestdata"
