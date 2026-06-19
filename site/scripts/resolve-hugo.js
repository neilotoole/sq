/**
 * Resolve the Hugo binary for local site scripts.
 * Prefer the vendored binary from hugo-installer (postinstall); fall back to PATH.
 */

const fs = require("fs");
const path = require("path");
const { execFileSync } = require("child_process");

function isExecutable(filePath) {
  try {
    return fs.existsSync(filePath) && fs.statSync(filePath).isFile();
  } catch {
    return false;
  }
}

/**
 * @param {string} [cwd]
 * @returns {{ bin: string, source: "vendored" | "path" }}
 */
function resolveHugoBin(cwd = process.cwd()) {
  const vendored = path.join(cwd, "node_modules", ".bin", "hugo", "hugo");
  const vendoredAlt = path.join(cwd, "node_modules", ".bin", "hugo");
  if (isExecutable(vendored)) {
    return { bin: vendored, source: "vendored" };
  }
  if (isExecutable(vendoredAlt)) {
    return { bin: vendoredAlt, source: "vendored" };
  }
  return { bin: "hugo", source: "path" };
}

function hugoOnPath() {
  try {
    execFileSync("hugo", ["version"], { stdio: "ignore" });
    return true;
  } catch {
    return false;
  }
}

function readExpectedHugoVersion(cwd) {
  try {
    const pkg = JSON.parse(
      fs.readFileSync(path.join(cwd, "package.json"), "utf8")
    );
    return pkg.otherDependencies?.hugo;
  } catch {
    return undefined;
  }
}

/**
 * @param {string} [cwd]
 * @returns {string} Absolute path or "hugo" when on PATH.
 */
function requireHugoBin(cwd = process.cwd()) {
  const { bin, source } = resolveHugoBin(cwd);
  if (source === "vendored") {
    return bin;
  }
  if (hugoOnPath()) {
    return bin;
  }

  const expected = readExpectedHugoVersion(cwd);
  const versionHint = expected
    ? `Hugo Extended ${expected}`
    : "Hugo Extended (see package.json otherDependencies.hugo)";

  console.error("Hugo not found.\n");
  console.error("Install site dependencies from site/:");
  console.error("  make deps");
  console.error("  # or: bun install\n");
  console.error(
    "That runs hugo-installer and places the binary at node_modules/.bin/hugo/hugo"
  );
  console.error(
    `\nAlternatively, install ${versionHint} and ensure hugo is on PATH.`
  );
  process.exit(1);
}

module.exports = { resolveHugoBin, requireHugoBin, isExecutable };
