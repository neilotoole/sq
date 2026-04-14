/**
 * Fetches the current sq version from the Homebrew formula (homebrew-core).
 * Port of the Go logic in sq's version command.
 * (Copy used by Netlify function; scripts/version-fetch.js is used by dev server.)
 */

const FORMULA_URL =
  "https://raw.githubusercontent.com/Homebrew/homebrew-core/HEAD/Formula/s/sq.rb";
const FETCH_TIMEOUT_MS = 5000;

const SEMVER_REGEX = /^\d+\.\d+\.\d+(-.+)?$/;

function getVersionFromBrewFormula(body) {
  let urlVersion = null;
  const lines = body.split(/\r?\n/);

  for (const line of lines) {
    const val = line.trim();

    if (val.startsWith("bottle")) {
      break;
    }

    if (val.startsWith('version "')) {
      const version = val.slice(9, val.length - 1);
      if (!SEMVER_REGEX.test(version)) {
        return { error: `invalid semver: ${version}` };
      }
      return { version };
    }

    if (val.startsWith('url "') && val.includes("/tags/v")) {
      const idx = val.indexOf("/tags/v");
      if (idx !== -1) {
        const remainder = val.slice(idx + 7);
        let version;
        if (remainder.includes(".tar.gz")) {
          version = remainder.split(".tar.gz")[0];
        } else if (remainder.includes(".zip")) {
          version = remainder.split(".zip")[0];
        } else {
          continue;
        }
        if (SEMVER_REGEX.test(version)) {
          urlVersion = version;
        }
      }
    }
  }

  if (urlVersion) {
    return { version: urlVersion };
  }
  return { error: "invalid brew formula" };
}

async function getVersion() {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);

  try {
    const res = await fetch(FORMULA_URL, {
      method: "GET",
      signal: controller.signal,
      headers: { Accept: "text/plain" },
    });

    clearTimeout(timeoutId);

    if (!res.ok) {
      throw new Error(`formula fetch failed: ${res.status} ${res.statusText}`);
    }

    const body = await res.text();
    const result = getVersionFromBrewFormula(body);

    if ("error" in result) {
      throw new Error(result.error);
    }
    return result.version;
  } catch (err) {
    clearTimeout(timeoutId);
    throw err;
  }
}

module.exports = { getVersion, getVersionFromBrewFormula };
