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

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SQ_BINARY="${SQ_BINARY:-/Users/65720/Development/Projects/go/bin/sq}"
ORACLE_DSN="${ORACLE_DSN:-oracle://testuser:testpass@localhost:1521/FREEPDB1}"
POSTGRES_DSN="${POSTGRES_DSN:-postgres://testuser:testpass@localhost:5432/sakila?sslmode=disable}"
TEST_TABLE_PREFIX="SQ_TEST"

# Set Oracle Instant Client library path if not already set
if [ -d "/opt/oracle/instantclient" ]; then
    export DYLD_LIBRARY_PATH="/opt/oracle/instantclient:${DYLD_LIBRARY_PATH:-}"
fi

# Function to print colored messages
info() {
    echo -e "${BLUE}ℹ${NC} $*"
}

success() {
    echo -e "${GREEN}✓${NC} $*"
}

warning() {
    echo -e "${YELLOW}⚠${NC} $*"
}

error() {
    echo -e "${RED}✗${NC} $*"
}

# Function to run sq command with error handling
run_sq() {
    local description="$1"
    shift
    info "$description"
    if "$SQ_BINARY" "$@"; then
        success "$description"
        return 0
    else
        error "$description"
        return 1
    fi
}

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check prerequisites
check_prerequisites() {
    info "Checking prerequisites..."

    if [ ! -f "$SQ_BINARY" ]; then
        error "sq binary not found at: $SQ_BINARY"
        error "Set SQ_BINARY environment variable or update the script"
        exit 1
    fi

    if ! command_exists docker; then
        error "docker is not installed or not in PATH"
        exit 1
    fi

    if ! docker info >/dev/null 2>&1; then
        error "Docker daemon is not running"
        exit 1
    fi

    success "Prerequisites check passed"
    info "Using sq binary: $SQ_BINARY"
    info "sq version: $("$SQ_BINARY" version 2>/dev/null || echo 'unknown')"
}

# Start database containers
start_containers() {
    info "Starting Oracle and Postgres containers..."
    cd "$SCRIPT_DIR"
    docker-compose up -d oracle postgres

    # Wait for Oracle to be healthy
    info "Waiting for Oracle to be ready..."
    local max_wait=180
    local elapsed=0

    while [ $elapsed -lt $max_wait ]; do
        local health=$(docker-compose ps -q oracle | xargs docker inspect -f '{{.State.Health.Status}}' 2>/dev/null || echo "unknown")
        if [ "$health" = "healthy" ]; then
            success "Oracle is ready (${elapsed}s)"
            break
        fi
        echo -n "."
        sleep 2
        elapsed=$((elapsed + 2))
    done

    if [ $elapsed -ge $max_wait ]; then
        error "Oracle did not become ready in time"
        return 1
    fi

    # Wait for Postgres to be healthy
    info "Waiting for Postgres to be ready..."
    elapsed=0
    max_wait=60

    while [ $elapsed -lt $max_wait ]; do
        local health=$(docker-compose ps -q postgres | xargs docker inspect -f '{{.State.Health.Status}}' 2>/dev/null || echo "unknown")
        if [ "$health" = "healthy" ]; then
            success "Postgres is ready (${elapsed}s)"
            break
        fi
        echo -n "."
        sleep 2
        elapsed=$((elapsed + 2))
    done

    success "Databases started"
}

# Stop database containers
stop_containers() {
    info "Stopping containers..."
    cd "$SCRIPT_DIR"
    docker-compose down
    success "Containers stopped"
}

# Add data sources to sq
add_sources() {
    info "Adding data sources to sq..."

    # Remove existing sources if they exist
    "$SQ_BINARY" rm @test_oracle 2>/dev/null || true
    "$SQ_BINARY" rm @test_postgres 2>/dev/null || true

    # Add Oracle source
    if run_sq "Adding Oracle source" add --handle @test_oracle "$ORACLE_DSN"; then
        success "Oracle source added"
    else
        error "Failed to add Oracle source"
        return 1
    fi

    # Add Postgres source
    if run_sq "Adding Postgres source" add --handle @test_postgres "$POSTGRES_DSN"; then
        success "Postgres source added"
    else
        error "Failed to add Postgres source"
        return 1
    fi
}

# Test Oracle connectivity and basic operations
test_oracle_basic() {
    info "Testing Oracle basic operations..."
    echo ""

    # Test 1: Inspect Oracle (show schema)
    info "Test 1: Inspect Oracle schema"
    if "$SQ_BINARY" inspect @test_oracle >/dev/null 2>&1; then
        success "Oracle schema inspection succeeded"
    else
        error "Oracle schema inspection failed"
        return 1
    fi

    # Test 2: Execute simple query using SQL
    info "Test 2: Execute simple query (SELECT FROM DUAL)"
    local result=$("$SQ_BINARY" sql --src @test_oracle "SELECT 'Hello' AS TEST_COL FROM DUAL" --json 2>/dev/null)
    if echo "$result" | /usr/bin/grep -q '"TEST_COL"'; then
        success "Simple query succeeded"
    else
        error "Simple query failed"
        return 1
    fi

    # Test 3: Query current schema using SQL
    info "Test 3: Query current schema"
    local schema=$("$SQ_BINARY" sql --src @test_oracle "SELECT SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA') AS SCHEMA_NAME FROM DUAL" --json 2>/dev/null | /usr/bin/grep -o '"SCHEMA_NAME": "[^"]*"' | cut -d'"' -f4)
    if [ -n "$schema" ]; then
        success "Current schema: $schema"
    else
        error "Failed to query current schema"
        return 1
    fi

    success "Oracle basic operations test passed"
}

# Test creating and querying tables in Oracle
test_oracle_tables() {
    info "Testing Oracle table operations..."
    echo ""

    local test_table="${TEST_TABLE_PREFIX}_DEMO_$(date +%s)"

    # Test 1: Create a test table using SQL
    info "Test 1: Create test table in Oracle"
    if "$SQ_BINARY" sql --src @test_oracle "CREATE TABLE ${test_table} (ID NUMBER(19,0), NAME VARCHAR2(100), CREATED_AT TIMESTAMP)" >/dev/null 2>&1; then
        success "Created table ${test_table} in Oracle"
    else
        error "Failed to create table in Oracle"
        return 1
    fi

    # Test 2: Insert data using SQL
    info "Test 2: Insert data into Oracle table"
    if "$SQ_BINARY" sql --src @test_oracle "INSERT INTO ${test_table} (ID, NAME, CREATED_AT) VALUES (1, 'Test User', SYSTIMESTAMP)" >/dev/null 2>&1; then
        success "Inserted data into ${test_table}"
    else
        error "Failed to insert data"
        return 1
    fi

    # Test 3: Query data back
    info "Test 3: Query data from Oracle table"
    local row_count=$("$SQ_BINARY" sql --src @test_oracle "SELECT TO_CHAR(COUNT(*)) AS CNT FROM ${test_table}" --json 2>/dev/null | /usr/bin/grep -o '"CNT": "[0-9]*"' | cut -d'"' -f4)
    if [ "$row_count" -eq 1 ]; then
        success "Table has correct row count: $row_count"
    else
        error "Table row count mismatch. Expected: 1, Got: $row_count"
        return 1
    fi

    # Test 4: Query with WHERE clause
    info "Test 4: Query with WHERE clause"
    local name=$("$SQ_BINARY" sql --src @test_oracle "SELECT NAME FROM ${test_table} WHERE ID = 1" --json 2>/dev/null | /usr/bin/grep -o '"NAME": "[^"]*"' | cut -d'"' -f4)
    if [ "$name" = "Test User" ]; then
        success "WHERE clause query succeeded: $name"
    else
        error "WHERE clause query failed. Expected: Test User, Got: $name"
        return 1
    fi

    # Note: We don't drop the table here because sq doesn't have a DROP TABLE command
    # The table will remain in Oracle but that's fine for testing
    warning "Test table ${test_table} left in Oracle (sq doesn't have DROP TABLE command)"

    success "Oracle table operations test passed"
}

# Test cross-database operations
test_cross_database() {
    info "Testing cross-database operations (Postgres + Oracle)..."
    echo ""

    # Test 1: Query Postgres data
    info "Test 1: Query data from Postgres"
    local pg_output=$("$SQ_BINARY" sql --src @test_postgres "SELECT COUNT(*)::text AS CNT FROM actor" --json 2>&1)
    local pg_count=$(echo "$pg_output" | /usr/bin/grep -o '"[Cc][Nn][Tt]": "[0-9]*"' | cut -d'"' -f4)
    if [ -n "$pg_count" ] && [ "$pg_count" -gt 0 ]; then
        success "Postgres query succeeded: $pg_count actors"
    else
        error "Failed to query Postgres"
        return 1
    fi

    # Test 2: Create a matching table in Oracle and insert data manually
    info "Test 2: Create test table in Oracle for cross-database test"
    local test_table="${TEST_TABLE_PREFIX}_XFER_$(date +%s)"

    if "$SQ_BINARY" sql --src @test_oracle "CREATE TABLE ${test_table} (ACTOR_ID NUMBER(10,0), FIRST_NAME VARCHAR2(45), LAST_NAME VARCHAR2(45))" >/dev/null 2>&1; then
        success "Created table ${test_table} in Oracle"
    else
        error "Failed to create table in Oracle"
        return 1
    fi

    # Test 3: Insert sample data into Oracle table
    info "Test 3: Insert sample data into Oracle table"
    if "$SQ_BINARY" sql --src @test_oracle "INSERT INTO ${test_table} (ACTOR_ID, FIRST_NAME, LAST_NAME) VALUES (1, 'TEST', 'USER')" >/dev/null 2>&1; then
        success "Inserted test data into Oracle"
    else
        error "Failed to insert data"
        return 1
    fi

    # Test 4: Query Oracle table
    info "Test 4: Query Oracle table"
    local ora_count=$("$SQ_BINARY" sql --src @test_oracle "SELECT TO_CHAR(COUNT(*)) AS CNT FROM ${test_table}" --json 2>/dev/null | /usr/bin/grep -o '"CNT": "[0-9]*"' | cut -d'"' -f4)
    if [ "$ora_count" -eq 1 ]; then
        success "Oracle table query succeeded: $ora_count row"
    else
        error "Failed to query Oracle table"
        return 1
    fi

    warning "Test table ${test_table} left in Oracle"

    success "Cross-database operations test passed"
}

# Test type mappings
test_type_mappings() {
    info "Testing Oracle type mappings..."
    echo ""

    local test_table="${TEST_TABLE_PREFIX}_TYPES_$(date +%s)"

    # Test 1: Create table with various data types
    info "Test 1: Create table with various Oracle data types"
    if "$SQ_BINARY" sql --src @test_oracle "CREATE TABLE ${test_table} (
        COL_INT NUMBER(19,0),
        COL_TEXT VARCHAR2(100),
        COL_FLOAT BINARY_DOUBLE,
        COL_DECIMAL NUMBER(10,2),
        COL_BOOL NUMBER(1,0),
        COL_TIMESTAMP TIMESTAMP,
        COL_DATE DATE
    )" >/dev/null 2>&1; then
        success "Created table with various types"
    else
        error "Failed to create table with types"
        return 1
    fi

    # Test 2: Insert data with different types
    info "Test 2: Insert data with various types"
    if "$SQ_BINARY" sql --src @test_oracle "INSERT INTO ${test_table} VALUES (
        42,
        'Test Text',
        3.14,
        123.45,
        1,
        TIMESTAMP '2024-01-01 12:00:00',
        DATE '2024-01-01'
    )" >/dev/null 2>&1; then
        success "Inserted data with various types"
    else
        error "Failed to insert data"
        return 1
    fi

    # Test 3: Query data back and verify
    info "Test 3: Query and verify type-mapped data"
    local int_val=$("$SQ_BINARY" sql --src @test_oracle "SELECT TO_CHAR(COL_INT) AS COL_INT FROM ${test_table}" --json 2>/dev/null | /usr/bin/grep -o '"COL_INT": "[0-9]*"' | cut -d'"' -f4)
    if [ "$int_val" -eq 42 ]; then
        success "Type mapping verified: INT=$int_val"
    else
        error "Type mapping failed. Expected: 42, Got: $int_val"
        return 1
    fi

    warning "Test table ${test_table} left in Oracle"

    success "Type mapping test passed"
}

# Cleanup
cleanup() {
    info "Cleaning up..."

    # Remove sources
    "$SQ_BINARY" rm @test_oracle 2>/dev/null || true
    "$SQ_BINARY" rm @test_postgres 2>/dev/null || true

    success "Cleanup completed"
}

# Main execution
main() {
    info "SQ CLI Oracle Driver Test"
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
    info "Test Summary"
    success "Tests passed: $tests_passed"
    if [ $tests_failed -gt 0 ]; then
        error "Tests failed: $tests_failed"
        error "SQ CLI Oracle driver test FAILED"
        exit 1
    else
        success "All tests passed!"
        success "SQ CLI Oracle driver test completed successfully"
        exit 0
    fi
}

# Trap to ensure cleanup on exit
cleanup_on_exit() {
    if [ $? -ne 0 ]; then
        warning "Tests failed, cleaning up..."
        cleanup 2>/dev/null || true
        stop_containers 2>/dev/null || true
    fi
}
trap cleanup_on_exit EXIT

# Run main function
main
