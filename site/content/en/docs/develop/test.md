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

`sq` is a much more difficult beast to test than a typical Go project. `sq` is all about integrating data sources, and
that means databases. Much of the test code is testing of interaction with actual databases instances via that
database's Go driver. This is not something that can or should be mocked. We must test against the real thing.

The `sq` test code generally tests against a version of the [Sakila](https://dev.mysql.com/doc/sakila/en/) sample
database. This sample database was originally created for MySQL many years ago. It was "_intended to provide a standard
schema that can be used for examples in books, tutorials, articles, samples, and so forth_" and the _so forth_ also
includes testing. Others have done the legwork of producing Sakila SQL scripts so that the same database is available
not just for MySQL, but also SQLite, Postgres, SQL Server, and others.

For file-based sources, the `sq` repo includes the Sakila data files. For example, there's
a [sakila.db](https://github.com/neilotoole/sq/raw/master/drivers/sqlite3/testdata/sakila.db) file for SQLite, and
a [sakila.xlsx](https://github.com/neilotoole/sq/raw/master/drivers/xlsx/testdata/sakila.xlsx) file for Excel. For typical
SQL database sources (Postgres, SQL Server, MySQL), the Sakila databases have been wrapped up into Docker images in a
sister project named `sakiladb`. See the [GitHub](https://github.com/sakiladb)
and [DockerHub](https://hub.docker.com/u/sakiladb) repos.

To run all of the `sq` tests, there must be an available Sakila database instance for each database/version. The full
set of sources that the test code uses can be found
in [testh/testdata/sources.sq.yml](https://github.com/neilotoole/sq/blob/master/testh/testdata/sources.sq.yml). That file
looks something like (truncated version shown):

```yaml
sources:
  items:
    - handle: '@sakila_sl3'
      type: sqlite3
      location: sqlite3://${SQ_ROOT}/drivers/sqlite3/testdata/sakila.db
    - handle: '@sakila_pg9'
      type: postgres
      location: postgres://sakila:p_ssW0rd@${SQ_TEST_SRC__SAKILA_PG9}/sakila
    - handle: '@sakila_pg10'
      type: postgres
      location: postgres://sakila:p_ssW0rd@${SQ_TEST_SRC__SAKILA_PG10}/sakila
    - handle: '@sakila_my56'
      type: mysql
      location: mysql://sakila:p_ssW0rd@${SQ_TEST_SRC__SAKILA_MY56}/sakila
    - handle: '@sakila_my57'
      type: mysql
      location: mysql://sakila:p_ssW0rd@${SQ_TEST_SRC__SAKILA_MY57}/sakila
```

Note that for each of the external databases, there is a matching envar. For example, `@sakila_pg9` has its `location`
field populated with envar `SQ_TEST_SRC__SAKILA_PG9`. The envar is simply the `host` (meaning `hostname:port`) part of
the connection string for that source.

> **Note:** In the `yaml` snippet above, for local file-based sources such as `@sakila_sl3`
> with `location: sqlite3://${SQ_ROOT}/drivers/sqlite3/testdata/sakila.db`, you'll notice a variable `${SQ_ROOT}`. It is
> not necessary to explicitly set this variable as an envar: the `sq` test framework calculates it automatically.

Importantly: **When running `sq` tests, if the envar for a source is not populated, any test that uses that source is
skipped.**

Thus, to run _all_ of the `sq` tests, there must be available instances of all of the Sakila database/versions. These
databases could be run locally, or on a remote server. For local dev/test, it is typical to export these envars
in `.bashrc`/`.zshrc` or similar. For example (in this case, the Docker containers are running on a remote server):

```sh
# MySQL
export SQ_TEST_SRC__SAKILA_MY56=192.168.30.129
export SQ_TEST_SRC__SAKILA_MY57=192.168.30.131
export SQ_TEST_SRC__SAKILA_MY8=192.168.30.132
# Postgres
export SQ_TEST_SRC__SAKILA_PG9=192.168.30.133
export SQ_TEST_SRC__SAKILA_PG10=192.168.30.134
export SQ_TEST_SRC__SAKILA_PG11=192.168.30.135
export SQ_TEST_SRC__SAKILA_PG12=192.168.30.136
# MSSQL
export SQ_TEST_SRC__SAKILA_MS17=192.168.30.137
```

