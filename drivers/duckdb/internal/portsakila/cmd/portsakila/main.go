// Command portsakila ports SQLite sakila SQL files to DuckDB-compatible form
// and builds all testdata fixtures. Run from the repo root:
//
//	go run ./drivers/duckdb/internal/portsakila/cmd/portsakila
//
// Reads from drivers/sqlite3/testdata/sqlite-sakila-*.sql and writes ported
// SQL and binary .duckdb fixtures to drivers/duckdb/testdata/.
//
// Pass -fixture <name> to build only one fixture; default is "all".
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/duckdb/duckdb-go/v2"

	"github.com/neilotoole/sq/drivers/duckdb/internal/portsakila"
)

const (
	srcDir = "drivers/sqlite3/testdata"
	dstDir = "drivers/duckdb/testdata"
)

func main() {
	which := flag.String("fixture", "all", "which fixture to (re)build (or 'all')")
	flag.Parse()

	ctx := context.Background()

	// Step 1: always port the main sakila SQL files.
	mainFiles := []string{
		"sqlite-sakila-schema.sql",
		"sqlite-sakila-insert-data.sql",
		"sqlite-sakila-drop-objects.sql",
		"sqlite-sakila-delete-data.sql",
	}
	for _, f := range mainFiles {
		if err := portFile(f, "duckdb-"+f[len("sqlite-"):]); err != nil {
			fmt.Fprintln(os.Stderr, "port:", err)
			os.Exit(1)
		}
	}

	// Step 2: port the whitespace variant SQL files.
	whitespaceFiles := []string{
		"sakila-whitespace-alter.sql",
		"sakila-whitespace-restore.sql",
	}
	for _, f := range whitespaceFiles {
		if err := portFile(f, f); err != nil {
			fmt.Fprintln(os.Stderr, "port whitespace:", err)
			os.Exit(1)
		}
	}

	// Fixture registry in deterministic order.
	type fixtureEntry struct {
		build func(context.Context) error
		name  string
	}
	fixtures := []fixtureEntry{
		{name: "sakila", build: buildSakila},
		{name: "sakila-whitespace", build: buildSakilaWhitespace},
		{name: "sakila_diff", build: buildSakilaDiff},
		{name: "empty", build: buildEmpty},
		{name: "misc", build: buildMisc},
		{name: "blob", build: buildBlob},
	}

	for _, fx := range fixtures {
		if *which != "all" && *which != fx.name {
			continue
		}
		if err := fx.build(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "fixture %s: %v\n", fx.name, err)
			os.Exit(1)
		}
	}
}

// portFile reads a SQL file from srcDir, runs it through the appropriate
// portsakila entrypoint (chosen by filename), and writes the result to
// dstDir under dstName. Schema/insert-data/drop-objects files need
// topological reordering to satisfy DuckDB's at-CREATE-and-at-INSERT
// FK enforcement; other files just need the generic transforms.
func portFile(srcName, dstName string) error {
	in, err := os.ReadFile(filepath.Join(srcDir, srcName))
	if err != nil {
		return fmt.Errorf("read %s: %w", srcName, err)
	}
	var portFn func(string) (string, error)
	switch {
	case strings.Contains(srcName, "schema"):
		portFn = portsakila.PortSchema
	case strings.Contains(srcName, "insert-data"):
		portFn = portsakila.PortInsertData
	case strings.Contains(srcName, "drop-objects"):
		portFn = portsakila.PortDropObjects
	default:
		portFn = portsakila.Port
	}
	out, err := portFn(string(in))
	if err != nil {
		return fmt.Errorf("port %s: %w", srcName, err)
	}
	dst := filepath.Join(dstDir, dstName)
	if err := os.WriteFile(dst, []byte(out), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	fmt.Fprintln(os.Stdout, "wrote", dst)
	return nil
}

// openFresh removes any existing DuckDB file at dbPath and opens a fresh one.
func openFresh(dbPath string) (*sql.DB, error) {
	if err := os.RemoveAll(dbPath); err != nil {
		return nil, fmt.Errorf("remove existing db: %w", err)
	}
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open duckdb %s: %w", dbPath, err)
	}
	return db, nil
}

// execFile reads a SQL file and executes its contents against db.
func execFile(ctx context.Context, db *sql.DB, path string) error {
	buf, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if _, err := db.ExecContext(ctx, string(buf)); err != nil {
		return fmt.Errorf("exec %s: %w", path, err)
	}
	fmt.Fprintln(os.Stdout, "executed", path)
	return nil
}

// buildSakila builds drivers/duckdb/testdata/sakila.duckdb from the ported
// schema and insert-data SQL files.
func buildSakila(ctx context.Context) error {
	dbPath := filepath.Join(dstDir, "sakila.duckdb")
	db, err := openFresh(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	for _, f := range []string{
		filepath.Join(dstDir, "duckdb-sakila-schema.sql"),
		filepath.Join(dstDir, "duckdb-sakila-insert-data.sql"),
	} {
		if err := execFile(ctx, db, f); err != nil {
			return err
		}
	}
	fmt.Fprintln(os.Stdout, "wrote", dbPath)
	return nil
}

// buildSakilaWhitespace builds sakila-whitespace.duckdb: the full sakila
// schema+data, then the whitespace-alter SQL applied on top.
//
// The whitespace-alter renames actor.first_name → "first name",
// actor.last_name → "last name", and film_actor → "film actor". DuckDB
// rejects ALTER on a table that has any dependent constraint (including
// incoming FKs), so this build path uses a no-FK variant of the schema —
// the main sakila.duckdb fixture keeps its 21 FKs. We must also:
//  1. Drop the film_list view (references actor.first_name/last_name and film_actor).
//  2. Drop idx_actor_last_name (index on actor.last_name).
//  3. Drop idx_fk_film_actor_actor and idx_fk_film_actor_film (indexes on film_actor).
//  4. Apply the whitespace-alter SQL.
//  5. Recreate the dropped indexes with updated table/column names.
//  6. Recreate film_list with updated identifiers.
func buildSakilaWhitespace(ctx context.Context) error {
	dbPath := filepath.Join(dstDir, "sakila-whitespace.duckdb")
	db, err := openFresh(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Re-port the source schema WITHOUT FKs for this build path. The
	// schema-with-FKs file on disk would block the renames below.
	schemaIn, err := os.ReadFile(filepath.Join(srcDir, "sqlite-sakila-schema.sql"))
	if err != nil {
		return fmt.Errorf("read source schema: %w", err)
	}
	schemaSQL, err := portsakila.PortSchemaWithoutFKs(string(schemaIn))
	if err != nil {
		return fmt.Errorf("port schema (no FKs): %w", err)
	}
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("exec schema (no FKs): %w", err)
	}
	if err := execFile(ctx, db, filepath.Join(dstDir, "duckdb-sakila-insert-data.sql")); err != nil {
		return err
	}

	// Drop dependents before rename.
	dropStmts := []string{
		`DROP VIEW IF EXISTS film_list`,
		`DROP INDEX IF EXISTS idx_actor_last_name`,
		`DROP INDEX IF EXISTS idx_fk_film_actor_actor`,
		`DROP INDEX IF EXISTS idx_fk_film_actor_film`,
	}
	for _, stmt := range dropStmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("drop dependent (%s): %w", stmt, err)
		}
	}

	if err := execFile(ctx, db, filepath.Join(dstDir, "sakila-whitespace-alter.sql")); err != nil {
		return err
	}

	// Recreate indexes with updated names/columns.
	recreateStmts := []string{
		`CREATE INDEX idx_actor_last_name ON actor("last name")`,
		`CREATE INDEX idx_fk_film_actor_actor ON "film actor"(actor_id)`,
		`CREATE INDEX idx_fk_film_actor_film ON "film actor"(film_id)`,
		// Recreate film_list view with updated identifiers.
		`CREATE VIEW film_list AS
SELECT film.film_id AS FID,
       film.title AS title,
       film.description AS description,
       category.name AS category,
       film.rental_rate AS price,
       film.length AS length,
       film.rating AS rating,
       actor."first name"||' '||actor."last name" AS actors
FROM category
  LEFT JOIN film_category ON category.category_id = film_category.category_id
  LEFT JOIN film ON film_category.film_id = film.film_id
  JOIN "film actor" ON film.film_id = "film actor".film_id
  JOIN actor ON "film actor".actor_id = actor.actor_id`,
	}
	for _, stmt := range recreateStmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("recreate dependent: %w\nSQL: %s", err, stmt)
		}
	}
	fmt.Fprintln(os.Stdout, "wrote", dbPath)
	return nil
}

// buildSakilaDiff copies sakila.duckdb to sakila_diff.duckdb then applies a
// one-row mutation so that sq diff tests can detect exactly one row difference.
func buildSakilaDiff(ctx context.Context) error {
	srcPath := filepath.Join(dstDir, "sakila.duckdb")
	dstPath := filepath.Join(dstDir, "sakila_diff.duckdb")

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", srcPath, err)
	}
	err = os.WriteFile(dstPath, data, 0o600) //nolint:gosec // G703: constant fixture path in dev-only tool
	if err != nil {
		return fmt.Errorf("write %s: %w", dstPath, err)
	}

	db, err := sql.Open("duckdb", dstPath)
	if err != nil {
		return fmt.Errorf("open duckdb %s: %w", dstPath, err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, `UPDATE actor SET first_name = 'CHANGED' WHERE actor_id = 1`); err != nil {
		return fmt.Errorf("mutate sakila_diff: %w", err)
	}
	fmt.Fprintln(os.Stdout, "wrote", dstPath)
	return nil
}

// buildEmpty creates an empty (schemaless) DuckDB file.
func buildEmpty(ctx context.Context) error {
	dbPath := filepath.Join(dstDir, "empty.duckdb")
	db, err := openFresh(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Force file creation.
	if _, err := db.ExecContext(ctx, `SELECT 1`); err != nil {
		return fmt.Errorf("select 1: %w", err)
	}
	fmt.Fprintln(os.Stdout, "wrote", dbPath)
	return nil
}

// buildMisc creates misc.duckdb with two schemas (foo, bar) and simple tables.
func buildMisc(ctx context.Context) error {
	dbPath := filepath.Join(dstDir, "misc.duckdb")
	db, err := openFresh(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	ddl := `
CREATE SCHEMA foo;
CREATE TABLE foo.t1 (id INTEGER, name VARCHAR);
INSERT INTO foo.t1 VALUES (1, 'one'), (2, 'two');
CREATE SCHEMA bar;
CREATE TABLE bar.t2 (x DOUBLE);
INSERT INTO bar.t2 VALUES (1.5);
`
	if _, err := db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("create misc schema: %w", err)
	}
	fmt.Fprintln(os.Stdout, "wrote", dbPath)
	return nil
}

// buildBlob creates blob.duckdb with a single table containing a BLOB column.
func buildBlob(ctx context.Context) error {
	dbPath := filepath.Join(dstDir, "blob.duckdb")
	db, err := openFresh(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	ddl := `
CREATE TABLE blobs (id INTEGER PRIMARY KEY, data BLOB);
INSERT INTO blobs VALUES (1, '\x00\x01\x02\x03'::BLOB), (2, NULL);
`
	if _, err := db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("create blobs table: %w", err)
	}
	fmt.Fprintln(os.Stdout, "wrote", dbPath)
	return nil
}
