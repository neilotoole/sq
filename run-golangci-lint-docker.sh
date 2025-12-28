#!/usr/bin/env bash

# This script runs golangci-lint in Docker containers for both linux/arm64 and
# linux/amd64 architectures in parallel. This is useful because golangci-lint
# (specifically the goconst linter) can behave differently on different
# architectures. GitHub Actions CI runs on linux/amd64, so testing both
# architectures locally helps catch issues before pushing.
#
# The script mounts the project directory into the container and runs
# golangci-lint as if it were running in a GH action.
#
# Usage:
#   ./run-golangci-lint-docker.sh [golangci-lint args...]
#
# Examples:
#   ./run-golangci-lint-docker.sh
#   ./run-golangci-lint-docker.sh --verbose
#   ./run-golangci-lint-docker.sh ./libsq/...
#
# Exit codes:
#   0 - Both architectures passed
#   1 - One or both architectures failed

set -euo pipefail

# Version should match .github/workflows/main.yml
GOLANGCI_LINT_VERSION="v2.7.2"

# Get the directory where this script is located (project root)
SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"

# Temp files for capturing output
ARM64_OUTPUT=$(mktemp)
AMD64_OUTPUT=$(mktemp)

# Cleanup temp files on exit
# shellcheck disable=SC2329
cleanup() {
  # shellcheck disable=SC2317
  rm -f "${ARM64_OUTPUT}" "${AMD64_OUTPUT}"
}
trap cleanup EXIT

echo "Running golangci-lint ${GOLANGCI_LINT_VERSION} in Docker (linux/arm64 and linux/amd64)..."
echo "Project dir: ${SCRIPT_DIR}"
echo ""

# Run both architectures in parallel
# We use separate subshells to capture exit codes independently

(
  docker run --rm \
    --platform linux/arm64 \
    -v "${SCRIPT_DIR}:/app" \
    -w /app \
    "golangci/golangci-lint:${GOLANGCI_LINT_VERSION}" \
    golangci-lint run "$@" > "${ARM64_OUTPUT}" 2>&1
) &
ARM64_PID=$!

(
  docker run --rm \
    --platform linux/amd64 \
    -v "${SCRIPT_DIR}:/app" \
    -w /app \
    "golangci/golangci-lint:${GOLANGCI_LINT_VERSION}" \
    golangci-lint run "$@" > "${AMD64_OUTPUT}" 2>&1
) &
AMD64_PID=$!

# Wait for both to complete and capture exit codes
ARM64_EXIT=0
AMD64_EXIT=0

wait ${ARM64_PID} || ARM64_EXIT=$?
wait ${AMD64_PID} || AMD64_EXIT=$?

# Display results
echo "=== linux/arm64 (exit code: ${ARM64_EXIT}) ==="
cat "${ARM64_OUTPUT}"
echo ""

echo "=== linux/amd64 (exit code: ${AMD64_EXIT}) ==="
cat "${AMD64_OUTPUT}"
echo ""

# Summary
echo "=== Summary ==="
if [[ ${ARM64_EXIT} -eq 0 ]]; then
  echo "  linux/arm64: PASSED"
else
  echo "  linux/arm64: FAILED (exit code ${ARM64_EXIT})"
fi

if [[ ${AMD64_EXIT} -eq 0 ]]; then
  echo "  linux/amd64: PASSED"
else
  echo "  linux/amd64: FAILED (exit code ${AMD64_EXIT})"
fi

# Exit with failure if either failed
if [[ ${ARM64_EXIT} -ne 0 ]] || [[ ${AMD64_EXIT} -ne 0 ]]; then
  echo ""
  echo "One or more architectures failed!"
  exit 1
fi

echo ""
echo "Both architectures passed!"
exit 0
