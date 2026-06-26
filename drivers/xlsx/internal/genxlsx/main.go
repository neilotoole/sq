// Command genxlsx regenerates the Sakila XLSX test fixtures from the canonical
// SQLite fixture (drivers/sqlite3/testdata/sakila.db), so they carry the same
// faithful data (restored accents, real phone numbers, original 2006
// timestamps) as the rest of the Sakila fixtures. Run from the repo root:
//
//	go run ./drivers/xlsx/internal/genxlsx
//
// It writes three workbooks to drivers/xlsx/testdata/:
//
//	sakila.xlsx           16 sheets (one per table), with a header row
//	sakila_noheader.xlsx  the same 16 sheets, without header rows
//	sakila_subset.xlsx    5 sheets (actor, category, film, film_actor, language)
//
// Cell encoding mirrors the original fixtures: integer/decimal columns are
// written as numeric cells, TIMESTAMP columns as ISO-8601 strings
// ("2006-02-15T04:34:33Z"), everything else as text; NULLs are omitted.
//
// The exotic format variants under testdata/file_formats/ (.xlam/.xlsm/.xltm/
// .xltx/.strict_openxml.xlsx) are NOT regenerated here — they exist only to
// exercise format detection and cannot be produced by this tool.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/xuri/excelize/v2"
)

const (
	srcDB  = "drivers/sqlite3/testdata/sakila.db"
	dstDir = "drivers/xlsx/testdata"
)

// fullTables is the sheet order of sakila.xlsx / sakila_noheader.xlsx.
var fullTables = []string{
	"actor", "address", "category", "city", "country", "customer",
	"film", "film_actor", "film_category", "film_text", "inventory",
	"language", "payment", "rental", "staff", "store",
}

// subsetTables is the sheet set of sakila_subset.xlsx.
var subsetTables = []string{"actor", "category", "film", "film_actor", "language"}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "genxlsx:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", srcDB)
	if err != nil {
		return err
	}
	defer db.Close()

	specs := []struct {
		file      string
		tables    []string
		hasHeader bool
	}{
		{"sakila.xlsx", fullTables, true},
		{"sakila_noheader.xlsx", fullTables, false},
		{"sakila_subset.xlsx", subsetTables, true},
	}

	for _, s := range specs {
		if err := buildWorkbook(ctx, db, s.file, s.tables, s.hasHeader); err != nil {
			return fmt.Errorf("%s: %w", s.file, err)
		}
		fmt.Fprintf(os.Stdout, "wrote %s/%s (%d sheets, header=%v)\n", dstDir, s.file, len(s.tables), s.hasHeader)
	}
	return nil
}

type column struct {
	name      string
	numeric   bool
	decimal   bool
	timestamp bool
}

func classify(declType string) column {
	t := strings.ToUpper(declType)
	c := column{}
	switch {
	case strings.HasPrefix(t, "TIMESTAMP"), strings.HasPrefix(t, "DATETIME"), strings.HasPrefix(t, "DATE"):
		c.timestamp = true
	case strings.HasPrefix(t, "DECIMAL"), strings.HasPrefix(t, "NUMERIC"),
		strings.HasPrefix(t, "REAL"), strings.HasPrefix(t, "FLOAT"), strings.HasPrefix(t, "DOUBLE"):
		c.numeric = true
		c.decimal = true
	case strings.HasPrefix(t, "INT"), strings.HasPrefix(t, "SMALLINT"),
		strings.HasPrefix(t, "BIGINT"), strings.HasPrefix(t, "TINYINT"):
		c.numeric = true
	}
	return c
}

func tableColumns(ctx context.Context, db *sql.DB, table string) ([]column, error) {
	// table is always a hardcoded fixture name (fullTables/subsetTables), never
	// user input, so the concatenation is safe.
	q := "SELECT name, type FROM pragma_table_info('" + table + "')" //nolint:gosec // G202: hardcoded name
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []column
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			return nil, err
		}
		c := classify(typ)
		c.name = name
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// isoDatetime converts a SQLite datetime text ("2006-02-15 04:34:33.000") to the
// ISO-8601 form the fixtures use ("2006-02-15T04:34:33Z"). Inputs that don't at
// least carry a full date are returned unchanged rather than mangled into a
// bogus "…Z" string (every Sakila TIMESTAMP column is a full datetime, so this
// only guards against future/other schemas).
func isoDatetime(s string) string {
	if len(s) < len("2006-02-15") {
		return s
	}
	if len(s) >= 19 {
		s = s[:19]
	}
	s = strings.Replace(s, " ", "T", 1)
	if !strings.HasSuffix(s, "Z") {
		s += "Z"
	}
	return s
}

func buildWorkbook(ctx context.Context, db *sql.DB, file string, tables []string, hasHeader bool) error {
	f := excelize.NewFile()
	defer f.Close()

	for _, table := range tables {
		cols, err := tableColumns(ctx, db, table)
		if err != nil {
			return err
		}
		if _, err = f.NewSheet(table); err != nil {
			return err
		}

		colNames := make([]string, len(cols))
		for j, c := range cols {
			colNames[j] = c.name
		}
		// table + column names are hardcoded fixture identifiers, not user input.
		query := "SELECT " + strings.Join(colNames, ", ") + " FROM " + table //nolint:gosec // G202: hardcoded
		dataRows, err := db.QueryContext(ctx, query)
		if err != nil {
			return err
		}

		rowIdx := 1
		if hasHeader {
			for j, c := range cols {
				cell, _ := excelize.CoordinatesToCellName(j+1, rowIdx)
				if err = f.SetCellStr(table, cell, c.name); err != nil {
					dataRows.Close()
					return err
				}
			}
			rowIdx++
		}

		vals := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for j := range vals {
			ptrs[j] = &vals[j]
		}

		for dataRows.Next() {
			if err = dataRows.Scan(ptrs...); err != nil {
				dataRows.Close()
				return err
			}
			for j, c := range cols {
				if !vals[j].Valid {
					continue // NULL: omit the cell
				}
				cell, _ := excelize.CoordinatesToCellName(j+1, rowIdx)
				if err = writeCell(f, table, cell, c, vals[j].String); err != nil {
					dataRows.Close()
					return err
				}
			}
			rowIdx++
		}
		if err = dataRows.Err(); err != nil {
			dataRows.Close()
			return err
		}
		dataRows.Close()
	}

	// Drop the default empty sheet that NewFile creates.
	if idx, _ := f.GetSheetIndex("Sheet1"); idx >= 0 {
		if err := f.DeleteSheet("Sheet1"); err != nil {
			return err
		}
	}
	// Make the first data sheet active.
	if idx, _ := f.GetSheetIndex(tables[0]); idx >= 0 {
		f.SetActiveSheet(idx)
	}

	return f.SaveAs(dstDir + "/" + file)
}

func writeCell(f *excelize.File, sheet, cell string, c column, v string) error {
	switch {
	case c.timestamp:
		return f.SetCellStr(sheet, cell, isoDatetime(v))
	case c.decimal:
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			// Fail loudly: silently writing a string cell would flip the
			// column's inferred kind (Decimal -> Text) and break the fixture.
			return fmt.Errorf("%s.%s: parse decimal %q: %w", sheet, c.name, v, err)
		}
		return f.SetCellValue(sheet, cell, n)
	case c.numeric:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("%s.%s: parse int %q: %w", sheet, c.name, v, err)
		}
		return f.SetCellValue(sheet, cell, n)
	default:
		return f.SetCellStr(sheet, cell, v)
	}
}
