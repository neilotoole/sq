# Package `jsonw`

Package `jsonw` implements JSON output writers.

Note that there are three implementations of `output.RecordWriter`.

- `NewStdRecordWriter` returns a writer that outputs in "standard" JSON.
- `NewArrayRecordWriter` outputs each record on its own line as an element of a JSON array.
- `NewObjectRecordWriter` outputs each record as a JSON object on its own line.

These `RecordWriter`s correspond to the `--json`, `--jsona`, and `--jsonl` flags
(where `jsonl` means "JSON Lines"). There are also other writer implementations,
such as an `output.ErrorWriter` and an `output.MetadataWriter`.

## Standard JSON `--json`

```json
[
  {
    "actor_id": 1,
    "first_name": "PENELOPE",
    "last_name": "GUINESS",
    "last_update": "2006-02-15T04:34:33Z"
  },
  {
    "actor_id": 2,
    "first_name": "NICK",
    "last_name": "WAHLBERG",
    "last_update": "2006-02-15T04:34:33Z"
  }
]
```

## JSON Array `--jsona`

```json
[1, "PENELOPE", "GUINESS", "2006-02-15T04:34:33Z"]
[2, "NICK", "WAHLBERG", "2006-02-15T04:34:33Z"]
```

## Object aka JSON Lines `--jsonl`

```json
{"actor_id": 1, "first_name": "PENELOPE", "last_name": "GUINESS", "last_update": "2006-02-15T04:34:33Z"}
{"actor_id": 2, "first_name": "NICK", "last_name": "WAHLBERG", "last_update": "2006-02-15T04:34:33Z"}
```

## Notes

At the time of development there was not a JSON encoder library available that provided the
functionality that `sq` required. These requirements:

- Optional colorization
- Optional pretty-printing (indentation, spacing)
- Preservation of the order of record fields (columns).

For the `RecordWriter` implementations, given the known "flat" structure of a record, it was
relatively straightforward to create custom writers for each type of JSON output.

For general-purpose JSON output (such as metadata output), `jsonw` uses
[`github.com/neilotoole/jsoncolor`](https://github.com/neilotoole/jsoncolor),
a color-capable JSON encoder forked from
[`segmentio/encoding`](https://github.com/segmentio/encoding). `jsoncolor`
was extracted from sq's own in-tree fork (`jsonw/internal/jcolorenc`, now
removed) and is now maintained as a standalone library.
