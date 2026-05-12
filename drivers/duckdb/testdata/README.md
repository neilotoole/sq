# DuckDB testdata

`sakila.duckdb` and friends are generated from the **SQLite** sakila SQL
files in `drivers/sqlite3/testdata/`, ported via the in-tree tool at
`drivers/duckdb/internal/portsakila/`.

## Regenerating

1. (If needed) update `drivers/sqlite3/testdata/sqlite-sakila-*.sql`.
2. From the repo root, run:

   ```bash
   go run ./drivers/duckdb/internal/portsakila/cmd/portsakila -build
   ```

   This rewrites the `duckdb-sakila-*.sql` files AND rebuilds `sakila.duckdb`
   in-process via the bundled `github.com/duckdb/duckdb-go/v2` binding. No
   external `duckdb` CLI required; the libduckdb version that builds the
   fixture is guaranteed to match the version used by the test suite.

## Divergences from SQLite sakila

- **No triggers.** DuckDB has no TRIGGER support. The ~30 `last_update`
  triggers are stripped during port. `last_update` columns are populated
  at insert time via `DEFAULT current_timestamp`; they will not auto-update
  on UPDATEs (sq's read-mostly tests don't exercise this).
- **No `AUTOINCREMENT`.** DuckDB has no such keyword. INSERT data already
  supplies all primary-key IDs explicitly.
- **No FOREIGN KEY constraints.** DuckDB validates referenced tables at
  `CREATE TABLE` time. The sakila schema has circular references between
  `store`, `staff`, and `customer` that cannot be resolved by reordering.
  FK constraint lines are stripped during port. Referential integrity is
  preserved by the insert-data ordering; sq's read-mostly tests do not
  exercise FK enforcement.
- **No `BLOB SUB_TYPE TEXT`.** This Firebird-heritage type alias is
  replaced with plain `TEXT` during port.
