// Command portsakila ports SQLite sakila SQL files to DuckDB-compatible form.
// Run from the repo root:
//
//	go run ./drivers/duckdb/internal/portsakila/cmd/portsakila
//
// Reads from drivers/sqlite3/testdata/sqlite-sakila-*.sql and writes to
// drivers/duckdb/testdata/duckdb-sakila-*.sql.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/neilotoole/sq/drivers/duckdb/internal/portsakila"
)

func main() {
	whitespace := flag.Bool("whitespace", false, "port the whitespace variant")
	flag.Parse()

	srcDir := "drivers/sqlite3/testdata"
	dstDir := "drivers/duckdb/testdata"

	files := []string{
		"sqlite-sakila-schema.sql",
		"sqlite-sakila-insert-data.sql",
		"sqlite-sakila-drop-objects.sql",
		"sqlite-sakila-delete-data.sql",
	}
	if *whitespace {
		// Whitespace variant uses different filenames — see SQLite testdata.
		files = []string{
			"sakila-whitespace-alter.sql",
			"sakila-whitespace-restore.sql",
		}
	}

	for _, f := range files {
		in, err := os.ReadFile(filepath.Join(srcDir, f))
		if err != nil {
			fmt.Fprintln(os.Stderr, "read:", err)
			os.Exit(1)
		}
		out, err := portsakila.Port(string(in))
		if err != nil {
			fmt.Fprintln(os.Stderr, "port:", err)
			os.Exit(1)
		}
		var dstName string
		if *whitespace {
			// Preserve the original filename, just relocate.
			dstName = f
		} else {
			// Replace "sqlite-" prefix with "duckdb-".
			dstName = "duckdb-" + f[len("sqlite-"):]
		}
		dst := filepath.Join(dstDir, dstName)
		if err := os.WriteFile(dst, []byte(out), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "write:", err)
			os.Exit(1)
		}
		fmt.Println("wrote", dst)
	}
}
