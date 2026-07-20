/**
 * Fetches the latest stable sq release and GitHub star count from the GitHub
 * API.
 *
 * GitHub's "latest" release endpoint returns the most recent non-prerelease,
 * non-draft release, so beta/rc tags are excluded by definition. These values
 * are baked into the site at build time by scripts/gen-site-data.js.
 */

const RELEASES_LATEST_URL = "https://api.github.com/repos/neilotoole/sq/releases/latest";
const REPO_URL = "https://api.github.com/repos/neilotoole/sq";
const FETCH_TIMEOUT_MS = 5000;
const USER_AGENT = "sq-site-version-fetch";

// Stable SemVer only: reject any pre-release (-rc.1, -beta) or build (+meta) suffix.
const STABLE_SEMVER_REGEX = /^\d+\.\d+\.\d+$/;

/**
 * GET a GitHub API URL and return parsed JSON, with a timeout and optional
 * token auth.
 * @param {string} url
 * @returns {Promise<unknown>}
 * @throws {Error} On fetch or HTTP failure.
 */
async function fetchGitHubJson(url) {
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
    const res = await fetch(url, {
      method: "GET",
      signal: controller.signal,
      headers,
    });
    clearTimeout(timeoutId);
    if (!res.ok) {
      throw new Error(`GitHub fetch failed: ${res.status} ${res.statusText} (${url})`);
    }
    return await res.json();
  } catch (err) {
    clearTimeout(timeoutId);
    throw err;
  }
}

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
 * Extract and validate the star count from a GitHub repo API response.
 * @param {unknown} json - Parsed JSON from the repo endpoint.
 * @returns {{ stars: number } | { error: string }}
 */
function parseStarCount(json) {
  if (!json || typeof json !== "object") {
    return { error: "repo response is not an object" };
  }
  const stars = json.stargazers_count;
  if (typeof stars !== "number" || !Number.isInteger(stars) || stars < 0) {
    return { error: `invalid stargazers_count: ${stars}` };
  }
  return { stars };
}

/**
 * Download and parse the latest stable sq version from GitHub.
 * @returns {Promise<string>} Bare version (e.g. "0.52.0").
 * @throws {Error} On fetch, HTTP, or parse failure.
 */
async function getLatestVersion() {
  const result = parseReleaseTag(await fetchGitHubJson(RELEASES_LATEST_URL));
  if ("error" in result) {
    throw new Error(result.error);
  }
  return result.version;
}

/**
 * Download and parse the sq GitHub star count.
 * @returns {Promise<number>} Star count (e.g. 1234).
 * @throws {Error} On fetch, HTTP, or parse failure.
 */
async function getStarCount() {
  const result = parseStarCount(await fetchGitHubJson(REPO_URL));
  if ("error" in result) {
    throw new Error(result.error);
  }
  return result.stars;
}

module.exports = {
  getLatestVersion,
  getStarCount,
  parseReleaseTag,
  parseStarCount,
};
