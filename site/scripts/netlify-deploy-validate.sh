#!/usr/bin/env bash
# Deploy the current site/ tree to Netlify (deploy-preview context) and poll
# the API until state=ready. Used by `make site-netlify-validate` and the
# sq-site-dependabot skill (Layer B). Shares poll logic with
# netlify-deploy-poll.sh (production uses netlify-deploy-prod.sh).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SITE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
LOG_BASH="${SCRIPT_DIR}/log.bash"

if [[ -f "${LOG_BASH}" ]]; then
	# shellcheck disable=SC1090
	source "${LOG_BASH}"
else
	echo "Error: log.bash not found at ${LOG_BASH}" >&2
	exit 1
fi

cd "${SITE_DIR}"

ENV_FILE="${PWD}/.env"
if { [ -z "${NETLIFY_AUTH_TOKEN:-}" ] || [ -z "${NETLIFY_SITE_ID:-}" ]; } && [ -f "${ENV_FILE}" ]; then
	set -a
	# shellcheck disable=SC1090
	. "${ENV_FILE}"
	set +a
fi

if [ -z "${NETLIFY_AUTH_TOKEN:-}" ]; then
	log_error "NETLIFY_AUTH_TOKEN is not set (export or add to site/.env)."
	exit 1
fi
if [ -z "${NETLIFY_SITE_ID:-}" ]; then
	log_error "NETLIFY_SITE_ID is not set (export or add to site/.env)."
	exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
	log_error "jq is required for deploy state polling."
	exit 1
fi

if ! bun x netlify-cli --version >/dev/null 2>&1; then
	log_error "netlify-cli not available via bun x (run: cd site && bun install)."
	if command -v netlify >/dev/null 2>&1; then
		log_error "A global/brew netlify CLI is on PATH; site-netlify-validate requires the lockfile devDependency."
	fi
	exit 1
fi

DEPLOY_MESSAGE="${MESSAGE:-dependabot-validate $(date -u +%Y-%m-%dT%H:%MZ)}"
deploy_json=$(mktemp)
trap 'rm -f "${deploy_json}"' EXIT

log_info "Starting Netlify deploy-preview build (--build)…"
log_indent log_dim "Message: ${DEPLOY_MESSAGE}"

bun x netlify-cli deploy \
	--build \
	--context deploy-preview \
	--message "${DEPLOY_MESSAGE}" \
	--json \
	| tee "${deploy_json}"

deploy_id=$(jq -r '.deploy_id // empty' "${deploy_json}")
deploy_url=$(jq -r '.deploy_url // .url // empty' "${deploy_json}")
log_indent log_info_dim "Deploy ID:  ${deploy_id:-<empty>}"
log_indent log_info_dim "Deploy URL: ${deploy_url:-<empty>}"
if [ -z "${deploy_id}" ]; then
	log_error "netlify-cli did not return deploy_id; cannot verify state."
	exit 1
fi

"${SCRIPT_DIR}/netlify-deploy-poll.sh" "${deploy_id}"
