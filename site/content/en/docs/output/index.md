---
title: Output
description: Output
lead: ''
draft: false
images: []
weight: 1039
toc: true
url: /docs/output
---
`sq` can output in many formats, e.g. `text` or `json`. It can also write
results to a database, using [`--insert`](#insert). The output format
can be specified using command-line flags (e.g. `--text`, `--json` etc.), or
it can be set using config. The default is `text`. As an alternative to the
shorthand forms, you can also use `--format text` etc.

```shell
# Execute a query, and output in CSV
$ sq '.actor | .first_name, .last_name' --csv

# Alternative --format flag
$ sq '.actor | .first_name, .last_name' --format csv

# Check default format
$ sq config get format

# Set config format
$ sq config set format json

# View list of output formats
$ sq config set format --help
...
Available formats:

  text, csv, tsv, xlsx,
  json, jsona, jsonl,
  markdown, html, xlsx, xml, yaml, raw
```

The output format applies to queries (e.g. `sq .actor --json`), and also to
other `sq` commands, e.g. `sq inspect @sakila --yaml`. Not every
command implements each format. For example, there's no `markdown` output format
for `sq version`. But every command (except for `help`) supports at least `text`
and `json` output.

## Modifiers

### verbose

The `--verbose` (`-v`) flag does not affect the output of a `sq` query, but
it frequently modifies the behavior of other `sq` commands.

![sq -v](sq_verbose.png)

`-v` works with a significant number of `sq` commands. Give it a try.
It can also be set via [config](/docs/config#verbose).


### header

Some formats optionally display a header row. This is controlled via
`--header` (`-h`) or `--no-header` (`-H`).
Or set via [config](/docs/config#header). The default is to print the header.

![sq query header](sq_query_header.png)

### compact

For some formats, the `--compact` (`-c`) flag prints compact instead of
pretty-printed output. It can also be set via [config](/docs/config#compact).

JSON is the main use case for `--compact`. This example outputs a query in compact JSON (`-jc`), followed by the same
query in pretty JSON.

![sq query -jc](sq_query_json_compact.png)

### monochrome

Use `--monochrome` (`-M`) flag to output without color. Or set via [config](/docs/config#monochrome).

![sq query -M](sq_query_monochrome.png)


### datetime

By default, `sq` outputs timestamps in an [IS08601](https://en.wikipedia.org/wiki/ISO_8601)
format, in [UTC](https://en.wikipedia.org/wiki/Coordinated_Universal_Time), e.g. `2020-06-11T02:50:54Z`.

You can use [`--format.datetime`](/docs/config/#formatdatetime) to specify a [pre-defined](/docs/config/#formatdatetime)
format such as `unix` or `RFC3339`. Or you can supply an arbitrary
[`strftime`](https://pubs.opengroup.org/onlinepubs/009695399/functions/strftime.html)
format, such as `%Y/%m/%d %H:%M:%S`.

![sq query datetime](sq_query_format_datetime.png)

Similarly [`--format.date`](/docs/config/#formatdate)
and [`--format.time`](/docs/config/#formattime) control the rendering of
date and time values.


{{< alert icon="👉" >}}
Microsoft Excel uses its own format string mechanism,
thus the [`xlsx`](#xlsx) format has separate but equivalent options:
[`--format.excel.datetime`](/docs/config/#formatexceldatetime),
[`--format.excel.date`](/docs/config/#formatexceldate) and [`--format.excel.time`](/docs/config/#formatexceltime)

{{< /alert >}}

There are yet more formatting options available. Check out the full list
in the [config guide](/docs/config/#output).

## Formats

### text

`text` (`-t`) is the default format.

![sq query --text](sq_query_text.png)

### json

`json` (`-j`) outputs an array of JSON objects. Use `-c` (`--compact`) to output
compact instead of pretty-printed JSON.

![sq query --json](sq_query_json.png)

### jsona

`jsona` (`-A`) outputs JSON Array. This is LF-delimited JSON arrays of values, without keys.

![sq query --jsona](sq_query_jsona.png)

### jsonl

`jsonl` (`-J`) outputs JSON Lines. This is LF-delimited JSON objects.

![sq query --jsonl](sq_query_jsonl.png)


<a id="tsv" />

<a id="csv" />

### csv, tsv

`csv` (`-C`) outputs [Comma-Separated Values](https://en.wikipedia.org/wiki/Comma-separated_values).
Its twin `tsv` (`-T`) outputs [Tab-Separated Values](https://en.wikipedia.org/wiki/Tab-separated_values).

![sq query csv](sq_query_csv_tsv.png)


### markdown

`markdown` outputs markdown tables.

![sq query --markdown](sq_query_markdown.png)


### html

`html` outputs a table in a HTML document.

![sq query --html](sq_query_html.png)


### xml

`xml` (`-X`) outputs an XML document.

![sq query --xml](sq_query_xml.png)

### xlsx

`xlsx` (`-x`) outputs an Excel `.xlsx` document.

![sq query --xlsx](sq_query_xlsx.png)

There are three config options for controlling date/time output.
Note that these format strings are distinct from [`format.datetime`](https://sq.io/docs/config#formatdatetime)
and friends, because Excel has its own format string mechanism.
- [`format.excel.datetime`](/docs/config#formatexceldatetime): Controls datetime format, e.g. `2023-08-03 16:07:01`.
- [`format.excel.date`](/docs/config#formatexceldatetime): Controls date-only format, e.g. `2023-08-03`.
- [`format.excel.time`](/docs/config#formatexceldatetime): Controls time-only format, e.g. `4:07 pm`.

### yaml

`yaml` (`-y`) outputs YAML.

![sq query --yaml](sq_query_yaml.png)


### raw

`--raw` outputs each record field in raw format without any encoding or delimiter.

![sq query --raw](sq_query_raw.png)

This is more commonly used with `BLOB` fields.

![sq query --raw image](sq_query_raw_image.png)

Typically you want to send raw output to a file.

```shell
$ sq '.images | .data | .[0]' --raw > gopher.gif; open gopher.gif
```

![sq query --raw gopher](sq_query_raw_image_gopher.png)

{{< alert icon="👉" >}}
On macOS, a handy trick is to pipe `BLOB` output directly to `Preview.app`.

```shell
$ sq '.images | .data | .[0]' --raw | open -f -a Preview.app
```
{{< /alert >}}


## Insert

Use the `--insert @SOURCE.TABLE` flag to write records to a table. This
powerful mechanism can be used to move data from one source to another.
If the named table does not exist, it is created.

```shell
$ sq '.actor | .[0:2]' --insert @sakila/pg12.actor_import
Inserted 2 rows into @sakila/pg12.actor_import
```

![sq query --insert](sq_query_insert.png)

