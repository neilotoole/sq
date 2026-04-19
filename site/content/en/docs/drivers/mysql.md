---
title: "MySQL"
description: "MySQL driver"
draft: false
images: []
weight: 4010
toc: false
url: /docs/drivers/mysql
---
The `sq` MySQL driver implements connectivity for
the [MySQL](https://www.mysql.com) and [MariaDB](https://mariadb.org) databases.
The driver implements all optional driver features.

## Add source

Use [`sq add`](/docs/cmd/add) to add a source. The location argument should start
with `mysql://`. For example:

```shell
sq add 'mysql://sakila:p_ssW0rd@localhost/sakila'
```
