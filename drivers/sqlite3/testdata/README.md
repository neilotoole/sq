# SQLite testdata

## sakila.db

[`sakila.db`](./sakila.db) contains the standard Sakila dataset. It can be regenerated
from the `sqlite-sakila-X.sql` SQL scripts
using [`recreate_sakila_sqlite.sh`](./recreate_sakila_sqlite.sh).

It is also the canonical source for the other drivers' Sakila fixtures: the DuckDB
fixtures are ported from it (see
[`drivers/duckdb/testdata`](../../duckdb/testdata/README.md)), and the XLSX workbooks
are generated from it by `genxlsx`.

## sakila_diff.db

[`sakila_diff.db`](./sakila_diff.db) is a lightly modified variant of `sakila.db`,
for use with test `sq diff`.

- The `actor` table is missing the second row.
  ```sql
  DELETE FROM actor WHERE actor_id=2;
  ```
- There's a new table `awards`.

## sakila_whitespace.db

[`sakila_whitespace.db`](./sakila_whitespace.db) contains a mutated Sakila
schema, with some table and column names changed. This is to facilitate
testing of `sq`'s ability to support such names. The mutated DB is achieved by
applying [`sakila-whitespace-alter.sql`](./sakila-whitespace-alter.sql) to
`sakila.db`. The changes can be reversed with
[`sakila-whitespace-restore.sql`](./sakila-whitespace-restore.sql).

Those two SQL scripts are also consumed by the DuckDB port tool, which builds its
own `sakila-whitespace.duckdb` from them.

## sakila_fts5.db

[`sakila_fts5.db`](./sakila_fts5.db) is based off [`sakila.db`](./sakila.db), but
contains an FTS5 virtual table `actor_fts`. This table was created via the statement:

```sql
CREATE VIRTUAL TABLE actor_fts
USING fts5(actor_id, first_name, last_name, last_update, content='actor', content_rowid='actor_id');
```

## sakila_db

[`sakila_db`](./sakila_db) is a copy of `sakila.db` with the file extension removed,
to verify that type detection identifies a SQLite file by its contents rather than
its name. Used by `libsq/files`.

## Non-Sakila fixtures

| File | Description |
|------|-------------|
| [`misc.db`](./misc.db) | Assorted small tables, for general-purpose tests. Registered as a source in `testh/testdata/test.sq.yml`. |
| [`empty.db`](./empty.db) | Valid SQLite file with no user tables, for empty-source edge cases. Registered in `test.sq.yml`. |
| [`blob.db`](./blob.db) | BLOB-typed data, for byte round-trip tests. Registered in `test.sq.yml`; the path is also `testh/fixt.BlobDBPath`. |
| [`type_test.ddl`](./type_test.ddl) | SQL-only file (not a binary DB). Defines and populates a `type_test` table exercising the SQLite types the driver maps to a `kind.Kind`. Executed by `db_type_test.go`. |
