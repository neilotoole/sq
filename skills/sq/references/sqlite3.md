# SQLite (`sqlite3` driver)

Local file-based [SQLite](https://www.sqlite.org/) databases. Driver type in `sq driver ls`: **`sqlite3`**.

**Canonical docs:** [SQLite driver](https://sq.io/docs/drivers/sqlite/)

## Add a source

Use [`sq add`](https://sq.io/docs/cmd/add) with the path to the `.db` file (relative or absolute). Example:

```shell
sq add ./sakila.db
sq add --driver=sqlite3 ./sakila.db
```

`sq` can usually [detect](https://sq.io/docs/detect#driver-type) SQLite files; use `--driver=sqlite3` if needed.

**Connection string form** with prefix `sqlite3://` and optional [parameters](https://github.com/mattn/go-sqlite3#connection-string):

```shell
sq add 'sqlite3://sakila.db?cache=shared&mode=rw'
```

## Create a new empty database

```shell
sq add --driver sqlite3 hello.db
```

## Notes

- SQLite implements the full optional driver feature set for `sq`.
- Extension support is documented on [sq.io](https://sq.io/docs/drivers/sqlite/) (early access); some features may require [`sq sql`](https://sq.io/docs/cmd/sql).
