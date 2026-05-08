# Testing Oracle Driver

How to run Oracle driver tests in this repo. The Go driver is pure Go
([go-ora](https://github.com/sijms/go-ora)); **Oracle Instant Client is not
required**.

## TL;DR

**Unit tests (no database):**

```bash
cd drivers/oracle
go test -v -short
```

**Integration tests (Docker):**

```bash
cd drivers/oracle/testutils
./test-integration.sh
```

With Postgres for cross-database tests:

```bash
./test-integration.sh --with-pg
```

Compose defaults to `sakiladb/oracle:latest` (Sakila user `sakila`,
password `p_ssW0rd`) plus optional Postgres.
If pull access is denied, scripts auto-build the image from
`github.com/sakiladb/oracle`.
Override the Oracle image with `ORACLE_IMAGE=...` if needed, and override the
integration DSN with `SQ_TEST_ORACLE_DSN`.

## Repo-wide (`testh`) Oracle Sakila

For [`libsq/driver`](../../../libsq/driver) and related packages, set:

```bash
export SQ_TEST_SRC__SAKILA_ORA=localhost:1521/FREEPDB1
```

Use any reachable host/port/service that matches
[`testh/testdata/sources.sq.yml`](../../../testh/testdata/sources.sq.yml) handle
`@sakila_ora`. Easiest database:
[`docker run -p 1521:1521 sakiladb/oracle:latest`](https://github.com/sakiladb/oracle).

Example narrowed runs:

```shell
go test ./libsq/driver/... -run 'Oracle|SourceMetadata_Oracle'
go test ./drivers/oracle/...
```

## Scripts

| Script | Purpose |
| ------ | ------- |
| [`test-integration.sh`](./test-integration.sh) | Compose up → `go test` in `drivers/oracle` |
| [`test-sq-cli.sh`](./test-sq-cli.sh) | End-to-end checks via `sq` CLI — see [TEST_SQ_CLI.md](./TEST_SQ_CLI.md) |

## Compose profiles

Default [`docker-compose.yml`](./docker-compose.yml):

- **oracle** — `${ORACLE_IMAGE:-sakiladb/oracle:latest}`
- **postgres** — started when passing `--with-pg` to `test-integration.sh`

First startup can take a few minutes while the healthcheck passes.

To run explicitly against `sakiladb/oracle`, for example:

```bash
ORACLE_IMAGE=sakiladb/oracle:latest \
ORACLE_HEALTHCHECK_DSN=sakila/p_ssW0rd@//localhost:1521/FREEPDB1 \
SQ_TEST_ORACLE_DSN=oracle://sakila:p_ssW0rd@localhost:1521/FREEPDB1 \
./test-integration.sh
```

## Troubleshooting

**Tests skip with “Oracle database not reachable”**

- Ensure Docker is running and `docker compose up -d` (from `testutils/`)
  succeeds.
- Wait until `docker compose ps` shows the oracle service healthy.
- Confirm `SQ_TEST_ORACLE_DSN` matches your listener (default uses
  `localhost:1521` and PDB `FREEPDB1`).

**Port 1521 already in use**

- Stop other Oracle containers or change the host port mapping in
  `docker-compose.yml`.

## Further reading

- [Oracle driver README](../README.md)
- [SQ CLI Oracle testing](./TEST_SQ_CLI.md)
