# Sakila SQLite Test Data

## sakila.db

[`sakila.db`](./sakila.db) contains the standard Sakila dataset. It can be regenerated
from the `sqlite-sakila-X.sql` SQL scripts
using [`recreate_sakila_sqlite.sh`](./recreate_sakila_sqlite.sh%60).

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
[`sakila-whitespace-restore.sql](./sakila-whitespace-restore.sql).

## sakila_fts5.db

[`sakila_fts5.db`](./sakila_fts5.db) is based off [`sakila.db`](./sakila.db), but
contains an FTS5 virtual table `actor_fts`. This table was created via the statement:

```sql
CREATE VIRTUAL TABLE actor_fts
USING fts5(actor_id, first_name, last_name, last_update, content='actor', content_rowid='actor_id');
```
