---
title: "Sakila DB"
description: "Sakila DB"
lead: "Sakila is an example dataset, ported to many databases."
draft: false
images: []
weight: 6001
toc: true
url: /docs/develop/sakila
---
`sq` documentation typically uses the [Sakila](https://dev.mysql.com/doc/sakila/en/) example database. Sakila was
originally created for MySQL, but the data is available for many database implementations.

This page shows how to add a Sakila source to `sq` for various driver types.

## SQLite

To add a source with handle `@sakila_sl3`, download [`sakila.db`](https://sq.io/testdata/sakila.db) and `sq add`.

```shell
$ wget https://sq.io/testdata/sakila.db

$ sq add ./sakila.db --handle=@sakila_sl3
@sakila_sl3  sqlite3  sakila.db
```

Note above the `--handle=@sakila_sl3` flag. This flag is optional: if no handle specified, a suitable handle is
generated. You can also use the shorthand flag `-N @sakila_sl3`.

## Postgres

The Sakila database has been bundled into a Postgres [docker image](https://hub.docker.com/r/sakiladb/postgres).
Run the image and then `sq add`.

> It may take several minutes for docker to download and start the image. Eventually the docker logs will show:
`sakiladb/postgres has successfully initialized.`. Shortly after this message is logged, the database should start
> accepting connections.

```shell
$ docker run -d -p 5432:5432 sakiladb/postgres:latest
# Wait a while...

sq add 'postgres://sakila:p_ssW0rd@localhost/sakila' --handle @sakila_pg
@sakila_pg  postgres  sakila@localhost/sakila
```

## SQL Server

{{< alert icon="⚠️" >}}
SQL Server runs only on `amd64` . For `arm64` (e.g. Apple M1+),
use [Azure SQL Edge](#azure-sql-edge).
{{< /alert >}}

The Sakila database has been bundled into a SQL Server [docker image](https://hub.docker.com/r/sakiladb/sqlserver).
Run the image and then `sq add`.

> It may take several minutes for docker to download and start the image. Eventually the docker logs will show:
`sakiladb/sqlserver has successfully initialized.`. Shortly after this message is logged, the database should start
> accepting connections.

```shell
$ docker run -d -p 1433:1433 sakiladb/sqlserver:latest
# Wait a while...

$ sq add 'sqlserver://sakila:p_ssW0rd@localhost:1433?database=sakila' --handle @sakila_sqlserver
@sakila_sqlserver  sqlserver  sakila@localhost:1433/sakila
```

## Azure SQL Edge

The Sakila database has been bundled into an Azure SQL
Edge [docker image](https://hub.docker.com/r/sakiladb/azure-sql-edge).
Azure SQL Edge is effectively a slimmed-down SQL Server distro, but it runs
both on `amd64` and `arm64`. Note that `sq` treats Azure SQL Edge as if it is SQL Server
(they use the same driver etc.).

Run the image and then `sq add`.

> It may take several minutes for docker to download and start the image. Eventually the docker logs will show:
`sakiladb/sqlserver has successfully initialized.`. Shortly after this message is logged, the database should start
> accepting connections.

```shell
$ docker run -d -p 1433:1433 sakiladb/azure-sql-edge:latest
# Wait a while...

$ sq add 'sqlserver://sakila:p_ssW0rd@localhost:1433?database=sakila' --handle @sakila_sqlserver
@sakila_sqlserver  sqlserver  sakila@localhost:1433/sakila
```

## MySQL

The Sakila database has been bundled into a MySQL [docker image](https://hub.docker.com/r/sakiladb/mysql).
Run the image and then `sq add`.

> It may take several minutes for docker to download and start the image. Eventually the docker logs will show:
`sakiladb/mysql has successfully initialized.`. Shortly after this message is logged, the database should start
> accepting connections.

```shell
$ docker run -d -p 3306:3306 sakiladb/mysql:latest
# Wait a while...

$ sq add 'mysql://sakila:p_ssW0rd@localhost:3306/sakila' --handle @sakila_mysql
@sakila_mysql  mysql  sakila@localhost:3306/sakila
```

## Microsoft Excel XLSX

To add a source with handle `@sakila_xlsx`, download [sakila.xlsx](https://sq.io/testdata/sakila.xlsx) and `sq add`.

```shell
$ wget https://sq.io/testdata/sakila.xlsx

$ sq add ./sakila.xlsx --handle @sakila_xlsx
@sakila_xlsx  xlsx  sakila.xlsx
```

## CSV

Note that CSV is a "monotable" data type. There's effectively only a single table unlike, say, XLSX, which can have
multiple sheets/tables. Thus, each Sakila table exists as a separate CSV file. When added to `sq`, each of these CSV
files would become its own data source, e.g. `@sakila_csv_actor`, `@sakila_csv_film` etc, with the data accessible
via `sq @sakila_csv_actor.data`, `sq @sakila_csv_film.data` etc.

Download and extract [sakila-csv.tar.gz](https://sq.io/testdata/sakila-csv.tar.gz) (
or [sakila-tsv.tar.gz](https://sq.io/testdata/sakila-tsv.tar.gz)), and `sq add`.

```shell
$ wget -qO- https://sq.io/testdata/sakila-csv.tar.gz | tar xvz -
$ cd sakila-csv
$ ls
actor.csv    category.csv  country.csv   film.csv        film_category.csv  inventory.csv  payment.csv  staff.csv
address.csv  city.csv      customer.csv  film_actor.csv  film_text.csv      language.csv   rental.csv   store.csv

$ sq add actor.csv --handle @sakila_csv_actor
@sakila_csv_actor  csv  actor.csv
```
