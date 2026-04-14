---
title: Inspect
description: Inspect source or table metadata
lead: ''
draft: false
images:
weight: 1037
toc: true
url: /docs/inspect
---
[`sq inspect`](/docs/cmd/inspect) inspects metadata (schema/structure, tables, columns) for a source,
or for an individual table. When used with `--json`, the output of `sq inspect` can
be fed into other tools such as [jq](https://jqlang.github.io/jq/) to enable complex data pipelines.

Let's start off with a single source, a Postgres [Sakila](/docs/develop/sakila/) database:

```shell
# Start the Postgres container
$ docker run -d -p 5432:5432 sakiladb/postgres:12

# Add the source
$ sq add postgres://sakila:p_ssW0rd@localhost/sakila --handle @sakila_pg
@sakila_pg  postgres  sakila@localhost/sakila
```

## Inspect source

Use `sq inspect @sakila_pg` to inspect the source.

{{< alert icon="👉" >}}
You can also use `sq inspect` with `stdin`, e.g.:

```shell
$ cat actor.csv | sq inspect
```

However, note that `stdin` sources can't take advantage of [ingest caching](/docs/source#ingest), because
the `stdin` pipe is "anonymous", and `sq` can't do a cache lookup for it. If you're going to
repeatedly inspect the same `stdin` data, you should probably just [`sq add`](#add) it.
{{< /alert >}}

This output includes the source
metadata, and the schema structure (tables, columns, etc.).

### `--text` (default)

```shell
$ sq inspect @sakila_pg
```
![sq inspect source text](sq_inspect_source_text.png)

### `--verbose`

To see more detail, use the `--verbose` (`-v`) flag with the `--text` format.

![sq inspect source verbose](sq_inspect_source_text_verbose.png)

### `--yaml`

To see the full output, use the `--yaml` (`-y`) flag. YAML has the advantage
of being reasonably human-readable.

![sq inspect source yaml](sq_inspect_source_yaml.png)

{{< alert icon="⚠️" >}}
If the schema is large and complex, it can take some time (a few seconds or longer)
for `sq` to introspect the schema.
{{< /alert >}}

### `--json`

The `--json` (`-j`) format renders the same content as `--yaml`, but is more
suited for use with other tools, such as [jq](https://jqlang.github.io/jq/).

![sq_inspect_source json](sq_inspect_source_json.png)

Here's an example of using `sq` with jq to list all table names:

```shell
$ sq inspect -j | jq -r '.tables[] | .name'
```

![sq_inspect_pipe_jq_table_names](sq_inspect_pipe_jq_table_names.png)

See more examples in the [cookbook](/docs/cookbook).

## Source overview

Sometimes you don't need the full schema, but still want to view the source
metadata. Use the `--overview` (`-O`) mode to see just the top-level metadata.
This excludes the schema structure, and is also much faster to complete.


![sq inspect overview text](sq_inspect_source_overview_text.png)

Well, that's not a lot of detail. The `--yaml` output is more useful:

![sq_inspect_overview_yaml](sq_inspect_source_overview_yaml.png)

The `--json` format produces similar output.

## Database properties

The `--dbprops` mode displays the underlying database's properties, server config,
and the like.

```shell
$ sq inspect @sakila_pg --dbprops
```

![sq_inspect_source_dbprops_pg_text](sq_inspect_source_dbprops_pg_text.png)

Use `--dbprops` with `--yaml` or `--json` to get the properties in machine-readable
format. Note that while the returned structure is generally a set of key-value
pairs, the specifics can vary significantly from one driver type to another.
Here's `--dbprops` from a [SQLite](/docs/drivers/sqlite/) database (in `--yaml` format):

![sq inspect source dbprops sqlite yaml](sq_inspect_source_dbprops_sqlite_yaml.png)

## Catalogs

The `--catalogs` mode lists the [catalogs](/docs/concepts#schema--catalog) (databases)
available in the source.

![sq inspect source catalogs pg yaml](sq_inspect_source_catalogs_yaml.png)

## Schemata

Like `--catalogs`, the `--schemata` mode lists the [schemas](/docs/concepts#schema--catalog)
available in the source.

![sq inspect source schemata pg yaml](sq_inspect_source_schemata_yaml.png)

To list the schemas in a specific catalog, supply `CATALOG.` to the
[`--src.schema`](/docs/source#source-override) flag:

```shell
# List the schemas in the "inventory" catalog.
$ sq inspect @sakila/pg12 --schemata --src.schema inventory.
````


## Inspect table

In additional to inspecting a source, you can drill down on a specific table.

```shell
$ sq inspect @sakila_pg.actor
```
![sq inspect table text](sq_inspect_table_text.png)

Use `--verbose` mode for more detail:

![sq inspect table text verbose](sq_inspect_table_text_verbose.png)

And, as you might expect, you can also see the output in `--json` and `--yaml` formats.

![sq inspect table json](sq_inspect_table_json.png)

{{< alert icon="👉" >}}
Note that the `--overview` and `--dbprops` flags apply only to inspecting sources,
not tables.
{{< /alert >}}

## Override active schema

By default, `sq inspect` uses the active [schema](/docs/concepts#schema--catalog)
for the source. You can override the active schema (and catalog)
using the [`--src.schema`](/docs/source#source-override) flag. See the [sources](/docs/source#source-override) section
for a fuller explanation of `--src.schema`, but here's a quick example of
inspecting Postgres's `information_schema` schema:

```shell
$ sq inspect @sakila/pg12 --src.schema sakila.information_schema
SOURCE        DRIVER    NAME    FQ NAME                    SIZE    TABLES  VIEWS  LOCATION
@sakila/pg12  postgres  sakila  sakila.information_schema  16.6MB  7       61     postgres://sakila:xxxxx@192.168.50.132/sakila

NAME                                   TYPE   ROWS   COLS
sql_features                           table  716    feature_id, feature_name, sub_feature_id, sub_feature_name, is_supported, is_verified_by, comments
sql_implementation_info                table  12     implementation_info_id, implementation_info_name, integer_value, character_value, comments
sql_languages                          table  4      sql_language_source, sql_language_year, sql_language_conformance, sql_language_integrity, sql_language_implementation, sql_language_binding_style, sql_language_programming_language
sql_packages                           table  10     feature_id, feature_name, is_supported, is_verified_by, comments
sql_parts                              table  9      feature_id, feature_name, is_supported, is_verified_by, comments
sql_sizing                             table  23     sizing_id, sizing_name, supported_value, comments
sql_sizing_profiles                    table  0      sizing_id, sizing_name, profile_id, required_value, comments
_pg_foreign_data_wrappers              view   0      oid, fdwowner, fdwoptions, foreign_data_wrapper_catalog, foreign_da
```
