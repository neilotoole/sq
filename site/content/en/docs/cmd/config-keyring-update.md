---
title: "sq config keyring update"
description: "Update an existing keyring entry"
draft: false
images: []
menu:
  docs:
    parent: "cmd"
weight: 2038
toc: false
url: /docs/cmd/config-keyring-update
---
Part of the [`sq config keyring`](/docs/cmd/config-keyring) command group;
see [Secrets](/docs/secrets) for the broader picture. Typical use is to rotate a
credential: pass the same `PATH` that already appears in a source's
location, with a new value.

## Reference

{{< readfile file="config-keyring-update.help.txt" code="true" lang="text" >}}
