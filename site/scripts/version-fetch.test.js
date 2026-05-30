const { describe, test, expect } = require("bun:test");
const { parseReleaseTag, parseStarCount } = require("./version-fetch.js");
const { renderVersionJson } = require("./gen-site-data.js");

describe("parseReleaseTag", () => {
  test("strips the leading v from a stable tag", () => {
    expect(parseReleaseTag({ tag_name: "v0.52.0" })).toEqual({ version: "0.52.0" });
  });

  test("accepts a tag without a leading v", () => {
    expect(parseReleaseTag({ tag_name: "0.52.0" })).toEqual({ version: "0.52.0" });
  });

  test("rejects a pre-release tag_name", () => {
    expect(parseReleaseTag({ tag_name: "v0.53.0-rc.1" })).toHaveProperty("error");
  });

  test("rejects a build-metadata tag_name", () => {
    expect(parseReleaseTag({ tag_name: "v0.53.0+build.7" })).toHaveProperty("error");
  });

  test("rejects an empty tag_name", () => {
    expect(parseReleaseTag({ tag_name: "" })).toHaveProperty("error");
  });

  test("rejects a v-only tag_name", () => {
    expect(parseReleaseTag({ tag_name: "v" })).toHaveProperty("error");
  });

  test("rejects a missing tag_name", () => {
    expect(parseReleaseTag({})).toHaveProperty("error");
  });

  test("rejects a non-object response", () => {
    expect(parseReleaseTag(null)).toHaveProperty("error");
  });
});

describe("parseStarCount", () => {
  test("accepts a non-negative integer", () => {
    expect(parseStarCount({ stargazers_count: 1234 })).toEqual({ stars: 1234 });
  });

  test("accepts zero", () => {
    expect(parseStarCount({ stargazers_count: 0 })).toEqual({ stars: 0 });
  });

  test("rejects a negative count", () => {
    expect(parseStarCount({ stargazers_count: -1 })).toHaveProperty("error");
  });

  test("rejects a non-integer count", () => {
    expect(parseStarCount({ stargazers_count: 12.5 })).toHaveProperty("error");
  });

  test("rejects a non-number count", () => {
    expect(parseStarCount({ stargazers_count: "1234" })).toHaveProperty("error");
  });

  test("rejects a missing count", () => {
    expect(parseStarCount({})).toHaveProperty("error");
  });

  test("rejects a non-object response", () => {
    expect(parseStarCount(null)).toHaveProperty("error");
  });
});

describe("renderVersionJson", () => {
  test("emits legacy /version response shape", () => {
    expect(JSON.parse(renderVersionJson("0.53.0"))).toEqual({
      "latest-version": "0.53.0",
    });
  });
});
