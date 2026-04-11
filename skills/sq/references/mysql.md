# MySQL / MariaDB (`mysql` driver)

[MySQL](https://www.mysql.com/) and [MariaDB](https://mariadb.org/). Driver implements all optional `sq` features.

**Canonical docs:** [MySQL driver](https://sq.io/docs/drivers/mysql/)

## Add a source

Location string should start with **`mysql://`**. Use [`sq add`](https://sq.io/docs/cmd/add):

```shell
sq add 'mysql://user:password@localhost/dbname'
```

Quote the URL if it contains special characters.
