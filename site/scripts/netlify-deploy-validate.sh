#!/usr/bin/env bash
# Deploy the current site/ tree to Netlify (deploy-preview context) and poll
# the API until state=ready. Used by `make site-netlify-validate` and the
# sq-site-dependabot skill (Layer B). Mirrors polling logic in
# .github/workflows/site-publish-dispatch.yml.
#
# TODO: Deduplicate deploy JSON parse + API poll loop with site-publish-dispatch.yml
# (e.g. site/scripts/netlify-deploy-poll.sh). Paths differ: --build deploy-preview
# here vs --prod --dir=public in CI; keep both entry points, share poll-only helper.
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

if [ "${state}" != "ready" ]; then
	log_error "Deploy ${deploy_id} did not reach state=ready (last: ${state:-<empty>})."
	exit 1
fi
log_success "Deploy ${deploy_id} reached state=ready."
