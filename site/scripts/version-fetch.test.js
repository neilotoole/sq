const { describe, test, expect } = require("bun:test");
const { parseReleaseTag } = require("./version-fetch.js");

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
