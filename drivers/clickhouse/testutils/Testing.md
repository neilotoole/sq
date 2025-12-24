# Testing ClickHouse Driver

This document describes how to run tests for the ClickHouse driver.

## TL;DR

**Run only unit tests (no database required):**

```bash
cd drivers/clickhouse
go test -v -short
```

**Run integration tests (requires Docker):**

```bash
cd drivers/clickhouse/testutils
./test-integration.sh
```

**Run CLI end-to-end tests (requires Docker + sq binary):**

```bash
cd drivers/clickhouse/testutils
./test-sq-cli.sh
```

## Test Organization

Tests are organized into two categories:

### Unit Tests (No Database Required)

Unit tests run with `go test -short` and do not require any database connection. They test:

- Type mapping (Kind → ClickHouse types, ClickHouse types → Kind)
- SQL generation logic
- Error handling

```bash
cd drivers/clickhouse
go test -v -short
```

Expected output: Tests passing with some tests skipped

### Integration Tests (Database Required)

Integration tests require a running ClickHouse database (and optionally Postgres for cross-database tests). They are automatically skipped when using the `-short` flag.

**Automated approach (recommended):**

```bash
cd drivers/clickhouse/testutils
./test-integration.sh
```

The script handles:

- Starting docker-compose services
- Waiting for databases to be healthy
- Running integration tests
- Cleaning up containers

### CLI End-to-End Tests (Database + sq Binary Required)

CLI tests verify the complete end-to-end functionality using the actual `sq` command-line tool.

**Automated approach:**

```bash
cd drivers/clickhouse/testutils
./test-sq-cli.sh
```

## test-integration.sh Script

The `test-integration.sh` script automates the full integration test workflow. It's the recommended way to run integration tests.

### Usage

```bash
./test-integration.sh [OPTIONS]
```

### Options

- `--with-pg`, `--with-postgres` - Start Postgres and run cross-database tests
- `--keep` - Keep containers running after tests (for debugging)
- `--pattern PATTERN` - Run only tests matching PATTERN
- `--timeout DURATION` - Set test timeout (default: 10m)
- `--help`, `-h` - Show help message

### Examples

**Run all ClickHouse integration tests:**

```bash
./test-integration.sh
```

**Run all tests including cross-database:**

```bash
./test-integration.sh --with-pg
```

**Run specific test:**

```bash
./test-integration.sh --pattern TestSmoke
```

**Run tests and keep containers for debugging:**

```bash
./test-integration.sh --keep
# Containers remain running
# Stop manually: docker-compose down
```

### What the Script Does

1. ✓ Checks prerequisites (docker, go, docker daemon)
2. ✓ Starts docker-compose services (clickhouse, and optionally postgres)
3. ✓ Waits for services to become healthy (with timeout)
4. ✓ Runs integration tests with proper timeout
5. ✓ Shows colored output for easy reading
6. ✓ Cleans up containers (unless `--keep` specified)
7. ✓ Exits with proper status code

### Troubleshooting the Script

**Script fails with "Docker daemon is not running":**

- Start Docker Desktop/daemon and try again

**Script times out waiting for ClickHouse:**

- ClickHouse takes 30-60 seconds to initialize on first run
- Check logs: `docker-compose logs clickhouse`
- Try increasing timeout by waiting longer

**Tests fail but containers stop too quickly:**

- Use `--keep` flag to keep containers running
- Inspect logs: `docker-compose logs clickhouse postgres`
- Connect manually to debug

## test-sq-cli.sh Script

The `test-sq-cli.sh` script tests the ClickHouse driver using the actual `sq` CLI binary.

### Usage

```bash
./test-sq-cli.sh
```

Or with custom sq binary:

```bash
SQ_BINARY=/path/to/sq ./test-sq-cli.sh
```

### What It Tests

1. ✓ Basic connectivity and version checking
2. ✓ Schema inspection (`sq inspect`)
3. ✓ SQL queries (`sq sql`)
4. ✓ SLQ queries (sq native query language)
5. ✓ Table operations (create, insert, query, drop)
6. ✓ Querying pre-loaded test data

### SQ Binary Location

The script looks for the `sq` binary in this order:

1. `${SCRIPT_DIR}/sq` (testutils/sq)
2. `${GOPATH}/bin/sq`
3. `sq` in PATH
4. Custom path via `SQ_BINARY` environment variable

**To build sq binary for testing:**

```bash
# From project root
cd /Users/65720/Development/Projects/sq
go build -o drivers/clickhouse/testutils/sq .
```

## Integration Test Requirements

Integration tests require:

1. **ClickHouse database** (provided via Docker Compose)
2. **Docker** and **Docker Compose**
3. **Go** 1.19+ (for running tests)

### Manual Setup: Step 1 - Start ClickHouse Database

```bash
cd drivers/clickhouse/testutils

# Start ClickHouse container
docker-compose up -d

# Check status (wait until 'healthy')
docker-compose ps

# View logs if needed
docker-compose logs -f clickhouse
```

ClickHouse takes **30-60 seconds** to initialize. Wait until the health check shows `healthy`.

Container configuration:

- **Image**: `clickhouse/clickhouse-server:23.8`
- **User**: `testuser`
- **Password**: `testpass`
- **Database**: `testdb`
- **Ports**:
  - `19000` - Native protocol (mapped to localhost:19000)
  - `18123` - HTTP interface (mapped to localhost:18123)
- **Connection**: `clickhouse://testuser:testpass@localhost:19000/testdb`

### Manual Setup: Step 2 - Run Integration Tests

```bash
cd drivers/clickhouse

# Make sure ClickHouse is running and healthy
cd testutils && docker-compose ps && cd ..

# Run all tests
go test -v

# Or run with longer timeout
go test -v -timeout 5m

# Run specific test
go test -v -run TestSmoke
```

Expected output: Tests passing (exact count depends on which tests are enabled)

Integration tests cover:

- Basic connectivity (TestSmoke)
- Table creation and deletion (TestDriver_CreateTable)
- Type mappings with real data (TestTypeMapping*)
- Table column type retrieval (TestDriver_TableColumnTypes)
- Table copying (TestDriver_CopyTable)
- Metadata operations (TestMetadata_*)

## Docker Management Commands

```bash
# From the testutils directory
cd drivers/clickhouse/testutils

# Start ClickHouse (and optionally Postgres)
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f clickhouse
docker-compose logs -f postgres

# Stop databases (keeps data)
docker-compose down

# Stop and remove data volumes (clean slate)
docker-compose down -v

# Restart databases
docker-compose restart
```

## Test Data

The docker-compose setup includes initialization scripts that create sample tables:

- **users** - Sample user records
- **events** - Sample event logs
- **products** - Sample product catalog

These are created via `init-scripts/01-create-tables.sql` on first container startup.

## Troubleshooting

**Problem: Tests skip with "Skipping: requires ClickHouse test instance"**

- This is normal for integration tests when using `-short` flag
- Remove `-short` flag and ensure ClickHouse is running: `docker-compose up -d`

**Problem: Container not healthy**

- Wait longer (ClickHouse takes 30-60 seconds to start)
- Check logs: `docker-compose logs clickhouse`
- Try restarting: `docker-compose restart clickhouse`

**Problem: Connection refused**

- Ensure Docker is running: `docker ps`
- Ensure container is healthy: `docker-compose ps`
- Check port 9000 is available: `lsof -i :9000`
- Check port 8123 is available: `lsof -i :8123`

**Problem: Authentication failed**

- Wait for ClickHouse to fully initialize (check `docker-compose logs`)
- Verify credentials in docker-compose.yml
- Try restarting: `docker-compose restart clickhouse`

**Problem: Test data tables not found**

- Init scripts only run on first container creation
- If you need fresh data: `docker-compose down -v && docker-compose up -d`
- Check logs to see if init scripts ran: `docker-compose logs clickhouse | grep -i init`

**Problem: Cross-database test fails**

- Ensure both ClickHouse and Postgres containers are healthy
- Check Postgres port 5432: `lsof -i :5432`
- Verify both containers can communicate via Docker network

## Custom Connection Strings

To test against different database instances:

```bash
# Custom ClickHouse instance
export CLICKHOUSE_DSN="clickhouse://user:pass@host:9000/database"

go test -v
```

Or for CLI tests:

```bash
# Custom ClickHouse instance
export CLICKHOUSE_DSN="clickhouse://user:pass@host:9000/database"

./test-sq-cli.sh
```

## Continuous Integration

For CI environments, use the test scripts which handle container lifecycle:

```yaml
# Example GitHub Actions workflow
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      # Run integration tests
      - name: Run ClickHouse integration tests
        run: |
          cd drivers/clickhouse/testutils
          ./test-integration.sh

      # Build sq binary
      - name: Build sq
        run: go build -o drivers/clickhouse/testutils/sq .

      # Run CLI tests
      - name: Run CLI tests
        run: |
          cd drivers/clickhouse/testutils
          ./test-sq-cli.sh
```

## Performance Testing

For performance testing with larger datasets:

```bash
# Start ClickHouse
cd drivers/clickhouse/testutils
docker-compose up -d

# Generate larger test dataset
clickhouse-client --host localhost --port 9000 --user testuser --password testpass --database testdb --query "
CREATE TABLE perf_test (
    id UInt64,
    timestamp DateTime,
    value Float64,
    category String
) ENGINE = MergeTree()
ORDER BY (timestamp, id)
"

# Insert 1M rows
clickhouse-client --host localhost --port 9000 --user testuser --password testpass --database testdb --query "
INSERT INTO perf_test
SELECT
    number AS id,
    now() - INTERVAL number SECOND AS timestamp,
    rand() / 1000000000 AS value,
    concat('category_', toString(number % 10)) AS category
FROM numbers(1000000)
"

# Test query performance with sq
time sq sql 'SELECT category, count(*), avg(value) FROM perf_test GROUP BY category' @test_clickhouse
```

## Environment Variables

- `CLICKHOUSE_DSN` - Override default ClickHouse connection string
- `SQ_BINARY` - Path to sq binary for CLI tests
- `KEEP_CONTAINERS` - Set to `true` to keep containers running after tests

## Additional Resources

- [ClickHouse Documentation](https://clickhouse.com/docs)
- [clickhouse-go Driver](https://github.com/ClickHouse/clickhouse-go)
- [Docker Compose Reference](https://docs.docker.com/compose/)
- [sq Documentation](https://sq.io)
