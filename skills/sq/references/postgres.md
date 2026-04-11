# PostgreSQL (`postgres` driver)

[PostgreSQL](https://www.postgresql.org/) over a network connection. Driver implements all optional `sq` features.

**Canonical docs:** [Postgres driver](https://sq.io/docs/drivers/postgres/)

## Add a source

Location string should start with **`postgres://`**. Use [`sq add`](https://sq.io/docs/cmd/add):

```shell
sq add 'postgres://user:password@localhost/dbname'
```

Quote the URL when it contains `?` or special characters.

## Non-default schema

Default schema is `public`. To use another schema, add **`search_path`** to the URL:

```shell
sq add 'postgres://user:password@localhost/dbname?search_path=customer'
```

See [PostgreSQL search_path](https://www.postgresql.org/docs/current/ddl-schemas.html#DDL-SCHEMAS-PATH).
