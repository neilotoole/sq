# Sakila SQLite Test Data

## sakila.db

[`sakila.db`](./sakila.db) contains the standard Sakila dataset. It can be regenerated
from the `sqlite-sakila-X.sql` SQL scripts
using [`recreate_sakila_sqlite.sh`](./recreate_sakila_sqlite.sh%60).

## sakila-whitespace.db

[`sakila-whitespace.db`](./sakila-whitespace.db) contains a mutated Sakila
schema, with some table and column names changed. This is to facilitate
testing of `sq`'s ability to support such names. The mutated DB is achieved by
applying [`sakila-whitespace-alter.sql`](./sakila-whitespace-alter.sql) to
`sakila.db`. The changes can be reversed with
[`sakila-whitespace-restore.sql](./sakila-whitespace-restore.sql).
