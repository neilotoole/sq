#!/usr/bin/env bash
# Upload site/public to Netlify production and poll until ready.
#
# Usage: netlify-deploy-prod.sh "Deploy message"
#
# Run from site/ after `make ci`. Requires NETLIFY_AUTH_TOKEN, NETLIFY_SITE_ID,
# bun, jq, curl.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SITE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
LOG_BASH="${SCRIPT_DIR}/log.bash"

if [[ -f "${LOG_BASH}" ]]; then
	# shellcheck disable=SC1090
	source "${LOG_BASH}"
else
	log_info() { echo "$*"; }
	log_error() { echo "ERROR: $*" >&2; }
	log_indent() { "$@"; }
	log_info_dim() { echo "  $*"; }
	log_success() { echo "$*"; }
fi

MESSAGE="${1:-}"
if [[ -z "${MESSAGE}" ]]; then
	log_error "Usage: netlify-deploy-prod.sh \"Deploy message\""
	exit 1
fi
if [[ -z "${NETLIFY_AUTH_TOKEN:-}" ]]; then
	log_error "NETLIFY_AUTH_TOKEN is not set."
	exit 1
fi
if [[ -z "${NETLIFY_SITE_ID:-}" ]]; then
	log_error "NETLIFY_SITE_ID is not set."
	exit 1
fi
if [[ ! -d "${SITE_DIR}/public" ]]; then
	log_error "site/public not found; run make ci first."
	exit 1
fi
if ! bun x netlify-cli --version >/dev/null 2>&1; then
	log_error "netlify-cli not available via bun x (run: cd site && bun install)."
	exit 1
fi

cd "${SITE_DIR}"

deploy_json=$(mktemp)
trap 'rm -f "${deploy_json}"' EXIT

log_info "Publishing site/public to Netlify (production)…"
log_indent log_dim "Message: ${MESSAGE}"

bun x netlify-cli deploy \
	--prod \
	--dir=public \
	--message "${MESSAGE}" \
	--json \
	| tee "${deploy_json}"

deploy_id=$(jq -r '.deploy_id // empty' "${deploy_json}")
deploy_url=$(jq -r '.deploy_url // .url // empty' "${deploy_json}")
log_indent log_info_dim "Deploy ID:  ${deploy_id:-<empty>}"
log_indent log_info_dim "Deploy URL: ${deploy_url:-<empty>}"
if [[ -z "${deploy_id}" ]]; then
	log_error "netlify-cli did not return deploy_id; cannot verify state."
	exit 1
fi

"${SCRIPT_DIR}/netlify-deploy-poll.sh" "${deploy_id}"
