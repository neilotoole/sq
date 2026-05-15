# DuckDB testdata

`sakila.duckdb` and friends are generated from the **SQLite** sakila SQL
files in `drivers/sqlite3/testdata/`, ported via the in-tree tool at
`drivers/duckdb/internal/portsakila/`.

## Additional fixtures

| File | Description |
|------|-------------|
| `sakila-whitespace.duckdb` | Full sakila schema+data with three identifiers renamed to include whitespace: `actor.first_name` → `"first name"`, `actor.last_name` → `"last name"`, `film_actor` → `"film actor"`. Used to verify driver identifier-quoting. |
| `sakila_diff.duckdb` | Copy of `sakila.duckdb` with one row mutated (`actor.first_name = 'CHANGED'` for `actor_id = 1`). Used by `sq diff` tests that expect exactly one row difference. |
| `empty.duckdb` | Valid DuckDB file with no user schema. Used for empty-source edge-case tests. |
| `misc.duckdb` | Two schemas (`foo`, `bar`) with simple tables. `foo.t1` has two rows; `bar.t2` has one. Used for multi-schema inspection tests. |
| `blob.duckdb` | Single `blobs(id INTEGER, data BLOB)` table with two rows (one NULL blob). Used for BLOB type-mapping tests. |
| `type_test.ddl` | SQL-only file (not a binary DB). Defines and populates a `type_test` table exercising every DuckDB type the driver maps to a `kind.Kind`. Executed by the type-mapping tests in `db_type_test.go` and `value_test.go`. |

## Regenerating

To rebuild all fixtures from scratch:

```bash
go run ./drivers/duckdb/internal/portsakila/cmd/portsakila
```

To rebuild only one fixture:

```bash
go run ./drivers/duckdb/internal/portsakila/cmd/portsakila -fixture sakila-whitespace
```

Available fixture names: `sakila`, `sakila-whitespace`, `sakila_diff`, `empty`,
`misc`, `blob`, `type_test`.

## Divergences from SQLite sakila

- **No triggers.** DuckDB has no TRIGGER support. The ~30 `last_update`
  triggers are stripped during port. `last_update` columns are populated
  at insert time via `DEFAULT current_timestamp`; they will not auto-update
  on UPDATEs (sq's read-mostly tests don't exercise this).
- **No `AUTOINCREMENT`.** DuckDB has no such keyword. INSERT data already
  supplies all primary-key IDs explicitly.
- **21 of 22 FOREIGN KEY constraints preserved.** The only omission is
  `fk_store_staff` (store → staff): the store/staff cycle in sakila
  cannot be represented in DuckDB because both `staff.store_id` and
  `store.manager_staff_id` are `NOT NULL`, DuckDB enforces FKs at INSERT
  time, and there is no `SET foreign_keys = off` or
  `ALTER TABLE ADD FOREIGN KEY` escape hatch. Dropping the back-edge
  of the cycle (and reordering CREATE TABLE and INSERT statements
  topologically) keeps every other constraint intact. `ON DELETE
  CASCADE`, `SET NULL`, and `SET DEFAULT` clauses are stripped from FKs
  too (DuckDB only supports the default `NO ACTION`); sq's read-mostly
  tests do not exercise cascade behaviour, so this is invisible in
  practice. The `sakila-whitespace.duckdb` variant strips FKs entirely
  because DuckDB rejects `ALTER` on tables with dependent constraints.
- **No `BLOB SUB_TYPE TEXT`.** This Firebird-heritage type alias is
  replaced with plain `TEXT` during port.
