/**
 * Local dev server: generates the build-time version data file (honoring
 * SQ_SITE_OFFLINE), then runs Hugo directly on PORT (default 1313). No Netlify
 * required locally.
 *
 * site/Makefile passes SQ_SITE_OFFLINE=1 by default for site-local, so the
 * header badge shows the committed last-known version without a network call;
 * use SQ_SITE_OFFLINE=0 make site-local to fetch the latest from GitHub.
 */

const path = require("path");
const { spawn, spawnSync } = require("child_process");
const { requireHugoBin } = require("./resolve-hugo.js");

const PORT = Number(process.env.PORT) || 1313;

const hugoBin = requireHugoBin();

// Bake the GitHub data file before Hugo starts (honors SQ_SITE_OFFLINE).
spawnSync("bun", [path.join(__dirname, "gen-site-data.js")], {
  stdio: "inherit",
  cwd: process.cwd(),
  env: process.env,
});

const hugoArgs = [
  "server",
  "--port",
  String(PORT),
  "--bind",
  "0.0.0.0",
  "--disableFastRender",
  "--logLevel",
  "info",
];
if (process.env.HUGO_BASEURL) {
  hugoArgs.push("--baseURL", process.env.HUGO_BASEURL, "--appendPort=false");
}
const hugo = spawn(hugoBin, hugoArgs, {
  stdio: "inherit",
  cwd: process.cwd(),
  env: { ...process.env, HUGO_ENV: "development" },
});

hugo.on("error", (err) => {
  console.error("Failed to start Hugo:", err);
  process.exit(1);
});

hugo.on("exit", (code) => {
  process.exit(code ?? 0);
});

// Forward termination signals so Hugo isn't orphaned when the wrapper is killed.
for (const sig of ["SIGINT", "SIGTERM"]) {
  process.on(sig, () => hugo.kill(sig));
}
