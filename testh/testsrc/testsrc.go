// Package testsrc holds testing constants (in addition
// to pkg sakila).
package testsrc

// Handles for various test data sources.
const (
	CSVPerson         = "@csv_person"
	CSVPersonBig      = "@csv_person_big"
	CSVPersonNoHeader = "@csv_person_noheader"

	// PplUD is the handle of a user-defined "people" source.
	PplUD = "@ud_ppl"

	// RSSNYTLocalUD is the handle of a user-defined RSS source.
	RSSNYTLocalUD = "@ud_rss_nytimes_local"

	// MiscDB is the handle of a SQLite DB with misc testing data.
	MiscDB = "@miscdb"
)

const (
	// TblTypes is a table in MiscDB.
	TblTypes = "tbl_types"
)

// Paths for various testdata.
const (
	// PathSrcsConfig is the path of the yml file containing
	// the test sources template config file.
	PathSrcsConfig = "/testh/testdata/sources.sq.yml"

	PathDriverDefPpl = "drivers/userdriver/xmlud/testdata/ppl.sq.yml"
	PathDriverDefRSS = "drivers/userdriver/xmlud/testdata/rss.sq.yml"

	PathXLSXTestHeader = "drivers/xlsx/testdata/test_header.xlsx"
)
