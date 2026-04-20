/**
 * Local dev server: runs Hugo on an internal port and handles GET /version
 * by fetching the Homebrew formula (same as the Netlify function).
 * No Netlify required locally.
 */

const http = require("http");
const path = require("path");
const fs = require("fs");
const { spawn } = require("child_process");
const { getVersion } = require("./version-fetch.js");

const HUGO_PORT = Number(process.env.HUGO_PORT) || 1314;
const SERVER_PORT = Number(process.env.SERVER_PORT) || process.env.PORT || 1313;

const root = process.cwd();
const nodeHugoA = path.join(root, "node_modules", ".bin", "hugo", "hugo");
const nodeHugoB = path.join(root, "node_modules", ".bin", "hugo");
function isExecutable(p) {
  try {
    return fs.existsSync(p) && fs.statSync(p).isFile();
  } catch {
    return false;
  }
}
const hugoBin =
  isExecutable(nodeHugoA)
    ? nodeHugoA
    : isExecutable(nodeHugoB)
      ? nodeHugoB
      : "hugo";

function waitForHugo() {
  return new Promise((resolve, reject) => {
    const deadline = Date.now() + 60000;
    function tryFetch() {
      if (Date.now() > deadline) {
        reject(new Error("Hugo did not become ready in time"));
        return;
      }
      const req = http.get(
        `http://127.0.0.1:${HUGO_PORT}/`,
        (res) => {
          res.resume();
          resolve();
        }
      );
      req.on("error", () => {
        setTimeout(tryFetch, 300);
      });
    }
    tryFetch();
  });
}

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

const server = http.createServer(async (req, res) => {
  const pathname = req.url?.split("?")[0] ?? "/";

  if (req.method === "GET" && pathname === "/version") {
    try {
      const version = await getVersion();
      res.writeHead(200, {
        "Content-Type": "application/json",
        "Cache-Control": "public, max-age=300",
      });
      res.end(JSON.stringify({ "latest-version": version }));
    } catch (_) {
      res.writeHead(503, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "unavailable" }));
    }
    return;
  }

  // Proxy to Hugo
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
    }
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
      console.log(`Dev server at http://localhost:${SERVER_PORT} (Hugo + /version)`);
    });
  })
  .catch((err) => {
    console.error(err);
    process.exit(1);
  });
