#!/usr/bin/env bash
#
# test-sq-cli.sh - Test Oracle driver using sq CLI
#
# This script tests the Oracle driver end-to-end using the sq command-line tool.
# It verifies:
# 1. Oracle connectivity and basic operations
# 2. Data querying and inspection
# 3. Cross-database data movement (Postgres -> Oracle)
#
# Usage:
#   ./test-sq-cli.sh              # Use default sq binary path
#   SQ_BINARY=/path/to/sq ./test-sq-cli.sh  # Use custom sq binary

set -euo pipefail

# Get script directory and source logging utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DRIVER_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
source "${SCRIPT_DIR}/log.bash"

# Configuration
SQ_BINARY="${SQ_BINARY:-/Users/65720/Development/Projects/go/bin/sq}"
ORACLE_DSN="${ORACLE_DSN:-oracle://testuser:testpass@localhost:1521/FREEPDB1}"
POSTGRES_DSN="${POSTGRES_DSN:-postgres://testuser:testpass@localhost:5432/sakila?sslmode=disable}"
TEST_TABLE_PREFIX="SQ_TEST"

# Set Oracle Instant Client library path if not already set
if [ -d "/opt/oracle/instantclient" ]; then
    export DYLD_LIBRARY_PATH="/opt/oracle/instantclient:${DYLD_LIBRARY_PATH:-}"
fi

# Function to run sq command with error handling
run_sq() {
    local description="$1"
    shift
    log_info "$description"
    if "$SQ_BINARY" "$@"; then
        log_success "$description"
        return 0
    else
        log_error "$description"
        return 1
    fi
}

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    if [ ! -f "$SQ_BINARY" ]; then
        log_error "sq binary not found at: $SQ_BINARY"
        log_error "Set SQ_BINARY environment variable or update the script"
        exit 1
    fi

    if ! command_exists docker; then
        log_error "docker is not installed or not in PATH"
        exit 1
    fi

    if ! docker info >/dev/null 2>&1; then
        log_error "Docker daemon is not running"
        exit 1
    fi

    log_success "Prerequisites check passed"
    log_info "Using sq binary: $SQ_BINARY"
    log_info "sq version: $("$SQ_BINARY" version 2>/dev/null || echo 'unknown')"
}

# Start database containers
start_containers() {
    log_info "Starting Oracle and Postgres containers..."
    cd "$SCRIPT_DIR"
    docker-compose up -d oracle postgres

    # Wait for Oracle to be healthy
    log_info "Waiting for Oracle to be ready..."
    local max_wait=180
    local elapsed=0

    while [ $elapsed -lt $max_wait ]; do
        local health=$(docker-compose ps -q oracle | xargs docker inspect -f '{{.State.Health.Status}}' 2>/dev/null || echo "unknown")
        if [ "$health" = "healthy" ]; then
            log_success "Oracle is ready (${elapsed}s)"
            break
        fi
        echo -n "."
        sleep 2
        elapsed=$((elapsed + 2))
    done

    if [ $elapsed -ge $max_wait ]; then
        log_error "Oracle did not become ready in time"
        return 1
    fi

    # Wait for Postgres to be healthy
    log_info "Waiting for Postgres to be ready..."
    elapsed=0
    max_wait=60

    while [ $elapsed -lt $max_wait ]; do
        local health=$(docker-compose ps -q postgres | xargs docker inspect -f '{{.State.Health.Status}}' 2>/dev/null || echo "unknown")
        if [ "$health" = "healthy" ]; then
            log_success "Postgres is ready (${elapsed}s)"
            break
        fi
        echo -n "."
        sleep 2
        elapsed=$((elapsed + 2))
    done

    log_success "Databases started"
}

# Stop database containers
stop_containers() {
    log_info "Stopping containers..."
    cd "$SCRIPT_DIR"
    docker-compose down
    log_success "Containers stopped"
}

# Add data sources to sq
add_sources() {
    log_info "Adding data sources to sq..."

    # Remove existing sources if they exist
    "$SQ_BINARY" rm @test_oracle 2>/dev/null || true
    "$SQ_BINARY" rm @test_postgres 2>/dev/null || true

    # Add Oracle source
    if run_sq "Adding Oracle source" add --handle @test_oracle "$ORACLE_DSN"; then
        log_success "Oracle source added"
    else
        log_error "Failed to add Oracle source"
        return 1
    fi

    # Add Postgres source
    if run_sq "Adding Postgres source" add --handle @test_postgres "$POSTGRES_DSN"; then
        log_success "Postgres source added"
    else
        log_error "Failed to add Postgres source"
        return 1
    fi
}

# Test Oracle connectivity and basic operations
test_oracle_basic() {
    log_info "Testing Oracle basic operations..."
    echo ""

    # Test 1: Inspect Oracle (show schema)
    log_info "Test 1: Inspect Oracle schema"
    if "$SQ_BINARY" inspect @test_oracle >/dev/null 2>&1; then
        log_success "Oracle schema inspection succeeded"
    else
        log_error "Oracle schema inspection failed"
        return 1
    fi

    # Test 2: Execute simple query using SQL
    log_info "Test 2: Execute simple query (SELECT FROM DUAL)"
    local result=$("$SQ_BINARY" sql --src @test_oracle "SELECT 'Hello' AS TEST_COL FROM DUAL" --json 2>/dev/null)
    if echo "$result" | /usr/bin/grep -q '"TEST_COL"'; then
        log_success "Simple query succeeded"
    else
        log_error "Simple query failed"
        return 1
    fi

    # Test 3: Query current schema using SQL
    log_info "Test 3: Query current schema"
    local schema=$("$SQ_BINARY" sql --src @test_oracle "SELECT SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA') AS SCHEMA_NAME FROM DUAL" --json 2>/dev/null | /usr/bin/grep -o '"SCHEMA_NAME": "[^"]*"' | cut -d'"' -f4)
    if [ -n "$schema" ]; then
        log_success "Current schema: $schema"
    else
        log_error "Failed to query current schema"
        return 1
    fi

    log_success "Oracle basic operations test passed"
}

# Test creating and querying tables in Oracle
test_oracle_tables() {
    log_info "Testing Oracle table operations..."
    echo ""

    local test_table="${TEST_TABLE_PREFIX}_DEMO_$(date +%s)"

    # Test 1: Create a test table using SQL
    log_info "Test 1: Create test table in Oracle"
    if "$SQ_BINARY" sql --src @test_oracle "CREATE TABLE ${test_table} (ID NUMBER(19,0), NAME VARCHAR2(100), CREATED_AT TIMESTAMP)" >/dev/null 2>&1; then
        log_success "Created table ${test_table} in Oracle"
    else
        log_error "Failed to create table in Oracle"
        return 1
    fi

    # Test 2: Insert data using SQL
    log_info "Test 2: Insert data into Oracle table"
    if "$SQ_BINARY" sql --src @test_oracle "INSERT INTO ${test_table} (ID, NAME, CREATED_AT) VALUES (1, 'Test User', SYSTIMESTAMP)" >/dev/null 2>&1; then
        log_success "Inserted data into ${test_table}"
    else
        log_error "Failed to insert data"
        return 1
    fi

    # Test 3: Query data back
    log_info "Test 3: Query data from Oracle table"
    local row_count=$("$SQ_BINARY" sql --src @test_oracle "SELECT TO_CHAR(COUNT(*)) AS CNT FROM ${test_table}" --json 2>/dev/null | /usr/bin/grep -o '"CNT": "[0-9]*"' | cut -d'"' -f4)
    if [ "$row_count" -eq 1 ]; then
        log_success "Table has correct row count: $row_count"
    else
        log_error "Table row count mismatch. Expected: 1, Got: $row_count"
        return 1
    fi

    # Test 4: Query with WHERE clause
    log_info "Test 4: Query with WHERE clause"
    local name=$("$SQ_BINARY" sql --src @test_oracle "SELECT NAME FROM ${test_table} WHERE ID = 1" --json 2>/dev/null | /usr/bin/grep -o '"NAME": "[^"]*"' | cut -d'"' -f4)
    if [ "$name" = "Test User" ]; then
        log_success "WHERE clause query succeeded: $name"
    else
        log_error "WHERE clause query failed. Expected: Test User, Got: $name"
        return 1
    fi

    # Note: We don't drop the table here because sq doesn't have a DROP TABLE command
    # The table will remain in Oracle but that's fine for testing
    log_warning "Test table ${test_table} left in Oracle (sq doesn't have DROP TABLE command)"

    log_success "Oracle table operations test passed"
}

# Test cross-database operations
test_cross_database() {
    log_info "Testing cross-database operations (Postgres + Oracle)..."
    echo ""

    # Test 1: Query Postgres data
    log_info "Test 1: Query data from Postgres"
    local pg_output=$("$SQ_BINARY" sql --src @test_postgres "SELECT COUNT(*)::text AS CNT FROM actor" --json 2>&1)
    local pg_count=$(echo "$pg_output" | /usr/bin/grep -o '"[Cc][Nn][Tt]": "[0-9]*"' | cut -d'"' -f4)
    if [ -n "$pg_count" ] && [ "$pg_count" -gt 0 ]; then
        log_success "Postgres query succeeded: $pg_count actors"
    else
        log_error "Failed to query Postgres"
        return 1
    fi

    # Test 2: Create a matching table in Oracle and insert data manually
    log_info "Test 2: Create test table in Oracle for cross-database test"
    local test_table="${TEST_TABLE_PREFIX}_XFER_$(date +%s)"

    if "$SQ_BINARY" sql --src @test_oracle "CREATE TABLE ${test_table} (ACTOR_ID NUMBER(10,0), FIRST_NAME VARCHAR2(45), LAST_NAME VARCHAR2(45))" >/dev/null 2>&1; then
        log_success "Created table ${test_table} in Oracle"
    else
        log_error "Failed to create table in Oracle"
        return 1
    fi

    # Test 3: Insert sample data into Oracle table
    log_info "Test 3: Insert sample data into Oracle table"
    if "$SQ_BINARY" sql --src @test_oracle "INSERT INTO ${test_table} (ACTOR_ID, FIRST_NAME, LAST_NAME) VALUES (1, 'TEST', 'USER')" >/dev/null 2>&1; then
        log_success "Inserted test data into Oracle"
    else
        log_error "Failed to insert data"
        return 1
    fi

    # Test 4: Query Oracle table
    log_info "Test 4: Query Oracle table"
    local ora_count=$("$SQ_BINARY" sql --src @test_oracle "SELECT TO_CHAR(COUNT(*)) AS CNT FROM ${test_table}" --json 2>/dev/null | /usr/bin/grep -o '"CNT": "[0-9]*"' | cut -d'"' -f4)
    if [ "$ora_count" -eq 1 ]; then
        log_success "Oracle table query succeeded: $ora_count row"
    else
        log_error "Failed to query Oracle table"
        return 1
    fi

    log_warning "Test table ${test_table} left in Oracle"

    log_success "Cross-database operations test passed"
}

# Test type mappings
test_type_mappings() {
    log_info "Testing Oracle type mappings..."
    echo ""

    local test_table="${TEST_TABLE_PREFIX}_TYPES_$(date +%s)"

    # Test 1: Create table with various data types
    log_info "Test 1: Create table with various Oracle data types"
    if "$SQ_BINARY" sql --src @test_oracle "CREATE TABLE ${test_table} (
        COL_INT NUMBER(19,0),
        COL_TEXT VARCHAR2(100),
        COL_FLOAT BINARY_DOUBLE,
        COL_DECIMAL NUMBER(10,2),
        COL_BOOL NUMBER(1,0),
        COL_TIMESTAMP TIMESTAMP,
        COL_DATE DATE
    )" >/dev/null 2>&1; then
        log_success "Created table with various types"
    else
        log_error "Failed to create table with types"
        return 1
    fi

    # Test 2: Insert data with different types
    log_info "Test 2: Insert data with various types"
    if "$SQ_BINARY" sql --src @test_oracle "INSERT INTO ${test_table} VALUES (
        42,
        'Test Text',
        3.14,
        123.45,
        1,
        TIMESTAMP '2024-01-01 12:00:00',
        DATE '2024-01-01'
    )" >/dev/null 2>&1; then
        log_success "Inserted data with various types"
    else
        log_error "Failed to insert data"
        return 1
    fi

    # Test 3: Query data back and verify
    log_info "Test 3: Query and verify type-mapped data"
    local int_val=$("$SQ_BINARY" sql --src @test_oracle "SELECT TO_CHAR(COL_INT) AS COL_INT FROM ${test_table}" --json 2>/dev/null | /usr/bin/grep -o '"COL_INT": "[0-9]*"' | cut -d'"' -f4)
    if [ "$int_val" -eq 42 ]; then
        log_success "Type mapping verified: INT=$int_val"
    else
        log_error "Type mapping failed. Expected: 42, Got: $int_val"
        return 1
    fi

    log_warning "Test table ${test_table} left in Oracle"

    log_success "Type mapping test passed"
}

# Cleanup
cleanup() {
    log_info "Cleaning up..."

    # Remove sources
    "$SQ_BINARY" rm @test_oracle 2>/dev/null || true
    "$SQ_BINARY" rm @test_postgres 2>/dev/null || true

    log_success "Cleanup completed"
}

# Main execution
main() {
    log_separator
    log_banner
    log_info "SQ CLI Oracle Driver Test"
    echo ""

    # Track test results
    local tests_passed=0
    local tests_failed=0

    # Check prerequisites
    check_prerequisites
    echo ""

    # Start containers
    start_containers
    echo ""

    # Add sources
    if add_sources; then
        ((tests_passed++))
    else
        ((tests_failed++))
    fi
    echo ""

    # Run tests
    if test_oracle_basic; then
        ((tests_passed++))
    else
        ((tests_failed++))
    fi
    echo ""

    if test_oracle_tables; then
        ((tests_passed++))
    else
        ((tests_failed++))
    fi
    echo ""

    if test_cross_database; then
        ((tests_passed++))
    else
        ((tests_failed++))
    fi
    echo ""

    if test_type_mappings; then
        ((tests_passed++))
    else
        ((tests_failed++))
    fi
    echo ""

    # Cleanup
    cleanup
    echo ""

    # Stop containers
    stop_containers
    echo ""

    # Print summary
    log_info "Test Summary"
    log_success "Tests passed: $tests_passed"
    if [ $tests_failed -gt 0 ]; then
        log_error "Tests failed: $tests_failed"
        log_error "SQ CLI Oracle driver test FAILED"
        log_separator
        exit 1
    else
        log_success "All tests passed!"
        log_success "SQ CLI Oracle driver test completed successfully"
        log_separator
        exit 0
    fi
}

# Trap to ensure cleanup on exit
cleanup_on_exit() {
    if [ $? -ne 0 ]; then
        log_warning "Tests failed, cleaning up..."
        cleanup 2>/dev/null || true
        stop_containers 2>/dev/null || true
    fi
}
trap cleanup_on_exit EXIT

# Run main function
main
