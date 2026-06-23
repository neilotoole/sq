/**
 * Local dev server: generates the build-time version data file (honoring
 * SQ_SITE_OFFLINE), then runs Hugo and proxies to it. No Netlify required
 * locally.
 *
 * site/Makefile passes SQ_SITE_OFFLINE=1 by default for site-local, so the
 * header badge shows the committed last-known version without a network call;
 * use SQ_SITE_OFFLINE=0 make site-local to fetch the latest from GitHub.
 */

const http = require("http");
const path = require("path");
const { spawn, spawnSync } = require("child_process");
const { requireHugoBin } = require("./resolve-hugo.js");

const HUGO_PORT = Number(process.env.HUGO_PORT) || 1314;
const SERVER_PORT = Number(process.env.SERVER_PORT) || process.env.PORT || 1313;

const hugoBin = requireHugoBin();

function waitForHugo() {
  return new Promise((resolve, reject) => {
    const deadline = Date.now() + 60000;
    function tryFetch() {
      if (Date.now() > deadline) {
        reject(new Error("Hugo did not become ready in time"));
        return;
      }
      const req = http.get(`http://127.0.0.1:${HUGO_PORT}/`, (res) => {
        res.resume();
        resolve();
      });
      req.on("error", () => {
        setTimeout(tryFetch, 300);
      });
    }
    tryFetch();
  });
}

// Bake the GitHub data file before Hugo starts (honors SQ_SITE_OFFLINE).
spawnSync("bun", [path.join(__dirname, "gen-site-data.js")], {
  stdio: "inherit",
  cwd: process.cwd(),
  env: process.env,
});

const hugoArgs = [
  "server",
  "--port",
  String(HUGO_PORT),
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
  if (code != null && code !== 0) process.exit(code);
});

const server = http.createServer((req, res) => {
  // Proxy to Hugo.
  const proxyReq = http.request(
    {
      hostname: "127.0.0.1",
      port: HUGO_PORT,
      path: req.url,
      method: req.method,
      headers: req.headers,
    },
    (proxyRes) => {
      res.writeHead(proxyRes.statusCode ?? 200, proxyRes.headers);
      proxyRes.pipe(res);
    },
  );
  proxyReq.on("error", () => {
    res.writeHead(502, { "Content-Type": "text/plain" });
    res.end("Bad Gateway");
  });
  req.pipe(proxyReq);
});

waitForHugo()
  .then(() => {
    server.listen(SERVER_PORT, "0.0.0.0", () => {
      console.log(`Dev server at http://localhost:${SERVER_PORT} (Hugo)`);
    });
  })
  .catch((err) => {
    console.error(err);
    process.exit(1);
  });
