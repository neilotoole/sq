---
title: "sq config keyring migrate"
description: "Migrate inline-credential sources to the keyring"
group: config
draft: false
images: []
menu:
  docs:
    parent: "cmd"
toc: false
url: /docs/cmd/config-keyring-migrate
---
Part of the [`sq config keyring`](/docs/cmd/config-keyring) command group;
see [Secrets](/docs/secrets) for the broader picture. `migrate` is a bulk operation
that walks the source collection, writes each inline-password conn string to a
fresh opaque keyring ID, and replaces the YAML location with a bare
`${keyring:<id>}` placeholder. Use `--dry-run` first to preview.

## Reference

{{< readfile file="config-keyring-migrate.help.txt" code="true" lang="text" >}}
