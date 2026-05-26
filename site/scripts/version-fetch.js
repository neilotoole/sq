/**
 * Fetches the latest stable sq release version from the GitHub Releases API.
 *
 * GitHub's "latest" endpoint returns the most recent non-prerelease, non-draft
 * release, so beta/rc tags are excluded by definition. The value is baked into
 * the site at build time by scripts/gen-version-data.js.
 */

const RELEASES_LATEST_URL =
  "https://api.github.com/repos/neilotoole/sq/releases/latest";
const FETCH_TIMEOUT_MS = 5000;
const USER_AGENT = "sq-site-version-fetch";

// Stable SemVer only: reject any pre-release (-rc.1, -beta) or build (+meta) suffix.
const STABLE_SEMVER_REGEX = /^\d+\.\d+\.\d+$/;

/**
 * Extract and validate the bare version from a GitHub release API response.
 * @param {unknown} json - Parsed JSON from /releases/latest.
 * @returns {{ version: string } | { error: string }}
 */
function parseReleaseTag(json) {
  if (!json || typeof json !== "object") {
    return { error: "release response is not an object" };
  }
  const tag = json.tag_name;
  if (typeof tag !== "string" || tag === "") {
    return { error: "release response has no tag_name" };
  }
  const version = tag.startsWith("v") ? tag.slice(1) : tag;
  if (!STABLE_SEMVER_REGEX.test(version)) {
    return { error: `not a stable semver tag: ${tag}` };
  }
  return { version };
}

/**
 * Download and parse the latest stable sq version from GitHub.
 * @returns {Promise<string>} Bare version (e.g. "0.52.0").
 * @throws {Error} On fetch, HTTP, or parse failure.
 */
async function getLatestVersion() {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);

  const headers = {
    Accept: "application/vnd.github+json",
    "X-GitHub-Api-Version": "2022-11-28",
    "User-Agent": USER_AGENT,
  };
  const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN;
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  try {
    const res = await fetch(RELEASES_LATEST_URL, {
      method: "GET",
      signal: controller.signal,
      headers,
    });
    clearTimeout(timeoutId);

    if (!res.ok) {
      throw new Error(
        `releases/latest fetch failed: ${res.status} ${res.statusText}`,
      );
    }

    const json = await res.json();
    const result = parseReleaseTag(json);
    if ("error" in result) {
      throw new Error(result.error);
    }
    return result.version;
  } catch (err) {
    clearTimeout(timeoutId);
    throw err;
  }
}

module.exports = { getLatestVersion, parseReleaseTag };
