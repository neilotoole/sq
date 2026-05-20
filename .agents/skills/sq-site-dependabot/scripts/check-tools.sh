#!/usr/bin/env bash
# Phase 0 tool bootstrap for sq-site-dependabot.
# Site tools: cd site && make check (or make check-netlify for Layer B).
# Usage: check-tools.sh [--netlify]
set -euo pipefail

# gh respects PAGER/GH_PAGER; less shows "(END)" and waits for q/Enter.
export GH_PAGER=cat
export PAGER=cat

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"
SITE_DIR="${REPO_ROOT}/site"

echo "==> GitHub CLI"
command -v gh >/dev/null
gh --version | head -1
if ! gh api user -q .login >/dev/null 2>&1; then
	echo "gh is not authenticated for github.com (run: gh auth login)" >&2
	exit 1
fi
gh_login=$(gh api user -q .login)
echo "Logged in as ${gh_login}"

echo "==> Site tooling (make check)"
if [ ! -d "${SITE_DIR}" ]; then
	echo "site/ not found at ${SITE_DIR}" >&2
	exit 1
fi
if [ "${1:-}" = "--netlify" ]; then
	(cd "${SITE_DIR}" && make check-netlify)
else
	(cd "${SITE_DIR}" && make check)
fi
