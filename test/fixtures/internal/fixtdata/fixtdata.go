// Package fixtdata declares which Sakila fixtures are published at
// sq.io/testdata and which canonical in-repo fixture each one is copied from.
//
// It is shared by the gentestdata command, which writes the published copies,
// and by the guard test in test/fixtures, which verifies they have not
// drifted. Keeping the mapping in one place stops the generator and its guard
// from disagreeing.
package fixtdata

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
