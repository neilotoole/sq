# DuckDB (`duckdb` driver)

[DuckDB](https://duckdb.org/) analytics database (file-based or in-memory). Driver type in
`sq driver ls`: **`duckdb`**. Implements all optional `sq` driver features.

**Canonical docs:** [DuckDB driver](https://sq.io/docs/drivers/duckdb/)

## Add a source

Use [`sq add`](https://sq.io/docs/cmd/add) with the path to the `.duckdb` file (relative or
absolute). Example:

```shell
sq add ./sakila.duckdb
sq add --driver=duckdb ./sakila.duckdb
```

`sq` can usually [detect](https://sq.io/docs/detect/#driver-type) DuckDB files (`.duckdb`,
`.ddb`, or the `DUCK` magic header); use `--driver=duckdb` if needed.

**Connection string form** with prefix `duckdb://` and optional query parameters:

```shell
sq add 'duckdb:///abs/path/sakila.duckdb'
sq add 'duckdb://:memory:'
sq add 'duckdb:///path/sakila.duckdb?memory_limit=4GB&threads=4'
```

Common URI parameters: `access_mode` (`READ_ONLY` / `READ_WRITE`), `memory_limit`,
`threads`, `enable_external_access`. See [sq.io](https://sq.io/docs/drivers/duckdb/) for the
full list.

## Extensions

`json`, `parquet`, `icu`, `autocomplete` and DuckDB's core functions are statically linked.
`httpfs`, `excel`, `fts`, `inet`, `tpch`, `tpcds` and the other extensions in DuckDB's autoload
list install and load automatically on first use (needs network once per DuckDB version; cached
in `~/.duckdb`). You can query remote or local files directly, e.g. `read_parquet('file.parquet')`
or `read_csv_auto('https://example.com/data.csv')`. Extensions outside that list (`spatial`,
community extensions) need explicit `INSTALL`/`LOAD` and are not usable via `sq sql`.
`COPY ... TO 'x.xlsx'` is not autoloaded either; use `sq --xlsx` output to write Excel files.
`enable_external_access=false` disables autoload of all non-static extensions.

## Notes

- **Single writer:** only one process should open a `.duckdb` file for writes at a time;
  use `access_mode=READ_ONLY` for concurrent readers.
- Composite types (`LIST`, `STRUCT`, `MAP`) are stringified as text today; see driver docs
  for type-mapping detail.
