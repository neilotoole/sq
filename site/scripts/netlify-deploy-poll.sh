#!/usr/bin/env bash
# Poll the Netlify API until a deploy reaches state=ready (or a terminal error).
#
# Usage: netlify-deploy-poll.sh DEPLOY_ID
#
# Requires: NETLIFY_AUTH_TOKEN, NETLIFY_SITE_ID, jq, curl
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_BASH="${SCRIPT_DIR}/log.bash"
if [[ -f "${LOG_BASH}" ]]; then
	# shellcheck disable=SC1090
	source "${LOG_BASH}"
else
	log_info() { echo "$*"; }
	log_error() { echo "ERROR: $*" >&2; }
	log_indent() { "$@"; }
	log_info_dim() { echo "  $*"; }
	log_warning() { echo "WARN: $*"; }
	log_success() { echo "$*"; }
fi

deploy_id="${1:-}"
if [[ -z "${deploy_id}" ]]; then
	log_error "Usage: netlify-deploy-poll.sh DEPLOY_ID"
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
if ! command -v jq >/dev/null 2>&1; then
	log_error "jq is required for deploy state polling."
	exit 1
fi

state=""
for attempt in $(seq 1 30); do
	body=$(curl -fsS \
		-H "Authorization: Bearer ${NETLIFY_AUTH_TOKEN}" \
		"https://api.netlify.com/api/v1/sites/${NETLIFY_SITE_ID}/deploys/${deploy_id}") \
		|| {
			log_indent log_warning "Attempt ${attempt}: API call failed, will retry"
			sleep 5
			continue
		}
	state=$(jq -r '.state // empty' <<<"${body}")
	log_indent log_info_dim "Attempt ${attempt}: state=${state:-<empty>}"
	case "${state}" in
	ready) break ;;
	error | rejected)
		log_error "Deploy ${deploy_id} ended in state=${state}."
		exit 1
		;;
	esac
	sleep 5
done

if [[ "${state}" != "ready" ]]; then
	log_error "Deploy ${deploy_id} did not reach state=ready (last: ${state:-<empty>})."
	exit 1
fi
log_success "Deploy ${deploy_id} reached state=ready."
