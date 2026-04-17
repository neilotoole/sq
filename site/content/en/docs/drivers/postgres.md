---
title: "Postgres"
description: "Postgres driver"
draft: false
images: []
weight: 4020
toc: false
url: /docs/drivers/postgres
---
The `sq` Postgres driver implements connectivity for
the [Postgres](https://www.postgresql.org) database.
The driver implements all optional driver features.

## Add source

Use [`sq add`](/docs/cmd/add) to add a source.  The location argument should start
with `postgres://`. For example:

```shell
sq add 'postgres://sakila:p_ssW0rd@localhost/sakila'
```


### Non-default schema

By default, the Postgres driver connects to the default `public` schema.
To use an alternate schema, add the [`search_path`](https://www.postgresql.org/docs/current/ddl-schemas.html#DDL-SCHEMAS-PATH)
param to the location string when adding the Postgres source.

For example, to use the `customer` schema:

```shell
sq add 'postgres://sakila:p_ssW0rd@localhost/sakila?search_path=customer'
```
Note that the location string should be quoted due to the `?` character.
