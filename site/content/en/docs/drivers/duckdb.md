---
title: "DuckDB"
description: "DuckDB driver"
draft: false
images: []
weight: 4042
toc: true
url: /docs/drivers/duckdb
---
`sq`'s DuckDB driver implements connectivity for
[DuckDB](https://duckdb.org). It makes use of the backing
[`duckdb/duckdb-go/v2`](https://github.com/duckdb/duckdb-go) library,
which statically links `libduckdb`. The driver implements all optional
`sq` driver features.

## Add source

Use [`sq add`](/docs/cmd/add) to add a source. The location argument is the
filepath to the DuckDB file. For example:

```shell
# Relative path
$ sq add ./sakila.duckdb

# Absolute path
$ sq add /Users/neilotoole/sakila.duckdb
```

`sq` can [detect](/docs/detect#driver-type) that a file is a DuckDB datafile
via the `.duckdb` or `.ddb` extension, or by the `DUCK` magic header at byte
offset 8. If auto-detection doesn't trigger, specify the driver explicitly:

```shell
$ sq add --driver=duckdb ./sakila.duckdb
```

Use the `duckdb://` scheme to pass connection parameters or to open an
in-memory database:

```shell
# Persistent file via URI
$ sq add 'duckdb:///abs/path/sakila.duckdb'

# In-memory database
$ sq add 'duckdb://:memory:'

# File with connection parameters
$ sq add 'duckdb:///path/sakila.duckdb?memory_limit=4GB&threads=4'
```

## Bundled extensions

The driver statically links the standard set of in-tree DuckDB extensions.
They are available immediately — no `INSTALL` or `LOAD` required:

| Extension      | Purpose                               |
|----------------|---------------------------------------|
| `json`         | JSON read/write functions             |
| `parquet`      | Parquet read/write (`read_parquet()`) |
| `icu`          | ICU collations and Unicode functions  |
| `fts`          | Full-text search                      |
| `httpfs`       | HTTP(S) and S3 file access            |
| `excel`        | Excel read (`excel_open()`)           |
| `inet`         | IP address types and functions        |
| `autocomplete` | SQL auto-completion helpers           |
| `tpch`         | TPC-H benchmark tables                |
| `tpcds`        | TPC-DS benchmark tables               |

Because the extensions are statically bundled, queries like the following
work without any setup:

```sql
-- Query a Parquet file directly
SELECT * FROM read_parquet('file.parquet');

-- Query a remote CSV via HTTPS
SELECT * FROM read_csv_auto('https://example.com/data.csv');

-- Query an S3 object (set AWS credentials first)
SELECT * FROM read_parquet('s3://bucket/key.parquet');
```

## Connection parameters

Pass parameters as URL query strings after the file path:

```shell
$ sq add 'duckdb:///path/db.duckdb?memory_limit=4GB&threads=4'
```

| Parameter               | Values                  | Description                               |
|-------------------------|-------------------------|-------------------------------------------|
| `access_mode`           | `READ_WRITE`, `READ_ONLY` | Open the database read-only             |
| `memory_limit`          | e.g. `4GB`              | Maximum memory DuckDB may use             |
| `threads`               | integer                 | Number of threads                         |
| `default_order`         | `ASC`, `DESC`           | Default sort order                        |
| `default_null_order`    | `NULLS_FIRST`, `NULLS_LAST` | Default NULL sort position           |
| `enable_external_access`| `true`, `false`         | Allow reading from external files/URLs    |
| `enable_object_cache`   | `true`, `false`         | Cache metadata for remote objects         |
| `temp_directory`        | path                    | Directory for temporary files             |
| `wal_autocheckpoint`    | e.g. `1000`             | WAL autocheckpoint threshold (pages)      |

DuckDB supports [many more settings](https://duckdb.org/docs/configuration/overview);
the list above covers the parameters most commonly used from the CLI.

## Type mapping

| DuckDB type                                              | `sq` kind    | Notes                                              |
|----------------------------------------------------------|--------------|----------------------------------------------------|
| `BOOLEAN`                                                | `bool`       |                                                    |
| `TINYINT` … `BIGINT`, `UTINYINT`, `USMALLINT`, `UINTEGER` | `int`     |                                                    |
| `HUGEINT`, `UHUGEINT`, `UBIGINT`, `INT128`               | `decimal`    | Promoted to `decimal` because values can exceed `int64`. |
| `FLOAT`, `REAL`, `DOUBLE`                                | `float`      |                                                    |
| `DECIMAL(p,s)`                                           | `decimal`    |                                                    |
| `VARCHAR`, `TEXT`, `STRING`                              | `text`       |                                                    |
| `BLOB`, `BYTEA`                                          | `bytes`      |                                                    |
| `DATE`                                                   | `date`       |                                                    |
| `TIME`, `TIME WITH TIME ZONE`                            | `time`       |                                                    |
| `TIMESTAMP`, `TIMESTAMP_S/MS/NS`, `TIMESTAMPTZ`         | `datetime`   |                                                    |
| `INTERVAL`                                               | `text`       | ISO 8601 duration string.                          |
| `UUID`                                                   | `text`       | Hex-dash form.                                     |
| `JSON`                                                   | `text`       | Already JSON.                                      |
| `LIST` / `ARRAY`                                         | `text`       | Projected via `to_json(col)` at scan time.         |
| `STRUCT`                                                 | `text`       | Projected via `to_json(col)` at scan time.         |
| `MAP`                                                    | `text`       | Projected via `to_json(col)` at scan time.         |
| `ENUM`                                                   | `text`       | Underlying value.                                  |
| `BIT`                                                    | `text`       | Bit-string.                                        |

Composite types (`LIST`, `STRUCT`, `MAP`) are projected to JSON text at scan
time. First-class `kind.Array` / `kind.Struct` support is planned as a
follow-up.

## Limitations

- **Single writer per file.** DuckDB enforces a single writer per database
  file. If two `sq` processes open the same `.duckdb` file simultaneously, the
  second will receive a lock error. For `sq`'s typical single-shot CLI usage
  this is rarely a problem, but scripts that parallelize `sq` against the same
  file should serialize writes. Read-only access (`access_mode=READ_ONLY`) from
  multiple processes is safe.

- **Sakila FOREIGN KEY constraints.** The bundled test Sakila database for
  DuckDB omits several FOREIGN KEY constraints that exist in the original
  MySQL/SQLite schemas, due to circular-dependency issues during schema
  porting. This does not affect `sq`'s behavior against real-world DuckDB
  databases.
