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
Part of the [`sq config keyring`](/docs/cmd/config-keyring) command group;
see [Secrets](/docs/secrets) for the broader picture. Most users let
[`sq add --store keyring`](/docs/cmd/add) create entries automatically;
this command is for hand-crafted paths used by composition or shared
keyring references.

## Reference

{{< readfile file="config-keyring-create.help.txt" code="true" lang="text" >}}
