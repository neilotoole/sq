---
name: Bug report
about: Create a report to help us improve
title: ''
labels: ''
assignees: ''

---

**Describe the bug**
A clear and concise description of what the bug is.

**To Reproduce**

Steps to reproduce the behavior.

**Expected behavior**

A clear and concise description of what you expected to happen.

**`sq` version**

Paste the output of `sq version --yaml` into the code block below:

```yaml
# $ sq version --yaml
# REPLACE THIS WITH YOUR OUTPUT
version: v0.39.1
commit: 82dc378
timestamp: 2023-06-22T18:31:25Z
latest_version: v0.39.1
host:
  platform: darwin
  arch: arm64
  kernel: Darwin
  kernel_version: 22.5.0
  variant: macOS
  variant_version: "13.4"
```

**Source details**

If your issue pertains to a particular source (e.g. a Postgres database),
paste the output of `sq inspect --overview --yaml @your_source` into the
code block below. You may redact any sensitive fields.

```yaml
# $ sq inspect --overview --yaml @your_source
# REPLACE THIS WITH YOUR OUTPUT
handle: "@your_source"
location: postgres://sakila:xxxxx@192.168.50.132/sakila
name: sakila
name_fq: sakila.public
schema: public
driver: postgres
db_driver: postgres
db_product: PostgreSQL 12.13 on x86_64-pc-linux-musl, compiled by gcc (Alpine 12.2.1_git20220924-r4) 12.2.1 20220924, 64-bit
db_version: "12.13"
user: sakila
size: 17359727
```

**Logs**

If appropriate, attach your `sq` [log file](https://sq.io/docs/config/#logging).

Don't paste the log file contents into a GitHub comment: instead, attach the file.
The exception is if you know for sure that only a particular snippet of
the log file is relevant: then you can paste that short snippet. Be sure
to enclose it in a code block.

**Screenshots**

If applicable, add screenshots to help explain your problem.

**Additional context**

Add any other context about the problem here.
