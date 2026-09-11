# sq drivers

This directory holds the concrete `sq` driver implementations: one package per
datasource type (Postgres, MySQL, CSV, JSON, etc.). In `sq` parlance, a "driver"
implements a datasource type.

The full developer documentation lives under [`docs/`](../docs):

- **Driver development guide:** [`docs/DRIVERS.md`](../docs/DRIVERS.md), covering
  how to add and maintain a driver: package structure, type mapping, the
  [driver ship checklist](../docs/DRIVERS.md#driver-ship-checklist), test
  handles, and the SQL vs document driver split.
- **Architecture:** [`docs/ARCHITECTURE.md`](../docs/ARCHITECTURE.md) shows how
  drivers fit into the overall `sq` architecture.
- **User docs:** [sq.io drivers section](https://sq.io/docs/drivers).

Some drivers carry their own README with maintainer notes (implementation
quirks, upstream-driver workarounds), e.g.
[`clickhouse/README.md`](clickhouse/README.md).
