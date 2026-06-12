---
title: "sq config export"
description: "Export config as YAML"
group: config
draft: false
images: []
menu:
  docs:
    parent: "cmd"
toc: false
url: /docs/cmd/config-export
---
`sq config export` dumps the active config to YAML for backups. The export
covers the source collection, config options, and active source/group
state: the same content `sq` reads from its [config file](/docs/config#location).

By default, the export is a faithful copy of the live config:
`${scheme:path}` placeholders are written verbatim and inline values are
dumped as they appear in the file.

```shell
# Export to stdout (placeholders preserved)
$ sq config export

# Export to a file. The output file is created with mode 0600.
$ sq config export -o sq.bak.yml
```

## `--expand`

`--expand` resolves every `${scheme:path}` [placeholder](/docs/secrets#placeholders) (`keyring`,
`env`, `file`) and splices the fetched value into the exported `location`. The result is a fully
self-contained snapshot suitable for moving between machines, at the cost of writing every
referenced secret in plaintext (which is the intent of `--expand` anyway).

Because the exported file is itself a config, its locations are templates: any `$` in a
resolved value is written as
[`$$`](/docs/secrets#literal-dollar-signs), so that the export connects
byte-identically when used as `sq.yml`. To see resolved values in raw literal form, use
the display commands, e.g. `sq ls -v --expand`.

```shell
$ sq config export --expand -o sq.bak.yml
```

If a referenced keyring entry, environment variable, or file is missing,
the export errors with the failing source's handle.

For the broader picture (how `--expand` differs from `--reveal`, the
placeholder grammar, the threat model), see
[Secrets](/docs/secrets#expanding-placeholders).

See the [config](/docs/config) section for an overview of `sq`
configuration.

## Reference

{{< readfile file="config-export.help.txt" code="true" lang="text" >}}
