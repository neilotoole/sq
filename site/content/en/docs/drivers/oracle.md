---
title: "Oracle"
description: "Oracle driver"
draft: false
images: []
weight: 4037
toc: true
url: /docs/drivers/oracle
---
The `sq` Oracle driver implements connectivity for
[Oracle Database](https://www.oracle.com/database/).

{{< alert icon="🧪" >}}
The Oracle driver is experimental. Behavior may change as test coverage and
edge-case support improve.
{{< /alert >}}

## Add source

Use [`sq add`](/docs/cmd/add) to add an Oracle source.

```shell
sq add 'oracle://user:password@host:1521/service_name'
```

## Connection string format

```text
oracle://username:password@hostname:1521/service_name
oracle://username:password@hostname/service_name
oracle://username:password@tns_alias
```

## Notes

### Schema and catalog

Oracle does not implement catalogs in the same sense as Postgres or SQL
Server. In `sq`, `catalog()` returns `NULL` for Oracle sources, and
`schema()` returns the current Oracle schema (which maps to the connected
user).

### Metadata visibility

`sq inspect` metadata for Oracle is gathered from `USER_*` dictionary views for
the connected schema. Listing table names with an explicit schema uses
`ALL_TABLES` / `ALL_VIEWS` filtered by owner.

### Requirements

The Oracle driver depends on
[godror](https://github.com/godror/godror), which requires Oracle Instant
Client.

## Related

- [Oracle driver README](https://github.com/neilotoole/sq/blob/master/drivers/oracle/README.md)
