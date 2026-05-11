---
title: "MySQL"
description: "MySQL driver"
draft: false
images: []
weight: 4010
toc: false
url: /docs/drivers/mysql
---
The `sq` MySQL driver implements connectivity for
the [MySQL](https://www.mysql.com) and [MariaDB](https://mariadb.org) databases.
The driver implements all optional driver features.

## Add source

Use [`sq add`](/docs/cmd/add) to add a source. The location argument should start
with `mysql://`. For example:

```shell
sq add 'mysql://sakila:p_ssW0rd@localhost/sakila'
```

## Inspect field provenance

`sq inspect` populates the fields below from the MySQL server variables and
`information_schema`.

### Source-level fields

| Field | Source |
| --- | --- |
| `name`, `schema` | `DATABASE()` (MySQL conflates database and schema) |
| `catalog` | `INFORMATION_SCHEMA.SCHEMATA.CATALOG_NAME` (always `def` per the SQL standard for MySQL) |
| `user` | `CURRENT_USER()` |
| `db_product` | composed: `"@@GLOBAL.version_comment @@GLOBAL.version / @@GLOBAL.version_compile_os (@@GLOBAL.version_compile_machine)"` |
| `db_version` | `@@GLOBAL.version` |
| `size` | `SELECT SUM(data_length + index_length) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE()` |

### Per-table fields

| Field | Source |
| --- | --- |
| `row_count` | live ``SELECT COUNT(*) FROM `tbl` `` |
| `size` | `(DATA_LENGTH + INDEX_LENGTH)` from `information_schema.TABLES` |
