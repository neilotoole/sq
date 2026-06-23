# Parquet (`parquet` driver)

[Apache Parquet](https://parquet.apache.org) columnar binary files. **Read-only** document source.
Backed by DuckDB; the bundled `parquet` and `httpfs` extensions handle local and remote files with
column- and predicate-pushdown.

**Canonical docs:** [Parquet driver](https://sq.io/docs/drivers/parquet/)

## Add a source

```shell
sq add ./events.parquet
sq add https://example.com/data.parquet
sq add --driver=parquet ./ambiguous.bin
```

Detection uses the `PAR1` magic bytes at both ends of the file. Use `--driver=parquet` when the
extension is missing or ambiguous.

Piping works too (`cat events.parquet | sq '.data'`); stdin is buffered to a temp file since
Parquet isn't streamable.

## Monotable

Data is accessed via the synthetic **`.data`** table, e.g. `@handle.data`.

## Connection options

For local files, DuckDB options are forwarded via the location query string:

```shell
sq add './big.parquet?threads=4&memory_limit=2GB'
```

For remote URLs the query string belongs to the URL (presigned S3 URLs etc.), so the
above syntax is not available. Use env vars or `sq config set` for those.

## Remote files

HTTP/HTTPS URLs are handled by DuckDB's `httpfs` extension. AWS env vars
(`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`) configure S3 access; explicit
per-source S3 options are not yet supported.

## Limitations

- Read-only.
- Single file per source. Partitioned (directory-based) datasets are not yet supported.
