#!/usr/bin/env bash
#
# test-sq-cli.sh - Test ClickHouse driver using sq CLI
#
# This script tests the ClickHouse driver end-to-end using the sq command-line tool.
# It verifies:
# 1. ClickHouse connectivity and basic operations
# 2. Data querying and inspection
# 3. Table operations (create, insert, query)
#
# Usage:
#   ./test-sq-cli.sh              # Use default sq binary path
#   SQ_BINARY=/path/to/sq ./test-sq-cli.sh  # Use custom sq binary

set -euo pipefail

# Get script directory and source common utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.bash"

# Configuration
# Default to $GOPATH/bin/sq, fall back to finding sq in PATH
if [ -z "${SQ_BINARY:-}" ]; then
    if [ -n "${GOPATH:-}" ] && [ -x "${GOPATH}/bin/sq" ]; then
        SQ_BINARY="${GOPATH}/bin/sq"
    elif command -v sq >/dev/null 2>&1; then
        SQ_BINARY="$(command -v sq)"
    else
        SQ_BINARY="sq"  # Will fail later with helpful error message
    fi
fi
CLICKHOUSE_DSN="${CLICKHOUSE_DSN:-clickhouse://testuser:testpass@localhost:19000/testdb}"
TEST_TABLE_PREFIX="sq_test"

# ==============================================================================
# SQ-Specific Functions
# ==============================================================================

# Run sq command with error handling
run_sq() {
    local description="$1"
    shift
    log_dim "$description"
    if "$SQ_BINARY" "$@"; then
        return 0
    else
        log_error "$description failed"
        return 1
    fi
}

# Check prerequisites (sq binary, Docker, etc.)
check_prerequisites() {
    log "Checking Prerequisites"

    # Check for sq binary
    if [ ! -f "$SQ_BINARY" ] && ! command -v "$SQ_BINARY" >/dev/null 2>&1; then
        log_error "sq binary not found at: $SQ_BINARY"
        log_error "Options to fix:"
        log_error "  1. Install sq: go install github.com/neilotoole/sq"
        log_error "  2. Set GOPATH if sq is in \$GOPATH/bin"
        log_error "  3. Set SQ_BINARY=/path/to/sq environment variable"
        exit 1
    fi

    # Check Docker prerequisites (from common.bash)
    check_docker_prerequisites

    log_indent log_dim "sq binary: $SQ_BINARY"
    log_indent log_dim "sq version: $("$SQ_BINARY" version 2>/dev/null || echo 'unknown')"
    log_success "Prerequisites Found Successfully"
}

# Start database containers (ClickHouse)
start_containers() {
    log "Starting Containers"
    start_services clickhouse

    # Wait for ClickHouse to be healthy
    if ! wait_for_healthy "clickhouse" 180; then
        log_error "ClickHouse did not become ready in time"
        show_service_logs clickhouse 100
        return 1
    fi

    # Give init scripts a moment to complete after health check
    log_dim "Waiting for init scripts to complete..."
    sleep 3

    # Manually run init scripts since docker entrypoint doesn't use --multiquery flag
    log_dim "Running init scripts..."
    docker exec clickhouse-clickhouse-1 bash -c "cat /docker-entrypoint-initdb.d/01-create-tables.sql | clickhouse-client --host localhost --port 9000 --user testuser --password testpass --database testdb --multiquery" >/dev/null 2>&1
    sleep 2
}

# Stop database containers
stop_containers() {
    stop_services
}

# ==============================================================================
# SQ Source Management
# ==============================================================================

# Add data sources to sq
add_sources() {
    log "Adding Data Sources"

    # Remove existing source if it exists
    "$SQ_BINARY" rm @test_clickhouse 2>/dev/null || true

    # Add ClickHouse source
    if run_sq "Adding ClickHouse source" add --handle @test_clickhouse "$CLICKHOUSE_DSN"; then
        :
    else
        log_error "Failed to add ClickHouse source"
        return 1
    fi

    log_success "Data sources added"
}

# Cleanup sq sources
cleanup_sources() {
    log_dim "Cleaning up sources..."

    # Remove source
    "$SQ_BINARY" rm @test_clickhouse 2>/dev/null || true
}

# ==============================================================================
# Test Functions
# ==============================================================================

# Test ClickHouse connectivity and basic operations
test_clickhouse_basic() {
    log "Testing ClickHouse Basic Operations"

    # Test 1: Inspect ClickHouse (show schema)
    log_dim "Test 1: Inspect ClickHouse schema"
    if "$SQ_BINARY" inspect @test_clickhouse >/dev/null 2>&1; then
        :
    else
        log_error "ClickHouse schema inspection failed"
        return 1
    fi

    # Test 2: Execute simple query using SQL
    log_dim "Test 2: Execute simple query (SELECT 1)"
    local result
    result=$("$SQ_BINARY" sql --src @test_clickhouse "SELECT 1 AS test_col" --json 2>/dev/null)
    if echo "$result" | /usr/bin/grep -q '"test_col"'; then
        :
    else
        log_error "Simple query failed"
        return 1
    fi

    # Test 3: Query current database
    log_dim "Test 3: Query current database"
    local database
    database=$("$SQ_BINARY" sql --src @test_clickhouse "SELECT currentDatabase() AS db" --json 2>/dev/null | /usr/bin/grep -o '"db": "[^"]*"' | cut -d'"' -f4)
    if [ -n "$database" ]; then
        log_indent log_dim "Current database: $database"
    else
        log_error "Failed to query current database"
        return 1
    fi

    # Test 4: Query ClickHouse version
    log_dim "Test 4: Query ClickHouse version"
    local version
    version=$("$SQ_BINARY" sql --src @test_clickhouse "SELECT version() AS ver" --json 2>/dev/null | /usr/bin/grep -o '"ver": "[^"]*"' | cut -d'"' -f4)
    if [ -n "$version" ]; then
        log_indent log_dim "ClickHouse version: $version"
    else
        log_error "Failed to query version"
        return 1
    fi

    log_success "ClickHouse basic operations test passed"
}

# Test creating and querying tables in ClickHouse
test_clickhouse_tables() {
    log "Testing ClickHouse Table Operations"

    local test_table
    test_table="${TEST_TABLE_PREFIX}_demo_$(date +%s)"

    # Test 1: Create a test table using SQL
    log_dim "Test 1: Create test table in ClickHouse"
    if "$SQ_BINARY" sql --src @test_clickhouse "CREATE TABLE IF NOT EXISTS \`${test_table}\` (id Int64, name String, value Float64, created_at DateTime) ENGINE = MergeTree() ORDER BY id" >/dev/null 2>&1; then
        :
    else
        log_error "Failed to create table in ClickHouse"
        return 1
    fi

    # Test 2: Insert data using SQL
    log_dim "Test 2: Insert test data"
    if "$SQ_BINARY" sql --src @test_clickhouse "INSERT INTO \`${test_table}\` VALUES (1, 'Alice', 123.45, '2024-01-15 10:30:00'), (2, 'Bob', 678.90, '2024-01-16 11:45:00')" >/dev/null 2>&1; then
        :
    else
        log_error "Failed to insert data"
        return 1
    fi

    # Test 3: Query the data back
    log_dim "Test 3: Query test data"
    local row_count
    row_count=$("$SQ_BINARY" sql --src @test_clickhouse "SELECT count(*) AS cnt FROM \`${test_table}\`" --json 2>/dev/null | /usr/bin/grep -oE '"cnt":\s*[0-9]+' | /usr/bin/grep -oE '[0-9]+')
    if [ -n "$row_count" ] && [ "$row_count" = "2" ]; then
        log_indent log_dim "Row count: $row_count"
    else
        log_error "Expected 2 rows, got: $row_count"
        return 1
    fi

    # Test 4: Query using sq syntax (SLQ)
    log_dim "Test 4: Query using sq syntax"
    if "$SQ_BINARY" --src @test_clickhouse ".${test_table} | .name, .value" --json >/dev/null 2>&1; then
        :
    else
        log_error "SLQ query failed"
        return 1
    fi

    # Test 5: Drop the test table
    log_dim "Test 5: Drop test table"
    if "$SQ_BINARY" sql --src @test_clickhouse "DROP TABLE \`${test_table}\`" >/dev/null 2>&1; then
        :
    else
        log_error "Failed to drop table"
        return 1
    fi

    log_success "ClickHouse table operations test passed"
}

# Test existing tables from init scripts
test_existing_tables() {
    log "Testing Existing Tables"

    # Test 1: Query users table
    log_dim "Test 1: Query users table"
    local user_count
    user_count=$("$SQ_BINARY" sql --src @test_clickhouse "SELECT count(*) AS cnt FROM users" --json 2>/dev/null | /usr/bin/grep -oE '"cnt":\s*[0-9]+' | /usr/bin/grep -oE '[0-9]+')
    if [ -n "$user_count" ] && [ "$user_count" -gt "0" ]; then
        log_indent log_dim "Users count: $user_count"
    else
        log_warning "No users found (init script may not have run)"
    fi

    # Test 2: Query events table
    log_dim "Test 2: Query events table"
    local event_count
    event_count=$("$SQ_BINARY" sql --src @test_clickhouse "SELECT count(*) AS cnt FROM events" --json 2>/dev/null | /usr/bin/grep -oE '"cnt":\s*[0-9]+' | /usr/bin/grep -oE '[0-9]+')
    if [ -n "$event_count" ] && [ "$event_count" -gt "0" ]; then
        log_indent log_dim "Events count: $event_count"
    else
        log_warning "No events found"
    fi

    # Test 3: Query products table
    log_dim "Test 3: Query products table"
    local product_count
    product_count=$("$SQ_BINARY" sql --src @test_clickhouse "SELECT count(*) AS cnt FROM products" --json 2>/dev/null | /usr/bin/grep -oE '"cnt":\s*[0-9]+' | /usr/bin/grep -oE '[0-9]+')
    if [ -n "$product_count" ] && [ "$product_count" -gt "0" ]; then
        log_indent log_dim "Products count: $product_count"
    else
        log_warning "No products found"
    fi

    log_success "Existing tables test passed"
}

# ==============================================================================
# Main Execution
# ==============================================================================

main() {
    log_separator
    log_banner
    log_info "ClickHouse CLI Test Runner"
    log ""

    # Check prerequisites
    check_prerequisites
    log ""

    # Start containers
    if ! start_containers; then
        log_error "Failed to start containers"
        exit 1
    fi
    log ""

    # Add sources
    if ! add_sources; then
        log_error "Failed to add sources"
        stop_containers
        exit 1
    fi
    log ""

    # Run tests
    local test_result=0

    # Test basic operations
    if ! test_clickhouse_basic; then
        test_result=1
    fi
    log ""

    # Test table operations
    if ! test_clickhouse_tables; then
        test_result=1
    fi
    log ""

    # Test existing tables
    if ! test_existing_tables; then
        test_result=1
    fi
    log ""

    # Cleanup
    cleanup_sources
    log ""

    stop_containers
    log ""

    # Exit with test result
    if [ $test_result -eq 0 ]; then
        log_success "CLI test run completed successfully"
        log_separator
        exit 0
    else
        log_error "CLI test run failed"
        log_separator
        exit 1
    fi
}

# Trap to ensure cleanup on exit
cleanup_on_exit() {
    if [ $? -ne 0 ]; then
        log_warning "Tests failed, cleaning up..."
        cleanup_sources 2>/dev/null || true
        force_stop_containers
    fi
}
trap cleanup_on_exit EXIT

# Run main function
main
