---
title: "sq config keyring rm"
description: "Delete a keyring entry"
group: config
draft: false
images: []
menu:
  docs:
    parent: "cmd"
toc: false
url: /docs/cmd/config-keyring-rm
---
Part of the [`sq config keyring`](/docs/cmd/config-keyring) command group;
see [Secrets](/docs/secrets) for the broader picture. `rm` only deletes the keyring
entry; any `${keyring:PATH}` reference left in `sq.yml` will fail to
resolve at connect time. Run
[`sq config keyring ls`](/docs/cmd/config-keyring-ls) first to find
references.

## Reference

{{< readfile file="config-keyring-rm.help.txt" code="true" lang="text" >}}
