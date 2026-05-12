// Command portsakila ports SQLite sakila SQL files to DuckDB-compatible form.
// Run from the repo root:
//
//	go run ./drivers/duckdb/internal/portsakila/cmd/portsakila
//
// Reads from drivers/sqlite3/testdata/sqlite-sakila-*.sql and writes to
// drivers/duckdb/testdata/duckdb-sakila-*.sql.
//
// Pass -build to also create drivers/duckdb/testdata/sakila.duckdb in-process
// via the bundled github.com/duckdb/duckdb-go/v2 binding. No external duckdb
// CLI is required.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/neilotoole/sq/drivers/duckdb/internal/portsakila"
)

func main() {
	whitespace := flag.Bool("whitespace", false, "port the whitespace variant")
	buildFlag := flag.Bool("build", false, "after porting, also build the .duckdb binary fixture")
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
		if err := os.WriteFile(dst, []byte(out), 0o600); err != nil {
			fmt.Fprintln(os.Stderr, "write:", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, "wrote", dst)
	}

	if *buildFlag && !*whitespace {
		dbPath := filepath.Join(dstDir, "sakila.duckdb")
		sqlFiles := []string{
			filepath.Join(dstDir, "duckdb-sakila-schema.sql"),
			filepath.Join(dstDir, "duckdb-sakila-insert-data.sql"),
		}
		if err := build(context.Background(), dbPath, sqlFiles); err != nil {
			fmt.Fprintln(os.Stderr, "build:", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, "wrote", dbPath)
	}
}

// build removes any existing DuckDB file at dbPath, opens a fresh one, and
// executes each SQL file in order.
func build(ctx context.Context, dbPath string, sqlFiles []string) error {
	if err := os.RemoveAll(dbPath); err != nil {
		return fmt.Errorf("remove existing db: %w", err)
	}
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return fmt.Errorf("open duckdb: %w", err)
	}
	defer db.Close()

	for _, f := range sqlFiles {
		buf, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		if _, err := db.ExecContext(ctx, string(buf)); err != nil {
			return fmt.Errorf("exec %s: %w", f, err)
		}
		fmt.Fprintln(os.Stdout, "executed", f)
	}
	return nil
}
