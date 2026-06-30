---
title: "Testing"
description: "testing"
draft: false
images: []
weight: 6002
toc: true
url: /docs/develop/test
---

In a nutshell:

```shell
$ go test ./...
```

`sq` is a much more difficult beast to test than a typical Go project. `sq` is all about
integrating data sources, and that means databases. Much of the test code is testing of
interaction with actual databases instances via that database's Go driver. This is not something
that can or should be mocked. We must test against the real thing.

The `sq` test code generally tests against a version of the
[Sakila](https://dev.mysql.com/doc/sakila/en/) sample database. This sample database was
originally created for MySQL many years ago. It was "_intended to provide a standard schema
that can be used for examples in books, tutorials, articles, samples, and so forth_" and the
_so forth_ also includes testing. Others have done the legwork of producing Sakila SQL scripts
so that the same database is available not just for MySQL, but also SQLite, Postgres, SQL Server,
and others.

For file-based sources, the `sq` repo includes the Sakila data files. For example, there's
a [sakila.db](https://github.com/neilotoole/sq/raw/master/drivers/sqlite3/testdata/sakila.db)
file for SQLite, and
a [sakila.xlsx](https://github.com/neilotoole/sq/raw/master/drivers/xlsx/testdata/sakila.xlsx)
file for Excel. For typical SQL database sources (Postgres, SQL Server, MySQL), the Sakila
databases have been wrapped up into Docker images in a sister project named `sakiladb`. See the
[GitHub](https://github.com/sakiladb) and [DockerHub](https://hub.docker.com/u/sakiladb) repos.

To run all of the `sq` tests, there must be an available Sakila database instance for each
engine. There is one source per engine; the engine _version_ is a deployment detail (which
`sakiladb/<engine>:<tag>` image the source points at), not part of the handle. Coverage
across multiple versions of an engine is provided by CI (see [below](#multi-version-coverage)),
not by multiple handles. The full set of sources that the test code uses can be found in

<!-- markdownlint-disable-next-line MD013 -->

[testh/testdata/test.sq.yml](https://github.com/neilotoole/sq/blob/master/testh/testdata/test.sq.yml).
That file looks something like (truncated version shown):

```yaml
sources:
  items:
    - handle: "@sakila_sl3"
      type: sqlite3
      location: sqlite3://${env:SQ_ROOT}/drivers/sqlite3/testdata/sakila.db
    - handle: "@sakila_pg"
      type: postgres
      location: ${env:SQ_TEST_SRC__SAKILA_PG}
    - handle: "@sakila_my"
      type: mysql
      location: ${env:SQ_TEST_SRC__SAKILA_MY}
    - handle: "@sakila_ms"
      type: sqlserver
      location: ${env:SQ_TEST_SRC__SAKILA_MS}
```

Each external database has a matching envar holding its full DSN. For example:

```sh
SQ_TEST_SRC__SAKILA_PG=postgres://sakila:p_ssW0rd@localhost:5432/sakila
```

`test.sq.yml` references the envar as the entire location:

```yaml
location: ${env:SQ_TEST_SRC__SAKILA_PG}
```

Run `./sakila-start-local.sh` to start the containers and print the exact exports.

> **Note:** In the `yaml` snippet above, for local file-based sources such as `@sakila_sl3`
> with `location: sqlite3://${env:SQ_ROOT}/drivers/sqlite3/testdata/sakila.db`, you'll notice a
> placeholder `${env:SQ_ROOT}`. You do not set `SQ_ROOT` yourself: the `sq` test framework
> always derives it in-process from the working directory (the sq module root). Any `SQ_ROOT`
> you export in your shell is ignored.

By default the test harness loads `testh/testdata/test.sq.yml`. Set
`SQ_TEST_CONFIG_FILE=/path/to/your/test.sq.yml` to point the harness at a
different config file instead; its `${env:...}` placeholders (and `SQ_ROOT`
via `${env:SQ_ROOT}`) resolve exactly like the default file.

Importantly: **When running `sq` tests, if the envar for a source is not populated, any test
that uses that source is skipped.**

Thus, to run _all_ of the `sq` tests, there must be an available instance of each Sakila
engine. These databases could be run locally, or on a remote server. For local dev/test,
it is typical to export these envars in `.bashrc`/`.zshrc` or similar. There is one envar
per engine; point it at whichever `sakiladb/<engine>:<tag>` version you want to test against.
For example (in this case, the Docker containers are running on a remote server):

```sh
export SQ_TEST_SRC__SAKILA_PG=postgres://sakila:p_ssW0rd@192.168.30.133/sakila
export SQ_TEST_SRC__SAKILA_MY=mysql://sakila:p_ssW0rd@192.168.30.129/sakila
export SQ_TEST_SRC__SAKILA_MS=sqlserver://sakila:p_ssW0rd@192.168.30.137?database=sakila
export SQ_TEST_SRC__SAKILA_CH=clickhouse://sakila:p_ssW0rd@192.168.30.138/sakila
export SQ_TEST_SRC__SAKILA_OR=oracle://sakila:p_ssW0rd@192.168.30.139:1521/SAKILA
export SQ_TEST_SRC__SAKILA_RQ=http://192.168.30.140:4001
```

The easiest way to get these exports is to run `./sakila-start-local.sh`, which starts a
container for each engine (from [`.github/sakila-db.json`](https://github.com/neilotoole/sq/blob/master/.github/sakila-db.json),
the single source of truth shared with CI) and prints the exact export lines.

## Multi-version coverage

Because each engine has a single handle, running the test suite locally exercises whatever
image version each envar points at. Coverage across _multiple_ versions of an engine
(e.g. Postgres 12 through 17, MySQL 8 and 9) is provided by CI rather than by additional
handles: the [`DB integration`](https://github.com/neilotoole/sq/blob/master/.github/workflows/db-integration.yml)
workflow can be dispatched against a chosen set of engines/versions, and the
[`DB integration (scheduled)`](https://github.com/neilotoole/sq/blob/master/.github/workflows/db-scheduled.yml)
workflow sweeps the full version matrix on a nightly/weekly cadence.
