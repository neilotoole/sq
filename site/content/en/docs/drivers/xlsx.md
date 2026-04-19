---
title: "XLSX (Excel)"
description: "XLSX (Excel)"
draft: false
images: []
weight: 4060
toc: true
url: /docs/drivers/xlsx
---
The `sq` Excel driver implements connectivity
for Microsoft [XLSX](https://www.microsoft.com/en-us/microsoft-365/excel)
files and [variants](#supported-file-formats).

{{< alert icon="👉" >}}
Excel is a [document source](/docs/source#document-source) and thus its data
is [ingested](/docs/source#ingest) and [cached](/docs/source#cache).

Note also that an Excel source is read-only; you can't [insert](/docs/output#insert)
values into the source.
{{< /alert >}}
## Supported file formats

The driver supports `.xlsx`, `.xlam`, `.xlsm`,`.xltm` and `.xltx`. Note that even
if the file format is, say, `.xlam`, the driver type is still `xlsx`. The driver
does not support the older `.xls` and `.xlsb` formats.

## Add source

When adding an XLSX source via [`sq add`](/docs/cmd/add), the location string is simply the filepath.
For example:

```shell
$ sq add ./sakila.xlsx
@sakila_xlsx  xlsx  sakila.xlsx
```

You can also pass an absolute filepath (and, in fact, any relative path is expanded to
an absolute path when saved to `sq`'s config).

{{< alert icon="👉" >}}
The `sq add` command accepts a `--driver=TYPE` flag, e.g. `--driver=xlsx`. However,
in practice this flag can be omitted, because sq can [detect](/docs/detect/#driver-type)
the driver type. {{< /alert >}}


## Worksheets

When an XLSX source is added, `sq` treats each sheet as a separate database table.
Thus a sheet named `actor` can be queried as `@sakila_xlsx.actor`.

Empty sheets are ignored, and can't be queried.

## Header row

Excel sheets will often have a header row containing column names. If the sheet
doesn't have a header row, by default `sq` will name the columns `A`, `B`, `C`, etc.
(Note that the column naming behavior is [configurable](/docs/config/#ingestcolumnrename).

Generally, `sq` will automatically [detect](/docs/detect), for each sheet,
whether or not the first row is a header row. If the header row detection
is having difficulty with your workbook, you can explicitly specify that a
header row is present (or not) via [`--ingest.header`](/docs/config/#ingestheader).

```shell
# Explicitly specify that a header row exists (in each sheet)
$ sq add --ingest.header ./sakila.xlsx

# Explicitly specify no header row
$ sq add --ingest.header=false ./sakila-no-header.xlsx
```

{{< alert icon="👉" >}}
A downside to `--ingest.header` is that the option applies on a per-source basis, not per-sheet.
That is to say: when using `--ingest.header`, every sheet in the workbook
should have a header, or none of the sheets should have a header.
{{< /alert >}}

### Duplicate columns

If the header row has duplicate column names, the later columns are renamed.
For example, these columns:

```text
actor_id, first_name, actor_id
```

become:

```text
actor_id, first_name, actor_id_1
```

The renaming behavior is configurable via [`ingest.column.rename`](/docs/config#ingestcolumnrename).


## Column kind

When ingesting an Excel workbook, `sq` attempts to detect the data ["kind"](/docs/detect/#column-kind)
of each column (`int`, `float`, `text`, etc.). Thus an Excel date becomes a date in
the backing DB, an Excel
number becomes an int or a float, and various date & time values are parsed
into an appropriate DB type. See the [column kind section](/docs/detect/#column-kind)
for more on this mechanism.
