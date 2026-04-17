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
connection [parameters](https://github.com/mattn/go-sqlite3#connection-string).

```shell
$ sq add 'sqlite3://sakila.db?cache=shared&mode=rw'
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

The SQLite driver has several [extensions](https://github.com/mattn/go-sqlite3#feature--extension-list) baked in.

{{< alert icon="🐥" >}}
`sq`'s SQLite extension support is in early access. There is no special handling
in `sq`'s query language for any of the particular extension features (e.g. JSON). This may
change over time: [open an issue](https://github.com/neilotoole/sq/issues/new/choose)
if you have a suggestion, or encounter unexpected or undesirable behavior.

You may find it necessary to use [native sql mode](/docs/cmd/sql) to access some
extension features.
{{< /alert >}}

| Extension        | Details                                                                                                                                                 |
|------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------|
| `vtable`         | [Virtual Table](https://www.sqlite.org/vtab.html) <br/><small>👉 [`sq inspect`](/docs/inspect) will show the virtual table's type as `virtual`.</small> |
| `fts5`           | [Full Text Search 5](https://www.sqlite.org/fts5.html)                                                                                                  |
| `json`           | [JSON](https://sqlite.org/json1.html)                                                                                                                   |
| `math_functions` | [Math Functions](https://www.sqlite.org/lang_mathfunc.html)                                                                                             |
| `introspect`     | Additional `PRAGMA` statements: `function_list`, `module_list`, `pragma_list`                                                                           |
| `stat4`          | Additional statistics to assist query planning                                                                                                          |

{{< alert icon="👉" >}}
There is already a [feature request](https://github.com/neilotoole/sq/issues/86) to
add [SpatiaLite](https://www.gaia-gis.it/fossil/libspatialite/index) support.
{{< /alert >}}
