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

	RQ = "@sakila_rq" // rqlite

	// Duck is the handle for the DuckDB sakila DB.
	Duck = "@sakila_duck"
	// DuckWhitespace is the handle for the DuckDB sakila DB
	// with whitespace-containing identifiers.
	DuckWhitespace = "@sakila_duck_whitespace"

	// Pg is the handle for the Postgres sakila DB. The engine version is
	// determined by the image the source DSN points at, not by the handle.
	Pg = "@sakila_pg"
	// My is the handle for the MySQL sakila DB.
	My = "@sakila_my"
	// MS is the handle for the SQL Server sakila DB.
	MS = "@sakila_ms"
	// CH is the handle for the ClickHouse sakila DB.
	CH = "@sakila_ch"
	// Ora is the handle for the Oracle sakila DB.
	Ora = "@sakila_or"
)

// AllHandles returns all the typical sakila handles. It does not
// include monotable handles such as @sakila_csv_actor.
func AllHandles() []string {
	return []string{SL3, Duck, Pg, My, MS, CH, Ora, RQ, XLSX}
}

// SQLAll returns all the sakila SQL handles.
func SQLAll() []string {
	return []string{SL3, Duck, Pg, My, MS, CH, Ora, RQ}
}

// SQLAllExternal is the same as SQLAll, but only includes
// external (non-embedded) sources. That is, it excludes SL3 and Duck.
func SQLAllExternal() []string {
	return []string{Pg, My, MS, CH, Ora, RQ}
}

// Embedded returns the embedded SQL handles: SQLite and DuckDB. These run
// in-process (no separate server or container) and are always available,
// unlike the external engines in [SQLAllExternal]. Note that rqlite, though
// SQLite-backed, is external: it is reached over the network.
func Embedded() []string {
	return []string{SL3, Duck}
}

// IsEmbedded reports whether handle is an embedded SQL source (SQLite or
// DuckDB), as opposed to an external engine that needs a running server.
func IsEmbedded(handle string) bool {
	return handle == SL3 || handle == Duck
}

// CrossSourceDests returns the destination handles that origin should be
// paired with in cross-source (origin x dest) tests. Embedded origins
// (SQLite/DuckDB) pair with every engine; external origins pair only with the
// embedded sources plus themselves. This yields {embedded} x {target} coverage
// in both directions, plus same-source self-inserts, while excluding the
// external x external cross pairs: those need multiple external containers live
// at once, grow O(N^2) with the number of SQL engines, and can't run under the
// per-engine CI model. See gh #964.
func CrossSourceDests(origin string) []string {
	if IsEmbedded(origin) {
		return SQLLatest()
	}
	// External origin: embedded dests (both directions of {embedded}x{target})
	// plus origin itself (the single-container same-source insert path).
	return append(Embedded(), origin)
}

// SQLLatest returns the canonical per-engine handles. Retained alongside
// SQLAll for quicker iterative testing; DuckDB is included because it is
// embedded and exercises read-only/access-mode paths the others don't (gh #779).
func SQLLatest() []string {
	return []string{SL3, Duck, Pg, My, MS, CH, Ora}
}

// PgAll returns the postgres handles. Version coverage is a CI matrix
// dimension now, so this is a single handle.
func PgAll() []string { return []string{Pg} }

// MyAll returns the MySQL handles.
func MyAll() []string { return []string{My} }

// MSAll returns the SQL Server handles.
func MSAll() []string { return []string{MS} }

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

	// TblFilmText is present in every sakila dataset. (The Oracle image now
	// includes it too, as a plain table. See sakiladb/oracle schema notes.)
	TblFilmText                = "film_text"
	ViewActorInfo              = "actor_info"
	ViewFilmList               = "film_list"
	ViewNicerButSlowerFilmList = "nicer_but_slower_film_list"

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

// AllTblsViews returns all table AND view names (16 tables + 7 views),
// sorted by name.
func AllTblsViews() []string {
	return []string{
		"actor", "actor_info", "address", "category", "city", "country", "customer", "customer_list", "film",
		"film_actor", "film_category", "film_list", "film_text", "inventory", "language",
		"nicer_but_slower_film_list", "payment", "rental", "sales_by_film_category", "sales_by_store",
		"staff", "staff_list", "store",
	}
}

// URLs for sakila resources.
const (
	ActorCSVURL    = "https://sq.io/testdata/actor.csv"
	ActorCSVSize   = 7641
	ExcelSubsetURL = "https://sq.io/testdata/sakila_subset.xlsx"
	ExcelURL       = "https://sq.io/testdata/sakila.xlsx"
)

// Paths for sakila resources.
const (
	PathSL3              = "drivers/sqlite3/testdata/sakila.db"
	PathDuck             = "drivers/duckdb/testdata/sakila.duckdb"
	PathXLSX             = "drivers/xlsx/testdata/sakila.xlsx"
	PathXLSXSubset       = "drivers/xlsx/testdata/sakila_subset.xlsx"
	PathXLSXActorHeader  = "drivers/xlsx/testdata/actor_header.xlsx"
	PathCSVActor         = "drivers/csv/testdata/sakila-csv/actor.csv"
	PathCSVActorNoHeader = "drivers/csv/testdata/sakila-csv-noheader/actor.csv"
	PathTSVActor         = "drivers/csv/testdata/sakila-tsv/actor.tsv"
	PathTSVActorNoHeader = "drivers/csv/testdata/sakila-tsv-noheader/actor.tsv"
)
