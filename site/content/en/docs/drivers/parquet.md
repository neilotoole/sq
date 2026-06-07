---
title: "Parquet"
description: "Apache Parquet driver"
draft: false
images: []
weight: 4065
toc: true
url: /docs/drivers/parquet
---
The `sq` Parquet driver reads [Apache Parquet](https://parquet.apache.org) files.
Parquet is a columnar binary format widely used in analytics pipelines.

Under the hood, the driver delegates to the [DuckDB driver](/docs/drivers/duckdb),
which statically bundles the `parquet` and `httpfs` extensions. No `INSTALL` or
`LOAD` step is required.

{{< alert icon="👉" >}}
A Parquet source is read-only; you can't [insert](/docs/output#insert) values into it.
{{< /alert >}}

## Add source

When adding a Parquet source via [`sq add`](/docs/cmd/add), the location string is the filepath:

```shell
$ sq add ./events.parquet
@events  parquet  events.parquet
```

You can also pass an absolute filepath, or an HTTPS URL (see [Remote files](#remote-files)).

`sq` detects Parquet automatically via the `PAR1` magic bytes at the start and end of the
file. To force the driver type explicitly:

```shell
$ sq add --driver=parquet ./events.parquet
```

## Monotable

`sq` treats a Parquet source as a monotable source, similar to CSV or JSON.
The data is accessed via the synthetic `.data` table. For example:

```shell
$ sq @events.data
user_id  event_name  ts
42       login       2024-01-15T08:00:00Z
43       logout      2024-01-15T08:05:00Z
```

## Remote files

`sq` accepts HTTPS URLs directly:

```shell
$ sq add https://example.com/data.parquet
@data  parquet  https://example.com/data.parquet
```

DuckDB's bundled `httpfs` extension handles remote reads transparently. A `SELECT count(*)`
only fetches the Parquet footer, so it returns quickly. Column projection limits the bytes
pulled across the wire.

## S3 and other remotes

AWS credentials are read from the environment (`AWS_ACCESS_KEY_ID`,
`AWS_SECRET_ACCESS_KEY`, `AWS_REGION`) by `httpfs`. For example:

```shell
$ sq add 's3://my-bucket/events.parquet'
```

Explicit per-source S3 options are planned for a future release.

## Connection options

DuckDB connection parameters can be appended as query-string values on the location:

```shell
$ sq add './big.parquet?threads=4&memory_limit=2GB'
```

The supported parameters are the same as the [DuckDB driver](/docs/drivers/duckdb#connection-parameters).

## Limitations

- Read-only. Writing back to Parquet is not yet supported.
- Single file per source. Partitioned datasets (e.g. `./events/year=2024/`) are not yet
  supported as a single logical source. As a workaround, use `sq sql` against a DuckDB source
  with DuckDB's recursive glob syntax (see the
  [DuckDB Parquet docs](https://duckdb.org/docs/data/parquet/overview.html)), for example
  `read_parquet('events/*.parquet')` for a single directory.
