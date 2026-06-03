---
title: "sq config keyring get"
description: "Print metadata or value for a keyring entry"
draft: false
images: []
menu:
  docs:
    parent: "cmd"
weight: 2038
toc: false
url: /docs/cmd/config-keyring-get
---
See [Secrets](/docs/secrets#managing-keyring-entries) for an overview of
the `sq config keyring` command group. By default this prints only that
the entry exists; pass [`--reveal`](/docs/secrets#redact--reveal)
to print the stored value.

## Reference

{{< readfile file="config-keyring-get.help.txt" code="true" lang="text" >}}
