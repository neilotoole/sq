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

if [ ! -d "${SITE_DIR}" ]; then
	echo "site/ not found at ${SITE_DIR}" >&2
	exit 1
fi

# Netlify CLI is a site devDependency (bun x netlify-cli). Agents and fresh
# checkouts often lack node_modules; install before make check. Layer B also
# requires bun x (brew netlify alone is not enough for site-netlify-validate).
netlify_cli_in_node_modules() {
	[ -f "${SITE_DIR}/node_modules/netlify-cli/package.json" ]
}

ensure_site_deps() {
	if [ "${SKIP_SITE_DEPS:-}" = "1" ]; then
		echo "==> Site dependencies (skipped: SKIP_SITE_DEPS=1)"
		return 0
	fi
	echo "==> Site dependencies"
	if netlify_cli_in_node_modules; then
		echo "netlify-cli present in site/node_modules"
		return 0
	fi
	echo "netlify-cli not in node_modules; running bun install (needs network)…"
	(cd "${SITE_DIR}" && bun install)
	if ! netlify_cli_in_node_modules; then
		echo "error: netlify-cli still missing after bun install" >&2
		echo "From site/: bun install. Full mode requires bun x, not only brew netlify-cli." >&2
		exit 1
	fi
	echo "netlify-cli installed (verify with: cd site && bun x netlify-cli --version)"
}

ensure_site_deps

echo "==> Site tooling (make check)"
if [ "${1:-}" = "--netlify" ]; then
	(cd "${SITE_DIR}" && make check-netlify)
else
	(cd "${SITE_DIR}" && make check)
fi
