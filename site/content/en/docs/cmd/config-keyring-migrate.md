---
title: "sq config keyring migrate"
description: "Migrate inline-credential sources to the keyring"
draft: false
images: []
menu:
  docs:
    parent: "cmd"
weight: 2038
toc: false
url: /docs/cmd/config-keyring-migrate
---
See [Secrets](/docs/secrets#managing-keyring-entries) for an overview of
the `sq config keyring` command group. `migrate` is a bulk operation
that walks the source collection, writes each inline-password conn string to a
fresh opaque keyring ID, and replaces the YAML location with a bare
`${keyring:<id>}` placeholder. Use `--dry-run` first to preview.

## Reference

{{< readfile file="config-keyring-migrate.help.txt" code="true" lang="text" >}}
