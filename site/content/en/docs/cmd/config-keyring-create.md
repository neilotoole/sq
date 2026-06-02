---
title: "sq config keyring create"
description: "Create a new keyring entry"
draft: false
images: []
menu:
  docs:
    parent: "cmd"
weight: 2038
toc: false
url: /docs/cmd/config-keyring-create
---
See [Secrets](/docs/secrets#managing-keyring-entries) for an overview of
the `sq config keyring` command group. Most users let
[`sq add --store keyring`](/docs/cmd/add) create entries automatically;
this command is for hand-crafted paths used by composition or shared
keyring references.

## Reference

{{< readfile file="config-keyring-create.help.txt" code="true" lang="text" >}}
