// Package sakila holds test constants and such for the sakila test sources.
package sakila

import (
	"github.com/neilotoole/sq/libsq/sqlz"
)

// Sakila source handles.
const (
	XLSX             = "@sakila_xlsx"
	XLSXSubset       = "@sakila_xlsx_subset"
	XLSXNoHeader     = "@sakila_xlsx_noheader"
	CSVActor         = "@sakila_csv_actor"
	CSVActorHTTP     = "@sakila_csv_actor_http"
	CSVActorNoHeader = "@sakila_csv_actor_noheader"
	TSVActor         = "@sakila_tsv_actor"
	TSVActorNoHeader = "@sakila_tsv_actor_noheader"
	SL3              = "@sakila_sl3"
	Pg9              = "@sakila_pg9"
	Pg10             = "@sakila_pg10"
	Pg11             = "@sakila_pg11"
	Pg12             = "@sakila_pg12"
	Pg               = Pg12
	My56             = "@sakila_my56"
	My57             = "@sakila_my57"
	My8              = "@sakila_my8"
	My               = My8
	MS17             = "@sakila_ms17"
	MS               = MS17
)

// All returns all the sakila handles. It does not
// include monotable handles such as @sakila_csv_actor.
func All() []string {
	return []string{SL3, Pg9, Pg10, Pg11, Pg12, My56, My57, My8, MS17, XLSX}
}

// SQLAll returns all the sakila SQL handles.
func SQLAll() []string {
	return []string{SL3, Pg9, Pg10, Pg11, Pg12, My56, My57, My8, MS17}
}

// SQLAllExternal is the same as SQLAll, but only includes
// external (non-embedded) sources. That is, it excludes SL3.
func SQLAllExternal() []string {
	return []string{Pg9, Pg10, Pg11, Pg12, My56, My57, My8, MS17}
}

// SQLLatest returns the handles for the latest
// version of each supported SQL database. This is provided
// in addition to SQLAll to enable quicker iterative testing
// during development.
func SQLLatest() []string {
	return []string{SL3, Pg, My, MS}
}

// PgAll returns the handles for all postgres versions.
func PgAll() []string {
	return []string{Pg9, Pg10, Pg11, Pg12}
}

// MyAll returns the handles for all MySQL versions.
func MyAll() []string {
	return []string{My56, My57, My8}
}

// MSAll returns the handles for all SQL Server versions.
func MSAll() []string {
	return []string{MS17}
}

// Facts regarding the sakila database.
const (
	TblActor          = "actor"
	TblActorCount     = 200
	TblFilm           = "film"
	TblFilmCount      = 1000
	TblFilmActor      = "film_actor"
	TblFilmActorCount = 5462
	TblPayment        = "payment"
	TblPaymentCount   = 16049

	MillerEmail  = "MARIA.MILLER@sakilacustomer.org"
	MillerCustID = 7
	MillerAddrID = 11
	MillerCityID = 280
)

// Facts regarding the sakila database.
var (
	TblActorCols     = []string{"actor_id", "first_name", "last_name", "last_update"}
	TblActorColKinds = []sqlz.Kind{sqlz.KindInt, sqlz.KindText, sqlz.KindText, sqlz.KindDatetime}
	TblFilmActorCols = []string{"actor_id", "film_id", "last_update"}
	TblPaymentCols   = []string{"payment_id", "customer_id", "staff_id", "rental_id", "amount", "payment_date", "last_update"}
	AllTbls          = []string{"actor", "address", "category", "city", "country", "customer", "film", "film_actor", "film_category", "film_text", "inventory", "language", "payment", "rental", "staff", "store"}

	// AllTblsExceptFilmText exists because our current postgres image is different
	// from the others in that it doesn't have the film_text table.
	// FIXME: delete AllTblsExceptFilmText when postgres image is updated to include film_text.
	AllTblsExceptFilmText = []string{"actor", "address", "category", "city", "country", "customer", "film", "film_actor", "film_category", "inventory", "language", "payment", "rental", "staff", "store"}
)

// URLs for sakila resources.
const (
	URLActorCSV   = "https://sq.io/testdata/actor.csv"
	URLSubsetXLSX = "https://sq.io/testdata/sakila_subset.xlsx"
	URLXLSX       = "https://sq.io/testdata/sakila.xlsx"
)

// Paths for sakila resources.
const (
	PathSL3              = "drivers/sqlite3/testdata/sakila.db"
	PathXLSX             = "drivers/xlsx/testdata/sakila.xlsx"
	PathXLSXSubset       = "drivers/xlsx/testdata/sakila_subset.xlsx"
	PathCSVActor         = "drivers/csv/testdata/sakila-csv/actor.csv"
	PathCSVActorNoHeader = "drivers/csv/testdata/sakila-csv-noheader/actor.csv"
	PathTSVActor         = "drivers/csv/testdata/sakila-tsv/actor.tsv"
)
