---
title: "sq config keyring get"
description: "Print metadata or value for a keyring entry"
group: config
draft: false
images: []
menu:
  docs:
    parent: "cmd"
toc: false
url: /docs/cmd/config-keyring-get
---
Part of the [`sq config keyring`](/docs/cmd/config-keyring) command group;
see [Secrets](/docs/secrets) for the broader picture. By default this
prints only that the entry exists; pass [`--reveal`](/docs/secrets#redaction)
to print the stored value.

## Reference

{{< readfile file="config-keyring-get.help.txt" code="true" lang="text" >}}
