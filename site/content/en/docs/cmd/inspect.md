---
title: "sq inspect"
description: "Inspect data source schema and stats"
draft: false
images: []
menu:
  docs:
    parent: "cmd"
weight: 2060
toc: false
url: /docs/cmd/inspect
---
`sq inspect` inspects metadata (schema/structure, tables, columns) for a source,
or for an individual table. When used with `--json`, the output of `sq inspect` can
be fed into other tools such as [jq ](https://jqlang.github.io/jq/) to enable complex data pipelines.

See the [inspect guide](/docs/inspect) for an overview of `sq inspect`.

![sq_inspect_source_text_verbose.png](/images/sq_inspect_source_text_verbose.png)

## Reference

{{< readfile file="inspect.help.txt" code="true" lang="text" >}}
