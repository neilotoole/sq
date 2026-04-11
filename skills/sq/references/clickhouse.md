# ClickHouse (`clickhouse` driver)

[ClickHouse](https://clickhouse.com/) columnar database. Requires **ClickHouse v25+**.

**Canonical docs:** [ClickHouse driver](https://sq.io/docs/drivers/clickhouse/)

## Beta

The ClickHouse driver is **beta**; behavior may change. Prefer reporting issues via [sq issues](https://github.com/neilotoole/sq/issues/new/choose).

## Add a source

Location string should start with **`clickhouse://`**. Use [`sq add`](https://sq.io/docs/cmd/add):

```shell
sq add 'clickhouse://default:@localhost:9000/default'
sq add 'clickhouse://user:password@host:9000/database' --handle @ch
```

**Connection pattern:** `clickhouse://username:password@hostname:port/database` with optional `?param=value`.

Default ports: native **9000** (non-TLS), **9440** (TLS when `secure=true`). If the port is omitted, `sq` applies the default for the security mode.

## Behavior notes (summary)

- **Schema/catalog:** ClickHouse “database” maps to `sq`’s [schema/catalog](https://sq.io/docs/concepts/#schema--catalog); use `--src.schema` when needed.
- **New tables:** `sq` tends to use **MergeTree** with `ORDER BY` on the first column when creating tables (insert/tbl copy).
- **Mutations:** `sq` maps updates to ClickHouse-style mutations; see docs.
- **Rows affected:** often reported as unsupported / `-1` for DML.
- **Types:** rich ClickHouse types may be coarsened to `sq`’s [kind system](https://sq.io/docs/concepts); see the mapping table on [sq.io](https://sq.io/docs/drivers/clickhouse/).
