---
title: "sq"
description: "Execute SLQ query against data source"
draft: false
images: []
menu:
  docs:
    parent: "cmd"
weight: 2010
toc: true
url: /docs/cmd/sq
---
Use the root `sq` cmd to execute queries against data sources.


## Pipe Data

For file-based sources (such as CSV or XLSX), you can [`sq add`](/docs/cmd/add) the source file,
but you can also use the UNIX pipe mechanism:

```shell
$ cat ./example.xlsx | sq .Sheet1
```

Similarly, you can [`inspect`](/docs/cmd/inspect):

```shell
$ cat ./example.xlsx | sq inspect
```

## Predefined variables

The `--arg` flag passes a value to `sq` as a [predefined variable](/docs/query/#predefined-variables):

```shell
$ sq --arg first "TOM" '.actor | .first_name == $first'
actor_id  first_name  last_name  last_update
38        TOM         MCKELLEN   2020-06-11T02:50:54Z
42        TOM         MIRANDA    2020-06-11T02:50:54Z
```

## Output

`sq` can output results in many formats.

| Flag      | Shorthand | Format                                       |
|-----------|-----------|----------------------------------------------|
| `--table` | `-t`      | Text table                                   |
| `--json`  | `-j`      | JSON                                         |
| `--jsona` | `-A`      | LF-delimited JSON arrays                     |
| `--jsonl` | `-J`      | LF-delimited JSON objects                    |
| `--raw`   | `-r`      | Raw bytes, without any encoding or delimiter |
| `--xlsx`  | `-x`      | Excel XLSX                                   |
| `--csv`   | `-c`      | CSV                                          |
| `--tsv`   | `-T`      | TSV                                          |
| `--md`    |           | Markdown table                               |
| `--html`  |           | HTML table                                   |
| `--xml`   | `-X`      | XML                                          |

By default, `sq` outputs to `stdout`. You can use shell redirection to write
`sq` output to a file:

```shell
$ sq --csv .actor  > actor.csv
```

But you can also use `--output` (`-o`) to specify a file:

```shell
$ sq --csv .actor -o actor.csv
```

## Override active source

As explained in the [sources](/docs/source#active-source) section,
when you [`add`](/docs/cmd/add) your first source, it becomes the active source. That means
that queries without an explicit `@handle` are assumed to refer to the
active source. You can change the active source using [`sq src @othersrc`](/docs/cmd/src).

Sometimes you may want to override the active source just for a single query.
You can do that using the `--src` flag:

```shell
# Show the active source
$ sq src
@staging

$ sq  '.actor | count'
count
200

# Execute the same query, this time against @prod
$ sq --src @prod '.actor | count'
count
199
```

## Override active schema

Similarly, you can override the active [schema](/docs/concepts#schema--catalog) using `--src.schema`. For example,
let's say you have a Postgres source `@customers`, with a schema for each
customer. Use `--src.schema=SCHEMA_NAME` to override the active schema:


```shell
$ sq --src.schema=acme '.orders | count'
count
200

$ sq --src.schema=momcorp '.orders | count'
count
300
```

You can use the `sq` builtin [`schema()`](/docs/query#schema) function to output the active schema:

```shell
$ sq --src.schema=acme 'schema()'
schema()
acme
```

In addition to overriding the active schema, `--src.schema` can also
be used to override the active [catalog](/docs/concepts#schema--catalog)
when used in the `catalog.schema` form.
Let's say you had another catalog (database) named `projects` on the
same Postgres cluster, and a schema named `apollo` in that catalog.
You would specify `catalog.schema` like this:

```shell
$ sq --src.schema=projects.apollo '.missions | count'
count
17
```

There's also an equivalent [`catalog()`](/docs/query#catalog) builtin function
that returns the active catalog:

```shell
$ sq --src.schema=projects.apollo 'catalog(), schema()'
catalog()  schema()
projects   apollo
```

Note that not every database implements catalog support (this includes MySQL
and SQLite). See the driver support [matrix](/docs/concepts#catalog-schema-support).


## Reference

{{< readfile file="sq.help.txt" code="true" lang="text" >}}
