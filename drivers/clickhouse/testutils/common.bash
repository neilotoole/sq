#!/usr/bin/env bash
#
# common.bash - Shared utilities for ClickHouse driver test scripts
#
# This file contains common functions used by both test-integration.sh and test-sq-cli.sh:
# - Docker Compose detection and management
# - Container lifecycle (start, stop, wait for healthy)
# - Prerequisite checking
#
# Usage:
#   source "${SCRIPT_DIR}/common.bash"
#
# Required variables (set before sourcing):
#   SCRIPT_DIR - Directory containing this script
#
# Provided variables (set after sourcing):
#   DOCKER_COMPOSE - The docker compose command to use
#   DRIVER_DIR - Parent directory (drivers/clickhouse)

# Source logging utilities if not already sourced
if ! declare -f log_info >/dev/null 2>&1; then
    source "${SCRIPT_DIR}/log.bash"
fi

# Ensure SCRIPT_DIR is set
if [ -z "${SCRIPT_DIR:-}" ]; then
    log_error "ERROR: SCRIPT_DIR must be set before sourcing common.bash" >&2
    exit 1
fi


# ==============================================================================
# Directory Setup
# ==============================================================================

# DRIVER_DIR is the parent directory containing Go source files
DRIVER_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# ==============================================================================
# Docker Compose Detection
# ==============================================================================

# Determine the docker compose command (docker-compose or docker compose)
if command -v docker-compose >/dev/null 2>&1; then
    DOCKER_COMPOSE="docker-compose"
elif docker compose version >/dev/null 2>&1; then
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE=""  # Will be caught by check_docker_prerequisites
fi

# ==============================================================================
# Utility Functions
# ==============================================================================

# Check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# ==============================================================================
# Docker Prerequisites
# ==============================================================================

# Check Docker-related prerequisites (docker, docker-compose, daemon running)
# Returns 0 if all checks pass, exits with 1 otherwise
check_docker_prerequisites() {
    if ! command_exists docker; then
        log_error "docker is not installed or not in PATH"
        exit 1
    fi

    if [ -z "$DOCKER_COMPOSE" ]; then
        log_error "Neither docker-compose nor 'docker compose' is available"
        log "Please install Docker Compose: https://docs.docker.com/compose/install/"
        exit 1
    fi

    # Check if Docker daemon is running
    if ! docker info >/dev/null 2>&1; then
        log_error "Docker daemon is not running"
        log "Start Docker and try again"
        exit 1
    fi

    # Log version of docker compose
    log_dim "Docker Compose found: $($DOCKER_COMPOSE version --short)"

    return 0
}

# ==============================================================================
# Container Management
# ==============================================================================

# Wait for a docker-compose service to become healthy
# Usage: wait_for_healthy <service_name> [max_wait_seconds]
# Returns 0 if healthy, 1 if timeout or error
wait_for_healthy() {
    local service=$1
    local max_wait=${2:-120}  # Default 120 seconds
    local elapsed=0

    log_dim "Waiting for $service to be healthy (timeout: ${max_wait}s)..."

    while [ $elapsed -lt $max_wait ]; do
        local health
        health=$($DOCKER_COMPOSE ps -q "$service" | xargs docker inspect -f '{{.State.Health.Status}}' 2>/dev/null || echo "unknown")

        if [ "$health" = "healthy" ]; then
            log_success "$service is healthy (${elapsed}s)"
            return 0
        fi

        # Check if container is running
        local status
        status=$($DOCKER_COMPOSE ps -q "$service" | xargs docker inspect -f '{{.State.Status}}' 2>/dev/null || echo "not found")
        if [ "$status" != "running" ]; then
            log_error "$service container is not running (status: $status)"
            return 1
        fi

        echo -n "."
        sleep 2 || true
        elapsed=$((elapsed + 2))
    done

    log ""
    log_error "$service did not become healthy after ${max_wait}s"
    log_warning "Check logs: $DOCKER_COMPOSE logs $service"
    return 1
}

# Start specified docker-compose services
# Usage: start_services <service1> [service2] ...
# Must be called from SCRIPT_DIR or specify -f flag
start_services() {
    local services="$*"

    if [ -z "$services" ]; then
        log_error "No services specified"
        return 1
    fi

    log_dim "Starting containers: $services"

    pushd "$SCRIPT_DIR" > /dev/null || return 1
    $DOCKER_COMPOSE up -d --no-build $services
    local result=$?
    popd > /dev/null || return 1

    if [ $result -ne 0 ]; then
        log_error "Failed to start containers"
        return 1
    fi

    log_success "Containers started"
    return 0
}

# Stop all docker-compose services
# Usage: stop_services [--keep]
# If --keep or KEEP_CONTAINERS=true, containers are not stopped
stop_services() {
    local keep="${1:-}"

    if [ "$keep" = "--keep" ] || [ "${KEEP_CONTAINERS:-false}" = true ]; then
        log_info_dim "Keeping containers running (--keep flag specified)"
        log_info_dim "To stop manually, run: $DOCKER_COMPOSE down"
        return 0
    fi

    log_dim "Stopping containers..."

    pushd "$SCRIPT_DIR" > /dev/null || return
    $DOCKER_COMPOSE down
    popd > /dev/null || return

    log_success "Containers stopped"
}

# Show logs for a service
# Usage: show_service_logs <service> [tail_lines]
show_service_logs() {
    local service=$1
    local tail_lines=${2:-50}

    pushd "$SCRIPT_DIR" > /dev/null || return
    $DOCKER_COMPOSE logs --tail="$tail_lines" "$service"
    popd > /dev/null || return
}

# ==============================================================================
# Cleanup Helpers
# ==============================================================================

# Force stop all containers (for use in error handlers)
# Usage: force_stop_containers
force_stop_containers() {
    pushd "$SCRIPT_DIR" > /dev/null 2>&1 || true
    $DOCKER_COMPOSE down 2>/dev/null || true
    popd > /dev/null 2>&1 || true
}
