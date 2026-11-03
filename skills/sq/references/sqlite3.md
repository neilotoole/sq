# SQLite (`sqlite3` driver)

Local file-based [SQLite](https://www.sqlite.org/) databases. Driver type in `sq driver ls`: **`sqlite3`**.

**Canonical docs:** [SQLite driver](https://sq.io/docs/drivers/sqlite/)

## Add a source

Use [`sq add`](https://sq.io/docs/cmd/add) with the path to the `.db` file (relative or absolute). Example:

```shell
sq add ./sakila.db
sq add --driver=sqlite3 ./sakila.db
```

`sq` can usually [detect](https://sq.io/docs/detect/#driver-type) SQLite files; use `--driver=sqlite3` if needed.

**Connection string form** with prefix `sqlite3://` and optional [parameters](https://github.com/mattn/go-sqlite3#connection-string):

```shell
sq add 'sqlite3://sakila.db?cache=shared&mode=rw'
```

## Create a new empty database

```shell
sq add --driver=sqlite3 hello.db
```

## Notes

- SQLite implements the full optional driver feature set for `sq`.
- Extension support is documented on [sq.io](https://sq.io/docs/drivers/sqlite/) (early access); some features may require [`sq sql`](https://sq.io/docs/cmd/sql).
- Vectors: sqlite-vec `vec0` tables (built in) and libSQL `F32_BLOB(N)`-style columns have column kind `vector`. `sq inspect` reports `dim`, `elem` and `metric`. Text output shows `[384 × f32]`; JSON and YAML show the full array. SLQ decodes vectors from a single `vec0` table; in joins and in `sq sql`, `vec0` vectors are bytes (use `vec_to_json(col)`). SLQ has no KNN: use `sq sql --src @h "SELECT rowid, distance FROM t WHERE embedding MATCH '[0.1,0.2]' AND k = 2"`. Vector columns can't be written as vector types.
