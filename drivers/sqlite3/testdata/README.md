# Sakila SQLite Test Data

- [`sakila.db`](./sakila.db) contains the standard Sakila dataset. It can be regenerated
  from the `sqlite-sakila-X.sql` SQL scripts
  using [`recreate_sakila_sqlite.sh`](./recreate_sakila_sqlite.sh`).
- [`sakila-whitespace-names.db`](./sakila-whitespace-names.db) contains a mutated Sakila
  schema, with some table and column names changed. This is to facilitate
  testing of `sq`'s ability to support such names. The changed elements are:
  - `actor`:
    - `first_name -> first name`
    - `last_name -> last name`
  - `film_actor -> film actor`
