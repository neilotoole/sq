const zlib = require("node:zlib");
const { getVersion } = require("./version-fetch.js");

const CACHE_MAX_AGE = 300; // 5 minutes

/**
 * JSON response with Brotli or gzip when the client advertises support and
 * compression saves bytes. Netlify does not apply CDN compression to
 * function payloads the way it does for static files, so this keeps
 * Lighthouse's "Enable text compression" audit clean for GET /version.
 *
 * @param {number} statusCode
 * @param {Record<string, unknown>} bodyObj
 * @param {string} acceptEncoding
 */
function jsonResponse(statusCode, bodyObj, acceptEncoding) {
  const json = JSON.stringify(bodyObj);
  const raw = Buffer.from(json, "utf8");
  const ae = acceptEncoding.toLowerCase();

  const baseHeaders = {
    "Content-Type": "application/json; charset=utf-8",
  };
  if (statusCode === 200) {
    baseHeaders["Cache-Control"] = `public, max-age=${CACHE_MAX_AGE}`;
  }

  const tryCompressed = (encodingName, compressed) => {
    if (compressed.length >= raw.length) {
      return null;
    }
    return {
      statusCode,
      headers: {
        ...baseHeaders,
        "Content-Encoding": encodingName,
        Vary: "Accept-Encoding",
      },
      body: compressed.toString("base64"),
      isBase64Encoded: true,
    };
  };

  if (ae.includes("br")) {
    const br = zlib.brotliCompressSync(raw);
    const r = tryCompressed("br", br);
    if (r) {
      return r;
    }
  }
  if (ae.includes("gzip")) {
    const gz = zlib.gzipSync(raw);
    const r = tryCompressed("gzip", gz);
    if (r) {
      return r;
    }
  }

  return {
    statusCode,
    headers: baseHeaders,
    body: json,
  };
}

exports.handler = async (event) => {
  const acceptEncoding =
    event.headers?.["accept-encoding"] ||
    event.headers?.["Accept-Encoding"] ||
    "";

  try {
    const version = await getVersion();
    return jsonResponse(
      200,
      { "latest-version": version },
      acceptEncoding,
    );
  } catch {
    return jsonResponse(503, { error: "unavailable" }, acceptEncoding);
  }
};
