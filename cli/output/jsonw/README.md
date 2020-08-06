# Package `jsonw`

Package `jsonw` implements JSON output writers.

Note that there are three implementations of `output.RecordWriter`.

- `NewStdRecordWriter` returns a writer that outputs in "standard" JSON.
- `NewArrayRecordWriter` outputs each record on its own line as an element of a JSON array.
- `NewObjectRecordWriter` outputs each record as a JSON object on its own line.

These `RecordWriter`s correspond to the `--json`, `--jsona`, and `--jsonl` flags (where `jsonl` means "JSON Lines"). There are also other writer implementations, such as an `output.ErrorWriter` and an `output.MetadataWriter`.


#### Standard JSON `--json`:

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

#### JSON Array `--jsona`:

```json
[1, "PENELOPE", "GUINESS", "2020-06-11T02:50:54Z"]
[2, "NICK", "WAHLBERG", "2020-06-11T02:50:54Z"]
```

#### Object aka JSON Lines `--jsonl`: 

```json
{"actor_id": 1, "first_name": "PENELOPE", "last_name": "GUINESS", "last_update": "2020-06-11T02:50:54Z"}
{"actor_id": 2, "first_name": "NICK", "last_name": "WAHLBERG", "last_update": "2020-06-11T02:50:54Z"}
```

## Notes

At the time of development there was not a JSON encoder library available that provided the functionality that `sq` required. These requirements:

- Optional colorization
- Optional pretty-printing (indentation, spacing)
- Preservation of the order of record fields (columns).

For the `RecordWriter` implementations, given the known "flat" structure of a record, it was relatively straightforward to create custom writers for each type of JSON output.

For general-purpose JSON output (such as metadata output), it was necessary to modify an existing JSON library to provide colorization (and also on-the-fly indentation). After benchmarking, the [segmentio.io encoder](https://github.com/segmentio/encoding) was selected as the base library. Rather than a separate forked project (which probably would not make sense to ever merge with its parent project), the modified encoder is found in `jsonw/internal/jcolorenc`. 
