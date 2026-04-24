#!/usr/bin/env bun
/**
 * lighthouse-server.ts
 *
 * Tiny static file server used by `make site-lighthouse`. Serves the built
 * `public/` directory on a local port with on-the-fly brotli / gzip
 * compression so Lighthouse's "Enable text compression" audit reflects what
 * Netlify's edge actually does — not Hugo's dev server, which sends raw bytes.
 *
 * Intentionally dependency-free (Bun.serve + node:zlib) because pulling in
 * express/compression or sirv-cli just to satisfy one audit is overkill.
 *
 * Usage:
 *   bun scripts/lighthouse-server.ts [--port 8090] [--dir public]
 *
 * Shut down with SIGINT/SIGTERM; the `make site-lighthouse` target takes care
 * of that in the normal flow.
 */

import { existsSync, statSync } from "node:fs";
import { extname, join, resolve, sep } from "node:path";
import { brotliCompressSync, gzipSync } from "node:zlib";

function parseArgs(argv: string[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a.startsWith("--")) {
      const key = a.slice(2);
      const next = argv[i + 1];
      if (next && !next.startsWith("--")) {
        out[key] = next;
        i++;
      } else {
        out[key] = "true";
      }
    }
  }
  return out;
}

const args = parseArgs(process.argv.slice(2));
const port = Number(args.port ?? 8090);
const dir = resolve(String(args.dir ?? "public"));

if (!existsSync(dir) || !statSync(dir).isDirectory()) {
  console.error(`lighthouse-server: directory not found: ${dir}`);
  console.error("Hint: run `bun run build` first.");
  process.exit(1);
}

// Content types we care about. Everything else falls through to
// application/octet-stream — browsers sniff and render correctly anyway.
const MIME: Record<string, string> = {
  ".html": "text/html; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".js": "application/javascript; charset=utf-8",
  ".mjs": "application/javascript; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".gif": "image/gif",
  ".webp": "image/webp",
  ".avif": "image/avif",
  ".ico": "image/x-icon",
  ".woff": "font/woff",
  ".woff2": "font/woff2",
  ".ttf": "font/ttf",
  ".otf": "font/otf",
  ".xml": "application/xml; charset=utf-8",
  ".txt": "text/plain; charset=utf-8",
  ".map": "application/json; charset=utf-8",
  // Asciinema casts are newline-delimited JSON; serve as JSON so Netlify
  // (and this server) auto-compresses them. Matches the P3 Lighthouse fix.
  ".cast": "application/json; charset=utf-8",
};

// Compress text-y things only. Fonts/images are already compressed.
const COMPRESSIBLE = new Set([
  ".html",
  ".css",
  ".js",
  ".mjs",
  ".json",
  ".svg",
  ".xml",
  ".txt",
  ".map",
  ".cast",
]);

function resolveToFile(urlPath: string): string | null {
  let p = decodeURIComponent(urlPath.split("?")[0].split("#")[0]);
  if (p.startsWith("/")) p = p.slice(1);
  const target = resolve(dir, p);
  // Path traversal guard: target must stay inside `dir`.
  if (target !== dir && !target.startsWith(dir + sep)) return null;
  if (!existsSync(target)) return null;
  if (statSync(target).isDirectory()) {
    const idx = join(target, "index.html");
    return existsSync(idx) ? idx : null;
  }
  return target;
}

function pickEncoding(accept: string | null): "br" | "gzip" | null {
  if (!accept) return null;
  const a = accept.toLowerCase();
  if (a.includes("br")) return "br";
  if (a.includes("gzip")) return "gzip";
  return null;
}

const server = Bun.serve({
  port,
  hostname: "localhost",
  async fetch(req) {
    const url = new URL(req.url);
    const path = resolveToFile(url.pathname);

    if (!path) {
      const notFound = join(dir, "404.html");
      if (existsSync(notFound)) {
        const body = new Uint8Array(await Bun.file(notFound).arrayBuffer());
        return new Response(body, {
          status: 404,
          headers: { "content-type": MIME[".html"] },
        });
      }
      return new Response("Not Found", { status: 404 });
    }

    const ext = extname(path).toLowerCase();
    const type = MIME[ext] ?? "application/octet-stream";
    const raw = new Uint8Array(await Bun.file(path).arrayBuffer());

    const headers: Record<string, string> = {
      "content-type": type,
      // Modest cache — Lighthouse's cache-policy audit only wants some value.
      "cache-control": "public, max-age=3600",
      "x-content-type-options": "nosniff",
    };

    if (COMPRESSIBLE.has(ext)) {
      const enc = pickEncoding(req.headers.get("accept-encoding"));
      if (enc) {
        const compressed =
          enc === "br" ? brotliCompressSync(raw) : gzipSync(raw);
        return new Response(compressed, {
          headers: {
            ...headers,
            "content-encoding": enc,
            "content-length": String(compressed.length),
            vary: "accept-encoding",
          },
        });
      }
    }

    return new Response(raw, {
      headers: { ...headers, "content-length": String(raw.length) },
    });
  },
});

const shutdown = () => {
  server.stop();
  process.exit(0);
};
process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);

console.log(
  `lighthouse-server: http://localhost:${port} (serving ${dir}, gzip/br on)`,
);
