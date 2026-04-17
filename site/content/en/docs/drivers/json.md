---
title: "JSON"
description: "JSON"
draft: false
images: []
weight: 4055
toc: true
url: /docs/drivers/json
---
The `sq` JSON drivers implement connectivity
for [JSON](https://en.wikipedia.org/wiki/JSON), JSON Array, and [JSON Lines](https://en.wikipedia.org/wiki/JSON_streaming#Newline-Delimited_JSON) data sources.

{{< alert icon="👉" >}}
JSON is a [document source](/docs/source#document-source) and thus its data
is [ingested](/docs/source#ingest) and [cached](/docs/source#cache).

Note also that a JSON source is read-only; you can't [insert](/docs/output#insert)
values into the source.
{{< /alert >}}

## Variants

### JSON

The JSON (`json`) driver expects an array of JSON objects. For example:

```json
[
  {
    "actor_id": 1,
    "first_name": "PENELOPE",
    "last_name": "GUINESS",
    "last_update": "2020-06-11T02:50:54Z"
  },
  {
    "actor_id": 2,
    "first_name": "NICK",
    "last_name": "WAHLBERG",
    "last_update": "2020-06-11T02:50:54Z"
  }
]
```

### JSON Array

The JSON Array (`jsona`) driver expects newline-delimited lines of JSON array data. For example:

```json lines
[1, "PENELOPE", "GUINESS", "2020-06-11T02:50:54Z"]
[2, "NICK", "WAHLBERG", "2020-06-11T02:50:54Z"]
```

### JSON Lines

The JSON Lines (`jsonl`) driver expects newline-delimited lines, where each line is a JSON object.
For example:

```json lines
{"actor_id": 1, "first_name": "PENELOPE", "last_name": "GUINESS", "last_update": "2020-06-11T02:50:54Z"}
{"actor_id": 2, "first_name": "NICK", "last_name": "WAHLBERG", "last_update": "2020-06-11T02:50:54Z"}
```

## Monotable

`sq` considers JSON to be a _monotable_ data source (unlike, say, a Postgres data source, which
obviously can have many tables). Like all other `sq` monotable sources,
the source's data is accessed via the synthetic `.data` table. For example:

```shell
$ sq @actor_json.data
actor_id  first_name   last_name     last_update
1         PENELOPE     GUINESS       2020-02-15T06:59:28Z
2         NICK         WAHLBERG      2020-02-15T06:59:28Z
```

## Add source

When adding a JSON source via [`sq add`](/docs/cmd/add), the location string is simply the filepath.
For example:

```shell
$ sq add ./actor.json
@actor_json  json  actor.json
```

You can also pass an absolute filepath (and, in fact, any relative path is expanded to
an absolute path when saved to `sq`'s config).

When adding a JSON source, `sq` will usually figure out if a file is `json`, `jsona`, or `jsonl`.
However, if necessary, you can explicitly specify the JSON variant when adding the source.
For example:

```shell
$ sq add --driver json ./actor.json
$ sq add --driver jsona ./actor.jsona
$ sq add --driver jsonl ./actor.jsonl
```

## Nested data

When `sq` encounters nested JSON, it flattens the nested data, using underscores to
build the flattened field names. For example, given this data:

```json
[
  {
    "actor_id": 1,
    "first_name": "PENELOPE",
    "last_name": "GUINESS",
    "last_update": "2020-06-11T02:50:54Z",
    "address": {
      "city": "Galway",
      "country": "Ireland"
    }
  },
  {
    "actor_id": 2,
    "first_name": "NICK",
    "last_name": "WAHLBERG",
    "last_update": "2020-06-11T02:50:54Z"
  }
]
```

`sq` will flatten `address.city` into an `address_city` field.

```shell
$ sq @actor_json.data
actor_id  first_name  last_name  last_update           address_city  address_country
1         PENELOPE    GUINESS    2020-06-11T02:50:54Z  Galway        Ireland
2         NICK        WAHLBERG   2020-06-11T02:50:54Z  NULL          NULL
```

{{< alert icon="⚠️" >}}
The JSON driver implementations have not been extensively battle-tested
with very complex JSON documents. If you find that `sq` is not able to handle your JSON data,
please [open a bug](https://github.com/neilotoole/sq/issues/new), attaching a sample JSON file.
{{< /alert >}}
