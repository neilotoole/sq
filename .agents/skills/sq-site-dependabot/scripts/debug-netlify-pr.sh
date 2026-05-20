#!/usr/bin/env bash
# Debug a failed Netlify deploy-preview check for a site Dependabot PR.
# Usage (repo root): debug-netlify-pr.sh <pr-number>
# Requires: gh auth, site/.env (NETLIFY_AUTH_TOKEN, NETLIFY_SITE_ID), bun x netlify-cli
set -euo pipefail

export GH_PAGER=cat
export PAGER=cat

PR="${1:?Usage: debug-netlify-pr.sh <pr-number>}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"
SITE_DIR="${REPO_ROOT}/site"

echo "==> PR #${PR} — GitHub checks"
if ! gh pr checks "${PR}" 2>&1; then
	echo "(some checks failed — expected when debugging)" >&2
fi

HEAD_OID=$(gh pr view "${PR}" --json headRefOid --jq '.headRefOid')
echo ""
echo "headRefOid=${HEAD_OID}"

echo ""
echo "==> Netlify-related check lines"
gh pr checks "${PR}" 2>&1 | /usr/bin/grep -iE 'netlify|sq-web|Header rules|Redirect|Pages changed' || true

CHECKS_OUT=$(mktemp)
DEPLOY_JSON=""
cleanup() {
	rm -f "${CHECKS_OUT}"
	if [ -n "${DEPLOY_JSON}" ]; then
		rm -f "${DEPLOY_JSON}"
	fi
}
trap cleanup EXIT
gh pr checks "${PR}" >"${CHECKS_OUT}" 2>&1 || true
DEPLOY_ID=$(/usr/bin/grep -oE 'deploys/[a-f0-9]{24}' "${CHECKS_OUT}" | head -1 | /usr/bin/cut -d/ -f2 || true)

if [ -z "${DEPLOY_ID}" ]; then
	echo "Could not parse deploy_id from gh pr checks URLs; open the Netlify check link manually." >&2
	exit 1
fi

echo ""
echo "deploy_id=${DEPLOY_ID}"

if [ ! -f "${SITE_DIR}/.env" ]; then
	echo "Missing ${SITE_DIR}/.env — copy from .env.example for API calls." >&2
	exit 1
fi

# shellcheck disable=SC1091
set -a && . "${SITE_DIR}/.env" && set +a

if [ -z "${NETLIFY_AUTH_TOKEN:-}" ] || [ -z "${NETLIFY_SITE_ID:-}" ]; then
	echo "NETLIFY_AUTH_TOKEN and NETLIFY_SITE_ID must be set in site/.env" >&2
	exit 1
fi

echo ""
echo "==> getDeploy (netlify-cli api)"
DEPLOY_JSON=$(mktemp)
(cd "${SITE_DIR}" && bun x netlify-cli api getDeploy --data "{\"deploy_id\":\"${DEPLOY_ID}\"}") >"${DEPLOY_JSON}"

if command -v jq >/dev/null 2>&1; then
	jq '{state, error_message, commit_ref, context, review_id, build_id, deploy_ssl_url}' "${DEPLOY_JSON}"
	BUILD_ID=$(jq -r '.build_id // empty' "${DEPLOY_JSON}")
	COMMIT_REF=$(jq -r '.commit_ref // empty' "${DEPLOY_JSON}")
	if [ -n "${COMMIT_REF}" ] && [ "${COMMIT_REF}" != "${HEAD_OID}" ]; then
		echo "warning: deploy commit_ref ${COMMIT_REF} != PR headRefOid ${HEAD_OID}" >&2
	fi
	if [ -n "${BUILD_ID}" ]; then
		echo ""
		echo "==> getSiteBuild"
		(cd "${SITE_DIR}" && bun x netlify-cli api getSiteBuild --data "{\"build_id\":\"${BUILD_ID}\"}") \
			| jq '{deploy_state, error, done, sha}' 2>/dev/null || true
	fi
else
	cat "${DEPLOY_JSON}"
fi

echo ""
echo "==> Deploy log (UI)"
echo "  Open the Netlify check URL above → Deploy log (install errors are often"
echo "  only visible here, e.g. netlify-cli postinstall / execa ESM vs CJS)."
echo ""
echo "==> Local reproduction (optional; Node version may differ from Netlify)"
echo "  gh pr checkout ${PR}"
echo "  cd site && bun install    # mirrors Netlify 'Install dependencies'"
echo "  cd site && make ci        # if install succeeds"
echo ""
echo "See references/netlify-build-debug.md for interpretation."
