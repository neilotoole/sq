#!/usr/bin/env bash
# Build the Hugo site and check its generated links with lychee.

set -euo pipefail

scope="${LYCHEE_SCOPE:-full}"
if [[ -n "${1:-}" ]]; then
  scope="$1"
fi

if [[ "${scope}" != "internal" && "${scope}" != "full" ]]; then
  echo "Usage: $0 [internal|full]" >&2
  echo "LYCHEE_SCOPE may also select the scope (default: full)." >&2
  exit 2
fi

hugo="./node_modules/.bin/hugo/hugo"
lychee="./node_modules/.bin/lychee"
config="./lychee.toml"
lint_dir="./.serve-lint"

for required in "${hugo}" "${lychee}" "${config}"; do
  if [[ ! -e "${required}" ]]; then
    echo "Required link-check dependency not found: ${required}" >&2
    echo "Run 'make deps' from site/ first." >&2
    exit 1
  fi
done

rm -rf "${lint_dir}"
"${hugo}" --gc --minify --destination "${lint_dir}"

site_root="$(cd "${lint_dir}" && pwd -P)"
site_url="$(bun -e \
  'import { pathToFileURL } from "node:url"; console.log(pathToFileURL(process.argv.at(-1)).href)' \
  "${site_root}")"
args=(
  --config "${config}"
  --root-dir "${site_root}"
  --remap "^https://sq\\.io ${site_url}"
)

if [[ "${scope}" == "internal" ]]; then
  args+=(--offline)
fi

echo "Checking generated site links (scope=${scope})"
"${lychee}" "${args[@]}" "${lint_dir}/**/*.html"
