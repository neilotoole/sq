#!/usr/bin/env bash
# Install the package.json-pinned lychee release from its checksummed binary archive.

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
site_dir="$(cd "${script_dir}/.." && pwd)"
version="$(cd "${site_dir}" && bun -e 'console.log(require("./package.json").otherDependencies.lychee)')"
destination="${site_dir}/node_modules/.bin/lychee"

if [[ -x "${destination}" ]] && [[ "$("${destination}" --version)" == "lychee ${version}" ]]; then
  echo "lychee ${version} is already installed."
  exit 0
fi

case "$(uname -s):$(uname -m)" in
  Linux:x86_64)
    target="x86_64-unknown-linux-gnu"
    checksum="1f4e0ef7f6554a6ed33dd7ac144fb2e1bbed98598e7af973042fc5cd43951c9a"
    ;;
  Linux:aarch64 | Linux:arm64)
    target="aarch64-unknown-linux-gnu"
    checksum="91a7bd65685da41b90ccb9bc867a3d649a7818042dae04ff405e55a25bddee4c"
    ;;
  Darwin:x86_64)
    target="x86_64-apple-darwin"
    checksum="887503a9cff667d322b8d0892b40bf49976eb9507af8483220a3706cdad55978"
    ;;
  Darwin:arm64 | Darwin:aarch64)
    target="aarch64-apple-darwin"
    checksum="c9d3740ea2d891854d37116c9fba840f37b6e7c89d330e7db84ac333631c4977"
    ;;
  *)
    echo "Unsupported lychee platform: $(uname -s) $(uname -m)" >&2
    echo "Use Linux or macOS (x86_64 or arm64); Windows contributors should use WSL." >&2
    exit 1
    ;;
esac

archive="lychee-${target}.tar.gz"
url="https://github.com/lycheeverse/lychee/releases/download/lychee-v${version}/${archive}"
tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/sq-lychee.XXXXXX")"
trap 'rm -rf "${tmp_dir}"' EXIT

echo "Installing lychee ${version} for ${target}..."
curl --fail --silent --show-error --location --retry 3 \
  --output "${tmp_dir}/${archive}" "${url}"

if command -v sha256sum >/dev/null 2>&1; then
  actual_checksum="$(sha256sum "${tmp_dir}/${archive}" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  actual_checksum="$(shasum -a 256 "${tmp_dir}/${archive}" | awk '{print $1}')"
else
  echo "sha256sum or shasum is required to verify lychee." >&2
  exit 1
fi

if [[ "${actual_checksum}" != "${checksum}" ]]; then
  echo "lychee checksum mismatch for ${archive}." >&2
  echo "Expected: ${checksum}" >&2
  echo "Actual:   ${actual_checksum}" >&2
  exit 1
fi

tar -xzf "${tmp_dir}/${archive}" -C "${tmp_dir}"
mkdir -p "$(dirname "${destination}")"
install -m 0755 "${tmp_dir}/lychee-${target}/lychee" "${destination}"
"${destination}" --version
