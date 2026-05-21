# Oracle (`oracle` driver)

[Oracle Database](https://www.oracle.com/database/) over Oracle Net. Pure-Go
[go-ora](https://github.com/sijms/go-ora) — no Oracle Instant Client, OCI, or CGo.

**Canonical docs:** [Oracle driver](https://sq.io/docs/drivers/oracle/)

## Experimental

The Oracle driver is **experimental**; behavior may change. Tested primarily against Oracle
Database 23ai Free ([sakiladb/oracle](https://github.com/sakiladb/oracle)). Prefer reporting
issues via [sq issues](https://github.com/neilotoole/sq/issues/new/choose).

## Add a source

Location string should start with **`oracle://`**. Use [`sq add`](https://sq.io/docs/cmd/add)
with **`-p`** so the password is prompted rather than embedded in the URL:

```shell
sq add -p 'oracle://user@host:1521/service_name'
```

Sakila test image:

```shell
sq add 'oracle://sakila:p_ssW0rd@localhost:1521/SAKILA' --handle @sakila_ora
```

Quote the URL when it contains `?`, `&`, spaces, or shell-special characters. Query
parameters (SSL, wallet, timeouts) pass through to `go-ora`, e.g.
`?SSL=true` or `?CONNECTION%20TIMEOUT=30`.

`TNSNAMES.ora` aliases and Kerberos are not handled by `sq` directly — use an Oracle URL
for `sq add`.

## Schema and catalog

- **Schema** in `sq` is the connected Oracle user (`CURRENT_SCHEMA`).
- **Catalog** is the database/PDB name (`DB_NAME`); the URL **service name** routes the
  connection but is not the catalog itself.
- Unquoted identifiers are stored **uppercase** in Oracle; cross-source inserts to
  case-sensitive destinations (Postgres, ClickHouse) map column names case-insensitively.

## Behavior notes (summary)

- Bind placeholders render as `:1`, `:2`, …
- Empty strings are treated as `NULL` in Oracle.
- Booleans are stored as `NUMBER(1,0)`.
- `sq tbl` column-kind alteration is not implemented for Oracle yet.
- Synonyms are not implemented yet.

Local dev: `docker run -d -p 1521:1521 sakiladb/oracle:latest` (startup can take several
minutes).
