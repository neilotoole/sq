# SQ CLI Oracle Driver Testing

This document describes the `test-sq-cli.sh` script, which tests the Oracle driver end-to-end using the `sq` command-line interface.

## Overview

The `test-sq-cli.sh` script validates the Oracle driver by running real-world operations through the `sq` CLI tool, rather than through Go unit tests. This provides an integration test from the user's perspective.

## What It Tests

1. **Oracle Connectivity**
   - Adding Oracle as a data source
   - Inspecting Oracle schema
   - Executing simple queries (SELECT FROM DUAL)
   - Querying system information

2. **Table Operations**
   - Creating tables in Oracle from Postgres data
   - Querying tables with column selection
   - Filtering with WHERE clauses
   - Verifying row counts

3. **Cross-Database Data Movement**
   - Copying data from Postgres to Oracle
   - Verifying data integrity across databases
   - Testing cross-database joins
   - Handling 10+ row transfers

4. **Type Mappings**
   - Transferring various data types (INT, VARCHAR, TIMESTAMP, DATE)
   - Querying back type-mapped data
   - Verifying type conversion accuracy

## Prerequisites

- Docker and Docker Compose installed and running
- `sq` binary built and available
- Oracle Instant Client installed (for connecting to Oracle)

## Usage

### Basic Usage

```bash
cd drivers/oracle/testutils
./test-sq-cli.sh
```

### Custom SQ Binary Location

```bash
SQ_BINARY=/path/to/custom/sq ./test-sq-cli.sh
```

### Custom Database Connections

```bash
# Custom Oracle connection
ORACLE_DSN="oracle://user:pass@host:1521/service" ./test-sq-cli.sh

# Custom Postgres connection
POSTGRES_DSN="postgres://user:pass@host:5432/dbname" ./test-sq-cli.sh

# Both custom
ORACLE_DSN="..." POSTGRES_DSN="..." ./test-sq-cli.sh
```

## Configuration

The script supports the following environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `SQ_BINARY` | `/Users/65720/Development/Projects/go/bin/sq` | Path to sq binary |
| `ORACLE_DSN` | `oracle://testuser:testpass@localhost:1521/FREEPDB1` | Oracle connection string |
| `POSTGRES_DSN` | `postgres://testuser:testpass@localhost:5432/sakila?sslmode=disable` | Postgres connection string |

## Test Sequence

The script performs tests in this order:

1. **Prerequisites Check**
   - Verifies `sq` binary exists
   - Verifies Docker is running
   - Shows `sq` version

2. **Start Databases**
   - Starts Oracle and Postgres containers via docker-compose
   - Waits for both databases to be healthy

3. **Add Data Sources**
   - Adds `@test_oracle` source
   - Adds `@test_postgres` source

4. **Oracle Basic Operations Test**
   - List data sources
   - Inspect Oracle schema
   - Execute simple SELECT query
   - Query current schema name

5. **Oracle Table Operations Test**
   - Create table from Postgres data (5 rows)
   - Verify row count
   - Query specific columns
   - Query with WHERE clause

6. **Cross-Database Data Movement Test**
   - Copy actor table (10 rows) from Postgres to Oracle
   - Verify row counts match
   - Verify data integrity
   - Test cross-database join (optional)

7. **Type Mapping Test**
   - Copy data with various types (INT, TIMESTAMP, DATE, VARCHAR)
   - Verify type-mapped data can be queried

8. **Cleanup**
   - Remove data sources from sq config
   - Stop Docker containers

## Output

The script provides colorful, detailed output showing:

- ℹ Info messages (blue) - What's currently happening
- ✓ Success messages (green) - What succeeded
- ✗ Error messages (red) - What failed
- ⚠ Warning messages (yellow) - Non-critical issues

### Sample Output

```
ℹ SQ CLI Oracle Driver Test

ℹ Checking prerequisites...
✓ Prerequisites check passed
ℹ Using sq binary: /Users/65720/Development/Projects/go/bin/sq
ℹ sq version: v0.48.5

ℹ Starting Oracle and Postgres containers...
ℹ Waiting for Oracle to be ready...
✓ Oracle is ready (10s)
✓ Databases started

ℹ Adding data sources to sq...
✓ Oracle source added
✓ Postgres source added

ℹ Testing Oracle basic operations...
✓ Oracle source is listed
✓ Oracle schema inspection succeeded
✓ Simple query succeeded
✓ Current schema: TESTUSER
✓ Oracle basic operations test passed

...

✓ All tests passed!
✓ SQ CLI Oracle driver test completed successfully
```

## Exit Codes

- `0` - All tests passed
- `1` - One or more tests failed

## Notes

- **Test Tables**: The script creates tables in Oracle with prefix `SQ_TEST_`. These tables are left in Oracle after the test completes because `sq` doesn't have a DROP TABLE command. This is intentional and allows manual inspection of test results.

- **Docker Containers**: Containers are automatically stopped after tests complete, regardless of test success or failure.

- **Source Cleanup**: Data sources are removed from `sq` configuration during cleanup.

## Troubleshooting

### sq binary not found

```
✗ sq binary not found at: /path/to/sq
```

**Solution**: Set the `SQ_BINARY` environment variable to the correct path:

```bash
SQ_BINARY=/correct/path/to/sq ./test-sq-cli.sh
```

### Docker not running

```
✗ Docker daemon is not running
```

**Solution**: Start Docker Desktop or Docker daemon.

### Oracle/Postgres connection failed

```
✗ Failed to add Oracle source
```

**Solution**:
1. Ensure containers are running: `docker-compose ps`
2. Check container logs: `docker-compose logs oracle`
3. Verify connection strings are correct
4. Ensure Oracle Instant Client is installed

### Tests fail intermittently

If tests fail due to timing issues, the containers may not be fully ready. The script waits up to 180s for Oracle and 60s for Postgres. If you have a slower system, you may need to adjust these timeouts in the script.

## Comparison with Integration Tests

| Aspect | Integration Tests | CLI Test Script |
|--------|------------------|-----------------|
| **Language** | Go test code | Bash script |
| **Tests via** | Go driver API | sq CLI commands |
| **Focus** | Driver internals | User experience |
| **Speed** | Fast (parallel) | Slower (sequential CLI calls) |
| **Coverage** | Comprehensive | High-level workflows |
| **Debugging** | Go debugger | Shell output |

Both test approaches are valuable:
- **Integration tests** validate the driver implementation at the code level
- **CLI test script** validates the end-to-end user experience

## Future Enhancements

Potential improvements to the script:

- [ ] Add support for DROP TABLE when available in sq
- [ ] Test more complex queries (aggregations, subqueries)
- [ ] Test error handling scenarios
- [ ] Add performance benchmarking
- [ ] Test with larger datasets
- [ ] Add CSV/JSON export/import testing
- [ ] Test schema migration workflows
