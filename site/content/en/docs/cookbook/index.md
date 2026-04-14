---
title: "Cookbook"
description: "Recipes for various tasks"
draft: false
images: []
weight: 1080
toc: true
url: /docs/cookbook
---
This cookbook demonstrates how to perform various tasks using `sq`.

- If not otherwise specified, assume the `@sakila_sl3` data source is present.
  See [here](/docs/develop/sakila#sqlite) to add that source.
- The snippets were created on macOS (using `zsh`).
- Some of these examples require [jq ](https://jqlang.github.io/jq/).

## List table names

List the tables of the active source:

```shell
$ sq inspect -j | jq -r '.tables[] | .name'
actor
address
category
[...]
```

List the tables of a named source (`@sakila_sl3` in this example):

```shell
$ sq inspect @sakila_sl3 -j | jq -r '.tables[] | .name'
actor
address
category
[...]
```

## Export all table data to CSV

With `@sakila_sl3` as the active source:

```shell
$ sq inspect -j | jq -r '.tables[] | .name' | xargs -I % sq .% --csv --output %.csv

$ ls
actor.csv    category.csv  country.csv   film.csv        film_category.csv  inventory.csv  payment.csv  staff.csv
address.csv  city.csv      customer.csv  film_actor.csv  film_text.csv      language.csv   rental.csv   store.csv
```

The above snippet will:

- invoke `sq inspect` on the active source (`@sakila_sl3`) and output in JSON
- pipe that JSON to jq, and filter for just the table names
- pipe those table names to `xargs`
- `xargs` invokes `sq .% --csv --output %.csv` for each table name (e.g. `sq .actor --csv --output actor.csv`)
- thus the content of each table is outputted in CSV format to an individual `.csv` file.


## Import JSON Array to database

```shell
$ wget https://github.com/neilotoole/sq/raw/master/drivers/json/testdata/actor.jsona

$ cat actor.jsona
[1, "PENELOPE", "GUINESS", "2020-06-11T02:50:54Z"]
[2, "NICK", "WAHLBERG", "2020-06-11T02:50:54Z"]
[3, "ED", "CHASE", "2020-06-11T02:50:54Z"]
[...]

$ sq add actor.jsona
@actor_jsona  jsona  actor.jsona

# Insert the first 10 rows from that JSONA source into a new table "actor_jsona" in the SQLite source "@sakila_sl3"
$ sq '@actor_jsona.data | .[0:10]' --insert @sakila_sl3.actor_jsona
Inserted 10 rows into @sakila_sl3.actor_jsona
```

## Import JSON Lines to database

```shell
$ wget https://github.com/neilotoole/sq/raw/master/drivers/json/testdata/actor.jsonl

$ cat actor.jsonl
{"actor_id": 1, "first_name": "PENELOPE", "last_name": "GUINESS", "last_update": "2020-06-11T02:50:54Z"}
{"actor_id": 2, "first_name": "NICK", "last_name": "WAHLBERG", "last_update": "2020-06-11T02:50:54Z"}
{"actor_id": 3, "first_name": "ED", "last_name": "CHASE", "last_update": "2020-06-11T02:50:54Z"}
[...]

$ sq add actor.jsonl
@actor_jsonl  jsonl  actor.jsonl

# Insert the first 10 rows from that JSONL source into a new table "actor_jsonl" in the SQLite source "@sakila_sl3"
$ sq '@actor_jsonl.data | .[0:10]' --insert @sakila_sl3.actor_jsonl
Inserted 10 rows into @sakila_sl3.actor_jsonl
```

## Access macOS Messages.app DB

Being that macOS `Messages.app` uses SQLite to store its data, you can use `sq`
to poke around.

Here we'll count the number of messages sent and received over the years.

```shell
$ cat ~/Library/Messages/chat.db | sq '.message | count'
count
215439
```

I'm sure millenials will scoff at that amateur number 👴🏻.

{{< alert icon="👉" >}}
Note that you'll need to enable macOS [Full Disk Access](https://spin.atomicobject.com/search-imessage-sql/)
to read the `chat.db` file.
{{< /alert >}}

You can also pipe `chat.db` to `sq inspect` for a look at the schema.

![sq_inspect_macos_messages_chatdb.png](sq_inspect_macos_messages_chatdb.png)

{{< alert icon="👉" >}}
Like any other data source, if you find yourself piping `chat.db` to `sq` often,
just add it as a source.

```shell
$ sq add ~/Library/Messages/chat.db
@chat  sqlite3  chat.db

$ sq '@chat | .message | where(.is_from_me > 0) | count'
count
96294
```

{{< /alert >}}
