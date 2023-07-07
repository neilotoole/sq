// Package sakila holds test constants and such for the sakila test sources.
package sakila

import (
	"github.com/neilotoole/sq/libsq/core/kind"
)

// Sakila source handles.
const (
	XLSX             = "@sakila_xlsx"
	XLSXSubset       = "@sakila_xlsx_subset"
	XLSXNoHeader     = "@sakila_xlsx_noheader"
	CSVActor         = "@sakila_csv_actor"
	CSVAddress       = "@sakila_csv_address"
	CSVActorHTTP     = "@sakila_csv_actor_http"
	CSVActorNoHeader = "@sakila_csv_actor_noheader"
	TSVActor         = "@sakila_tsv_actor"
	TSVActorNoHeader = "@sakila_tsv_actor_noheader"
	SL3              = "@sakila_sl3"
	SL3Whitespace    = "@sakila_sl3_whitespace"
	Pg9              = "@sakila_pg9"
	Pg10             = "@sakila_pg10"
	Pg11             = "@sakila_pg11"
	Pg12             = "@sakila_pg12"
	Pg               = Pg12
	My56             = "@sakila_my56" // TODO: rename to @sakila_my5_6
	My57             = "@sakila_my57" // TODO: rename to @sakila_my5_7
	My8              = "@sakila_my8"
	My               = My8
	MS17             = "@sakila_ms17"
	MS19             = "@sakila_ms19"
	MS               = MS19
)

// AllHandles returns all the typical sakila handles. It does not
// include monotable handles such as @sakila_csv_actor.
func AllHandles() []string {
	return []string{
		SL3,
		Pg9,
		// Pg10,
		// Pg11,
		Pg12,
		My56,
		My57,
		My8,
		// MS17,
		MS19,
		XLSX,
	}
}

// SQLAll returns all the sakila SQL handles.
func SQLAll() []string {
	return []string{
		SL3,
		Pg9,
		// Pg10,
		// Pg11,
		Pg12,
		My56,
		My57,
		My8,
		// MS17,
		MS19,
	}
}

// SQLAllExternal is the same as SQLAll, but only includes
// external (non-embedded) sources. That is, it excludes SL3.
func SQLAllExternal() []string {
	return []string{
		Pg9,
		// Pg10,
		// Pg11,
		Pg12,
		My56,
		My57,
		My8,
		// MS17,
		MS19,
	}
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
	return []string{
		Pg9,
		// Pg10,
		// Pg11,
		Pg12,
	}
}

// MyAll returns the handles for all MySQL versions.
func MyAll() []string {
	return []string{My56, My57, My8}
}

// MSAll returns the handles for all SQL Server versions.
func MSAll() []string {
	return []string{
		// MS17,
		MS19,
	}
}

// Facts regarding the sakila database.
const (
	TblActor          = "actor"
	TblActorCount     = 200
	TblAddress        = "address"
	TblAddressCount   = 603
	TblFilm           = "film"
	TblFilmCount      = 1000
	TblFilmActor      = "film_actor"
	TblFilmActorCount = 5462
	TblPayment        = "payment"
	TblPaymentCount   = 16049

	// TblFilmText is present in each sakila dataset, except Postgres for
	// some reason.
	TblFilmText   = "film_text"
	ViewActorInfo = "actor_info"
	ViewFilmList  = "film_list"

	MillerEmail  = "MARIA.MILLER@sakilacustomer.org"
	MillerCustID = 7
	MillerAddrID = 11
	MillerCityID = 280
)

// TblActorCols returns table "actor" column names.
func TblActorCols() []string {
	return []string{"actor_id", "first_name", "last_name", "last_update"}
}

// TblActorColKinds returns table "actor" column kinds.
func TblActorColKinds() []kind.Kind {
	return []kind.Kind{kind.Int, kind.Text, kind.Text, kind.Datetime}
}

// TblAddressCols returns table "address" column names.
func TblAddressCols() []string {
	return []string{
		"address_id",
		"address",
		"address2",
		"district",
		"city_id",
		"postal_code",
		"phone",
		"last_update",
	}
}

// TblAddressColKinds returns table "address" column kinds.
func TblAddressColKinds() []kind.Kind {
	return []kind.Kind{
		kind.Int,      // address_id
		kind.Text,     // address
		kind.Text,     // address2
		kind.Text,     // district
		kind.Int,      // city_id
		kind.Int,      // postal_code
		kind.Text,     // phone
		kind.Datetime, // last_update
	}
}

// TblFilmActorCols returns table "film" column names.
func TblFilmActorCols() []string {
	return []string{"actor_id", "film_id", "last_update"}
}

// TblFilmActorColKinds returns table "film_actor" column kinds.
func TblFilmActorColKinds() []kind.Kind {
	return []kind.Kind{kind.Int, kind.Int, kind.Datetime}
}

// TblPaymentCols returns table "payment" column names.
func TblPaymentCols() []string {
	return []string{"payment_id", "customer_id", "staff_id", "rental_id", "amount", "payment_date", "last_update"}
}

// TblPaymentColKinds returns table "payment" column kinds.
func TblPaymentColKinds() []kind.Kind {
	return []kind.Kind{kind.Int, kind.Int, kind.Int, kind.Int, kind.Decimal, kind.Datetime, kind.Datetime}
}

// AllTbls returns all table names.
func AllTbls() []string {
	return []string{
		"actor", "address", "category", "city", "country", "customer", "film", "film_actor",
		"film_category", "film_text", "inventory", "language", "payment", "rental", "staff", "store",
	}
}

// AllTblsViews returns all table AND view names.
func AllTblsViews() []string {
	return []string{
		"actor", "address", "category", "city", "country", "customer", "customer_list", "film",
		"film_actor", "film_category", "film_list", "film_text", "inventory", "language", "payment", "rental",
		"sales_by_film_category", "sales_by_store", "staff", "staff_list", "store",
	}
}

// AllTblsExceptFilmText exists because our current postgres image is different
// from the others in that it doesn't have the film_text table.
func AllTblsExceptFilmText() []string {
	// TODO: delete AllTblsExceptFilmText when postgres image is updated to include film_text.
	return []string{
		"actor", "address", "category", "city", "country", "customer", "film", "film_actor",
		"film_category", "inventory", "language", "payment", "rental", "staff", "store",
	}
}

// URLs for sakila resources.
const (
	URLActorCSV   = "https://sq.io/testdata/actor.csv"
	URLSubsetXLSX = "https://sq.io/testdata/sakila_subset.xlsx"
	URLXLSX       = "https://sq.io/testdata/sakila.xlsx"
)

// Paths for sakila resources.
const (
	PathSL3              = "drivers/sqlite3/testdata/sakila.db"
	PathSL3Whitespace    = "drivers/sqlite3/testdata/sakila-whitespace.db"
	PathXLSX             = "drivers/xlsx/testdata/sakila.xlsx"
	PathXLSXSubset       = "drivers/xlsx/testdata/sakila_subset.xlsx"
	PathXLSXActorHeader  = "drivers/xlsx/testdata/actor_header.xlsx"
	PathCSVActor         = "drivers/csv/testdata/sakila-csv/actor.csv"
	PathCSVActorNoHeader = "drivers/csv/testdata/sakila-csv-noheader/actor.csv"
	PathTSVActor         = "drivers/csv/testdata/sakila-tsv/actor.tsv"
	PathTSVActorNoHeader = "drivers/csv/testdata/sakila-tsv-noheader/actor.tsv"
)
