# MySQL / MariaDB (`mysql` driver)

[MySQL](https://www.mysql.com/) and [MariaDB](https://mariadb.org/). Driver implements all optional `sq` features.

**Canonical docs:** [MySQL driver](https://sq.io/docs/drivers/mysql/)

## Add a source

Location string should start with **`mysql://`**. Use [`sq add`](https://sq.io/docs/cmd/add)
with **`-p`** so the password is prompted rather than embedded in the URL:

```shell
sq add -p 'mysql://user@localhost/dbname'
```

Quote the URL if it contains special characters.

## Schema

MySQL uses database names as schemas. Use **`--src.schema`** to query a specific schema:

```shell
# Override schema at query time
sq --src.schema=myschema '.mytable'

# Or embed a default schema in the connection URL at add time
sq add -p 'mysql://user@localhost/dbname' --src.schema myschema
```

## Auth and SSL

MySQL supports standard username/password auth. For SSL, add TLS parameters to the DSN.
See the [MySQL driver docs](https://sq.io/docs/drivers/mysql/) for DSN options and auth caveats.
