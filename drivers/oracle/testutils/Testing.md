# Testing Oracle Driver

This document describes how to run tests for the Oracle driver.

## Prerequisites for Integration Tests

> **⚠️ Apple Silicon Mac Users (M1/M2/M3/M4):**
>
> You **must** manually download Oracle Instant Client for ARM64.
> The Homebrew version is Intel-only and **will not work**.
>
> Download from: <https://www.oracle.com/database/technologies/instant-client/macos-arm64-downloads.html>
>
> See [Step 2: Install Oracle Instant Client](#manual-setup-step-2---install-oracle-instant-client) for detailed instructions.

## TL;DR

**Run only unit tests (no database required):**

```bash
cd drivers/oracle
go test -v -short
```

> Note: The test scripts and docker-compose.yml are located in `drivers/oracle/testutils/`.

**Run integration tests (requires Oracle + Instant Client):**

```bash
cd drivers/oracle/testutils
./test-integration.sh
```

**Run all tests including cross-database (requires Oracle + Postgres):**

```bash
cd drivers/oracle/testutils
./test-integration.sh --with-pg
```

## Test Organization

Tests are organized into two categories:

### Unit Tests (No Database Required)

Unit tests run with `go test -short` and do not require any database connection. They test:

- Placeholder generation (`:1, :2, :3` format)
- Type mapping (Kind → Oracle types)
- Error code detection

```bash
cd drivers/oracle
go test -v -short
```

Expected output: **4 tests passing**, 6 tests skipped

> Note: Test scripts and docker-compose.yml are located in `testutils/` subdirectory.

You can also run specific unit tests:

```bash
go test -v -short -run "Test(Placeholders|HasErrCode|IsErrTableNotExist|DbTypeNameFromKind)"
```

### Integration Tests (Database Required)

Integration tests require a running Oracle database (and optionally Postgres for cross-database tests). They are automatically skipped when using the `-short` flag.

**Manual approach:**

```bash
# Start databases (from testutils directory)
cd drivers/oracle/testutils
docker-compose up -d

# Run tests from driver directory
cd drivers/oracle
go test -v

# Stop databases
cd drivers/oracle/testutils
docker-compose down
```

**Automated approach (recommended):**

```bash
# Use the test-integration.sh script
./test-integration.sh
```

The script handles:

- Starting docker-compose services
- Waiting for databases to be healthy
- Running integration tests
- Cleaning up containers

Expected output: **10 tests passing** (4 unit + 6 integration)

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

**Run all Oracle integration tests:**

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

**Run cross-database test only:**

```bash
./test-integration.sh --with-pg --pattern TestSakilaCrossDatabase
```

### What the Script Does

1. ✓ Checks prerequisites (docker, go, docker daemon)
2. ✓ Starts docker-compose services (oracle, and optionally postgres)
3. ✓ Waits for services to become healthy (with timeout)
4. ✓ Runs integration tests with proper timeout
5. ✓ Shows colored output for easy reading
6. ✓ Cleans up containers (unless `--keep` specified)
7. ✓ Exits with proper status code

### Troubleshooting the Script

**Script fails with "Docker daemon is not running":**

- Start Docker Desktop/daemon and try again

**Script times out waiting for Oracle:**

- Oracle takes 1-2 minutes to initialize on first run
- Check logs: `docker-compose logs oracle`
- Try increasing timeout by waiting longer

**Tests fail but containers stop too quickly:**

- Use `--keep` flag to keep containers running
- Inspect logs: `docker-compose logs oracle postgres`
- Connect manually to debug

## Integration Test Requirements

Integration tests require both:

1. **Oracle database** (provided via Docker Compose)
2. **Oracle Instant Client** (must be installed on your system)

If you're using the `test-integration.sh` script, it handles starting/stopping Docker automatically. The manual steps below are only needed if you want to manage containers yourself.

### Manual Setup: Step 1 - Start Oracle Database

```bash
cd drivers/oracle/testutils

# Start Oracle container
docker-compose up -d

# Check status (wait until 'healthy')
docker-compose ps

# View logs if needed
docker-compose logs -f oracle
```

Oracle takes **1-2 minutes** to initialize. Wait until the health check shows `healthy`.

Container configuration:

- **Image**: `gvenzl/oracle-free:23-slim`
- **User**: `testuser`
- **Password**: `testpass`
- **Database**: `FREEPDB1`
- **Port**: `1521` (mapped to localhost)
- **Connection**: `oracle://testuser:testpass@localhost:1521/FREEPDB1`

### Manual Setup: Step 2 - Install Oracle Instant Client

The `godror` driver requires Oracle Instant Client libraries on your system.

> **⚠️ IMPORTANT: You must install the correct architecture!**
>
> - **Apple Silicon Macs (M1/M2/M3/M4)** → Must use **ARM64** version (manual download required)
> - **Intel Macs** → Can use Homebrew or manual download (x86_64)
>
> Installing the wrong architecture causes this error:
>
> ```bash
> DPI-1047: Cannot locate a 64-bit Oracle Client library:
> "mach-o file, but is an incompatible architecture (have 'x86_64', need 'arm64')"
> ```

---

#### macOS Apple Silicon (M1/M2/M3/M4) - MANUAL INSTALL REQUIRED

The Homebrew tap only has the old Intel (x86_64) version. **You must manually download the ARM64 version:**

1. **Download** the ARM64 Basic Package from Oracle:

   <https://www.oracle.com/database/technologies/instant-client/macos-arm64-downloads.html>

   Look for: `instantclient-basic-macos.arm64-X.X.X.X.X.dmg` (or `.zip`)

2. **Install** the package:

```bash
# Create installation directory
sudo mkdir -p /opt/oracle

# If you downloaded the DMG:
# - Double-click to mount
# - Copy the instantclient_XX_X folder to /opt/oracle
# - Rename to 'instantclient' for simplicity

# If you downloaded the ZIP:
cd ~/Downloads
unzip instantclient-basic-macos.arm64-*.zip
sudo mv instantclient_* /opt/oracle/instantclient
```

3. **Set the library path** (add to your `~/.zshrc`):

```bash
echo 'export DYLD_LIBRARY_PATH=/opt/oracle/instantclient:$DYLD_LIBRARY_PATH' >> ~/.zshrc
source ~/.zshrc
```

4. **Verify** the installation:

```bash
ls /opt/oracle/instantclient/libclntsh.dylib
# Should show the file exists
```

---

#### macOS Intel (x86_64) - Homebrew Option Available

**Option A: Using Homebrew (easiest for Intel Macs)**

```bash
brew tap InstantClientTap/instantclient
brew install instantclient-basic
```

**Option B: Manual Installation**

1. Download from: <https://www.oracle.com/database/technologies/instant-client/macos-intel-x86-downloads.html>
2. Follow the same steps as Apple Silicon above

---

#### Linux

See: <https://oracle.github.io/odpi/doc/installation.html#linux>

---

#### Verifying Your Installation

After installation, verify the library can be found:

```bash
# Check architecture (should match your Mac)
file /opt/oracle/instantclient/libclntsh.dylib
# Apple Silicon: "Mach-O 64-bit dynamically linked shared library arm64"
# Intel: "Mach-O 64-bit dynamically linked shared library x86_64"

# Test with a simple Go program
cd drivers/oracle
go test -v -run TestSmoke
```

### Manual Setup: Step 3 - Run Integration Tests

```bash
cd drivers/oracle

# Make sure Oracle is running and healthy
docker-compose ps

# Run all tests
go test -v

# Or run with longer timeout
go test -v -timeout 5m

# Run specific test
go test -v -run TestSakilaCrossDatabase
```

Expected output: **10 tests passing** (4 unit + 6 integration)

Integration tests cover:

- Basic connectivity (TestSmoke)
- Schema detection (TestCurrentSchema)
- Table creation and deletion (TestCreateAndDropTable)
- Type mappings with real data (TestTypeMappings)
- Table listing (TestListTables)
- Cross-database replication from Postgres to Oracle (TestSakilaCrossDatabase)

### Test Behavior Without Oracle Client

If Oracle Instant Client is not installed, integration tests will **gracefully skip** with a message like:

```bash
--- SKIP: TestSmoke (0.00s)
    oracle_test.go:72: Oracle not available: DPI-1047: Cannot locate a 64-bit Oracle Client library
```

This is **expected behavior**. Unit tests will still pass.

## Cross-Database Integration Test

The `TestSakilaCrossDatabase` test demonstrates real-world usage by:

1. Loading Sakila dataset into Postgres
2. Reading data from Postgres
3. Writing data to Oracle
4. Verifying row counts and data integrity match

This test requires both Postgres and Oracle to be running. It can be run with:

```bash
# Start both databases (from testutils directory)
cd drivers/oracle/testutils
docker-compose up -d

# Run only the cross-database test (from driver directory)
cd drivers/oracle
go test -v -run TestSakilaCrossDatabase -timeout 10m
```

## Docker Management Commands

```bash
# From the testutils directory
cd drivers/oracle/testutils

# Start Oracle (and optionally Postgres)
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f oracle
docker-compose logs -f postgres

# Stop databases (keeps data)
docker-compose down

# Stop and remove data volumes (clean slate)
docker-compose down -v

# Restart databases
docker-compose restart
```

## Troubleshooting

**Problem: "incompatible architecture (have 'x86_64', need 'arm64')"**

This means you installed the Intel (x86_64) version on an Apple Silicon Mac.

- Uninstall the wrong version: `brew uninstall instantclient-basic`
- Download the ARM64 version manually from Oracle (see Step 2 above)
- The Homebrew tap does NOT have ARM64 builds

**Problem: Tests skip with "Oracle not available" or "DPI-1047"**

- Install Oracle Instant Client (see Step 2 above)
- Make sure you have the correct architecture for your Mac
- Verify `DYLD_LIBRARY_PATH` is set: `echo $DYLD_LIBRARY_PATH`

**Problem: Container not healthy**

- Wait longer (Oracle takes 1-2 minutes to start)
- Check logs: `docker-compose logs oracle`

**Problem: Connection refused**

- Ensure Docker is running: `docker ps`
- Ensure container is healthy: `docker-compose ps`
- Check port 1521 is available: `lsof -i :1521`

**Problem: Authentication failed**

- Wait for Oracle to fully initialize (check `docker-compose logs`)
- Try restarting: `docker-compose restart oracle`

**Problem: Cross-database test fails**

- Ensure both Postgres and Oracle containers are healthy
- Check Postgres port 5432: `lsof -i :5432`
- Verify Sakila data loaded: See docker-compose logs

## Custom Connection Strings

To test against different database instances:

```bash
# Custom Oracle instance
export SQ_TEST_ORACLE_DSN="oracle://user:pass@host:1521/service"

# Custom Postgres instance (for cross-database test)
export SQ_TEST_POSTGRES_DSN="postgres://user:pass@host:5432/dbname"

go test -v
```

## Environment Variables

- `SQ_TEST_ORACLE_DSN` - Override default Oracle connection string
- `SQ_TEST_POSTGRES_DSN` - Override default Postgres connection string (for cross-database tests)
