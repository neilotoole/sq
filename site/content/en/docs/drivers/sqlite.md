---
title: "SQLite"
description: "SQLite driver"
draft: false
images: []
weight: 4040
toc: true
url: /docs/drivers/sqlite
---

The `sq` SQLite driver implements connectivity for
the [SQLite](https://www.sqlite.org) database. It makes use of the backing
[`mattn/sqlite3`](https://github.com/mattn/go-sqlite3) library.
The driver implements all optional `sq` driver features.

## Add source

Use [`sq add`](/docs/cmd/add) to add a source. The location argument is simply the
filepath to the SQLite DB file. For example:

```shell
# Relative path
$ sq add ./sakila.db

# Absolute path
$ sq add /Users/neilotoole/sakila.db
```

`sq` usually can [detect](/docs/detect#driver-type) that a file is a SQLite datafile, but in the event
it doesn't, you can explicitly specify the driver type:

```shell
$ sq add --driver=sqlite3 ./sakila.db
```

Use the connection string form with prefix `sqlite3://` to specify
connection [parameters](https://github.com/mattn/go-sqlite3#connection-string):

```shell
# Read/write with a shared cache.
$ sq add 'sqlite3://sakila.db?cache=shared&mode=rw'

# Read-only — any write fails at the SQLite layer.
$ sq add 'sqlite3://sakila.db?mode=ro'

# Treat the file as immutable so SQLite skips locking. Useful for
# inspecting a live database while another process holds it open —
# for example, a Firefox cookies DB while the browser is running.
$ sq add 'sqlite3://cookies.sqlite?immutable=1' --handle @cookies
```

The full set of supported parameters can be found in the `mattn/sqlite3`
[docs](https://github.com/mattn/go-sqlite3#connection-string).

## Create new SQLite DB

You can use `sq` to create a new, empty, SQLite DB file.

```shell
$ sq add --driver sqlite3 hello.db
@hello  sqlite3  hello.db
```

## Extensions

The SQLite driver has several
[extensions](https://github.com/mattn/go-sqlite3#feature--extension-list) baked in.

{{< alert icon="🐥" >}}
`sq`'s SQLite extension support is in early access. There is no special handling
in `sq`'s query language for any of the particular extension features (e.g. JSON). This may
change over time: [open an issue](https://github.com/neilotoole/sq/issues/new/choose)
if you have a suggestion, or encounter unexpected or undesirable behavior.

You may find it necessary to use [native sql mode](/docs/cmd/sql) to access some
extension features.
{{< /alert >}}

| Extension        | Details                                                                                                                                                                                                              |
| ---------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `vtable`         | [Virtual Table](https://www.sqlite.org/vtab.html) <br/><small>👉 [`sq inspect`](/docs/inspect) will show the virtual table's type as `virtual`.</small>                                                              |
| `fts5`           | [Full Text Search 5](https://www.sqlite.org/fts5.html)                                                                                                                                                               |
| `json`           | [JSON](https://sqlite.org/json1.html)                                                                                                                                                                                |
| `math_functions` | [Math Functions](https://www.sqlite.org/lang_mathfunc.html)                                                                                                                                                          |
| `introspect`     | Additional `PRAGMA` statements: `function_list`, `module_list`, `pragma_list`                                                                                                                                        |
| `stat4`          | Additional statistics to assist query planning                                                                                                                                                                       |
| `vec0`           | [sqlite-vec](https://github.com/asg017/sqlite-vec) vector search, from the community fork [`vlasky/sqlite-vec`](https://github.com/vlasky/sqlite-vec) `v0.2.7-alpha` <br/><small>👉 See [Vectors](#vectors).</small> |

{{< alert icon="👉" >}}
There is already a [feature request](https://github.com/neilotoole/sq/issues/86) to
add [SpatiaLite](https://www.gaia-gis.it/fossil/libspatialite/index) support.
{{< /alert >}}

## Vectors

`sq` recognizes vector columns in two kinds of SQLite database, and gives them the
column kind `vector`.

- [sqlite-vec](https://github.com/asg017/sqlite-vec) `vec0` virtual tables. The
  extension is built into `sq`, so `vec0` tables open, inspect and query with no setup.
- libSQL/Turso vector columns, declared as `F32_BLOB(N)` (also `F64_BLOB`, `F16_BLOB`,
  `FB16_BLOB`, `F8_BLOB`, `F1BIT_BLOB` and the `FLOAT32(N)` style aliases). No extension
  is needed.

A plain `BLOB` or `JSON` column is never treated as a vector.

### Inspect

`sq inspect` reports each vector column with its declared type and, in JSON and YAML,
a `vector` object with the dimension (`dim`), element type (`elem`) and, for `vec0`,
the distance metric (`metric`).

```shell
$ sq inspect @vec.doc_vec -j
{
  "name": "doc_vec",
  "table_type": "virtual",
  "row_count": 3,
  "columns": [
    {
      "name": "doc_id",
      "primary_key": true,
      "column_type": "INTEGER",
      "kind": "int"
    },
    {
      "name": "embedding",
      "column_type": "float[4]",
      "kind": "vector",
      "vector": {
        "dim": 4,
        "elem": "f32",
        "metric": "cosine"
      }
    },
    {
      "name": "category",
      "kind": "text",
      "role": "metadata"
    },
    {
      "name": "note",
      "kind": "text",
      "role": "auxiliary"
    }
  ]
}
```

(Output abridged.) A `vec0` partition key, metadata or auxiliary column carries a `role`.
In `sq inspect -v` text output, the column type shows the metric and the role, e.g.
`float[4] cosine` and `TEXT (auxiliary)`.
For a libSQL table, a `libsql_vector_idx` index is listed with a `vector` object holding
its `method` (`diskann`), `metric` and `params`.

A virtual table that `sq` can't serve doesn't break `sq inspect` for the whole source.
The table is listed with an error, and `sq inspect @src` prints a `warning:` line for each
such table on stderr and exits 0. `sq inspect @src.tbl` on such a table exits with an error.

- For a `vec0` table that the bundled sqlite-vec can't open, such as one using a newer
  sqlite-vec feature like `INDEXED BY rescore(...)`, `inspect` shows the columns parsed
  from the table's declaration (including each vector column's dimension, element type
  and metric) together with the error. For `INDEXED BY`, the error names the column and
  the unsupported index type.
- For a table from any other unknown module, such as `vectorlite`, `inspect` shows the
  table's declaration and the error.

In JSON and YAML, such a table has an `error` field and a `definition` field holding its
declaration. Text output shows the error in the `COLS` column, and `sq inspect -v` prints
each such table's declaration after the table.

### Query

An SLQ query on a single `vec0` table decodes its vectors, including `SELECT *`-style
queries, column lists and aliases.

```shell
$ sq '@vec.doc_vec'
doc_id  embedding  category  note
1       [4 × f32]  animal    n1
2       [4 × f32]  animal    n2
3       [4 × f32]  vehicle   n3
```

SLQ has no nearest-neighbor or vector functions. For KNN search, use
[native SQL mode](/docs/cmd/sql), passing the query vector as a JSON literal:

```shell
$ sq sql --src @vec "SELECT doc_id, distance FROM doc_vec WHERE embedding MATCH '[0.85,0.15,0,0]' AND k = 2"
doc_id  distance
1       0.00204825890250504
2       0.0024704814422875643
```

In a join, and in `sq sql`, `vec0` vectors come back as bytes. In `sq sql`, wrap the
column in `vec_to_json(embedding)` to get numbers. libSQL vector columns decode
everywhere, including `sq sql`.

### Output formats

| Format                        | Vector rendering                                              |
| ----------------------------- | ------------------------------------------------------------- |
| Text                          | Summary, e.g. `[384 × f32]`                                   |
| JSON, YAML                    | Full array of numbers, in the shortest form that round-trips  |
| CSV, TSV, Markdown, HTML, XML | The array as a single text cell                               |
| XLSX                          | The same text, or the summary if it exceeds 32,767 characters |
| `--raw`                       | The stored bytes                                              |

When a query result is copied (`--insert`, or a cross-source join), a vector is written
as its original bytes into a `BLOB` column.

### Limits

- Vector columns can't be written as vector types.
- The internal tables behind a `vec0` table (`*_chunks`, `*_rowids`,
  `*_vector_chunksNN` and so on), and the internal tables of FTS5 and libSQL, are listed
  as ordinary tables.
- On a libSQL table with a `libsql_vector_idx` index, `sq inspect` and SLQ queries return
  the correct rows, but a raw `sq sql` query can undercount unless it adds `NOT INDEXED`
  after the table name: `SELECT count(*) FROM t NOT INDEXED`.

## Inspect field provenance

`sq inspect` populates the fields below from SQLite pragmas, built-in
functions, and the database file itself.

### Source-level fields

| Field        | Source                                                 |
| ------------ | ------------------------------------------------------ |
| `name`       | DB file basename                                       |
| `schema`     | first row of `pragma_database_list` (typically `main`) |
| `catalog`    | hardcoded `default` — SQLite has no catalog concept    |
| `user`       | not populated — SQLite has no user model               |
| `db_product` | `"SQLite3 v" + db_version`                             |
| `db_version` | `sqlite_version()`                                     |
| `size`       | filesystem size of the DB file (`os.Stat`)             |

### Per-table fields

| Field       | Source                                                  |
| ----------- | ------------------------------------------------------- |
| `row_count` | live `SELECT COUNT(*) FROM "tbl"`                       |
| `size`      | not reported — SQLite does not expose per-table storage |
