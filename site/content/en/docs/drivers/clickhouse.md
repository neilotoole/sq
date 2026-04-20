---
title: "ClickHouse"
description: "ClickHouse"
draft: false
images: []
weight: 4035
toc: true
url: /docs/drivers/clickhouse
---
The `sq` ClickHouse driver implements connectivity for
[ClickHouse](https://clickhouse.com), an open-source columnar database focused on
real-time analytics. The driver requires ClickHouse v25+.
The driver implements all optional driver features.

{{< alert icon="🐥" >}}
The ClickHouse driver is in beta release. There is still work to be done with
testing and edge cases. It is likely that the implementation will change
based upon user feedback. If you find a bug, please open an
[issue](https://github.com/neilotoole/sq/issues/new/choose).
{{< /alert >}}

## Add source

Use [`sq add`](/docs/cmd/add) to add a source. The location argument should
start with `clickhouse://`. For example:

```shell
sq add 'clickhouse://default:@localhost:9000/default'
```

With authentication:

```shell
sq add 'clickhouse://user:password@host:9000/database' --handle @ch
```

## Connection string format

```text
clickhouse://username:password@hostname:9000/database
clickhouse://username:password@hostname:9000/database?param=value
```

Default ports:

| Protocol | Non-Secure | Secure (TLS) |
|----------|------------|--------------|
| Native   | `9000`     | `9440`       |

If the port is omitted, `sq` auto-applies the default port: `9000` for
non-secure connections, or `9440` when `secure=true` is specified.

## Notes

### Active schema & catalog

ClickHouse "database" maps to `sq`'s [schema and catalog](/docs/concepts/#schema--catalog) concepts.

When executing a `sq` query, you can use `--src.schema` to specify the active schema
(or _catalog.schema_).

```shell
sq --src.schema=system '.tables | .[0:3]'
```

### Table engine

When `sq` creates tables (e.g., via `--insert` or `tbl copy`), it uses the
[MergeTree](https://clickhouse.com/docs/en/engines/table-engines/mergetree-family/mergetree)
engine with `ORDER BY` on the first column. MergeTree is the standard
ClickHouse table engine for most workloads.

### Mutations

ClickHouse does not support standard SQL `UPDATE` syntax directly. Instead,
it uses `ALTER TABLE ... UPDATE` for mutations. `sq` handles this transparently:
you can use `sq` normally and the driver generates the correct ClickHouse-specific
SQL. Mutations are forced to execute synchronously (via `mutations_sync = 1`)
so that data is consistent immediately after the operation returns.

### Rows affected

ClickHouse does not report the number of rows affected by DML operations
(`INSERT`, `UPDATE`, `DELETE`). When using `sq` with ClickHouse, you'll see
`rows affected: unsupported` in text output, or `-1` in JSON/YAML/etc output.

### Type mapping

ClickHouse's type system is richer than `sq`'s
[kind system](/docs/concepts), so some type coarsening occurs.
Wrapper types such as `Nullable(T)` and `LowCardinality(T)` are unwrapped
before mapping. For example, `LowCardinality(Nullable(String))` is treated
as `String`, which maps to `kind.Text`.

| ClickHouse Type (read)                | sq Kind         | ClickHouse Type (write) | Notes                              |
|---------------------------------------|-----------------|-------------------------|------------------------------------|
| `Int8`, `Int16`, `Int32`, `Int64`     | `kind.Int`      | `Int64`                 | All signed integers                |
| `UInt8`, `UInt16`, `UInt32`, `UInt64` | `kind.Int`      | `Int64`                 | All unsigned integers              |
| `Float32`, `Float64`                  | `kind.Float`    | `Float64`               |                                    |
| `Decimal(P,S)`, `Decimal128(S)`, etc. | `kind.Decimal`  | `Decimal(18,4)`         | All Decimal variants               |
| `Bool`                                | `kind.Bool`     | `Bool`                  |                                    |
| `String`, `FixedString(N)`            | `kind.Text`     | `String`                |                                    |
| `UUID`                                | `kind.Text`     | `String`                |                                    |
| `Date`, `Date32`                      | `kind.Date`     | `Date`                  |                                    |
| `DateTime`, `DateTime64`              | `kind.Datetime` | `DateTime`              | Including parameterized variants   |
| `Array(T)`                            | `kind.Text`     | `String`                | Serialized as comma-separated text |
| `Enum8(...)`, `Enum16(...)`           | `kind.Text`     | `String`                |                                    |
| `Map(K,V)`, `Tuple(...)`              | `kind.Text`     | `String`                |                                    |
| —                                     | `kind.Time`     | `DateTime`              | ClickHouse has no time-only type   |
| —                                     | `kind.Bytes`    | `String`                | Binary data stored as String       |

Nullable columns are wrapped with `Nullable(T)` (e.g., `Nullable(String)`,
`Nullable(Int64)`). ClickHouse columns are non-nullable by default.

The mapping is not a perfect round-trip. For example, `Int8` and `Int64`
both become `kind.Int`, and `kind.Int` always maps back to `Int64`. Similarly,
`kind.Time` maps to `DateTime`, which reads back as `kind.Datetime`, and
`kind.Bytes` maps to `String`, which reads back as `kind.Text`.
See [#544](https://github.com/neilotoole/sq/issues/544).

#### Array types

`Array(T)` types (e.g., `Array(String)`, `Array(Int32)`) are serialized as
comma-separated text values. For example, `["Action", "Drama"]` becomes
`Action,Drama` as `kind.Text`. This means that the original array structure
cannot be reconstructed from the text representation.
See [#545](https://github.com/neilotoole/sq/issues/545).

## Related

- [ClickHouse driver README](https://github.com/neilotoole/sq/blob/master/drivers/clickhouse/README.md)
- [#544](https://github.com/neilotoole/sq/issues/544) — Type roundtrip issues (`kind.Time`, `kind.Bytes`)
- [#545](https://github.com/neilotoole/sq/issues/545) — Array types flattened to CSV text (information loss)
