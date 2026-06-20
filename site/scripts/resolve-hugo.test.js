const { describe, test, expect } = require("bun:test");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { resolveHugoBin, isExecutable } = require("./resolve-hugo.js");

describe("resolveHugoBin", () => {
  test("prefers node_modules/.bin/hugo/hugo when present", () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "sq-hugo-"));
    const hugoDir = path.join(tmp, "node_modules", ".bin", "hugo");
    fs.mkdirSync(hugoDir, { recursive: true });
    const hugoBin = path.join(hugoDir, "hugo");
    fs.writeFileSync(hugoBin, "");
    fs.chmodSync(hugoBin, 0o755);

    const got = resolveHugoBin(tmp);
    expect(got.source).toBe("vendored");
    expect(got.bin).toBe(hugoBin);
  });

  test("falls back to PATH when vendored binary is missing", () => {
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "sq-hugo-"));
    const got = resolveHugoBin(tmp);
    expect(got).toEqual({ bin: "hugo", source: "path" });
  });
});

describe("isExecutable", () => {
  test("returns false for a missing path", () => {
    expect(isExecutable(path.join(os.tmpdir(), "sq-no-such-hugo-bin"))).toBe(false);
  });

  test("respects execute permission on unix", () => {
    if (process.platform === "win32") return;

    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "sq-hugo-"));
    const p = path.join(tmp, "hugo");
    fs.writeFileSync(p, "");

    fs.chmodSync(p, 0o644);
    expect(isExecutable(p)).toBe(false);

    fs.chmodSync(p, 0o755);
    expect(isExecutable(p)).toBe(true);
  });
});
