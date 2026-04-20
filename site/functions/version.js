const { getVersion } = require("./version-fetch.js");

const CACHE_MAX_AGE = 300; // 5 minutes

exports.handler = async () => {
  try {
    const version = await getVersion();
    return {
      statusCode: 200,
      headers: {
        "Content-Type": "application/json",
        "Cache-Control": `public, max-age=${CACHE_MAX_AGE}`,
      },
      body: JSON.stringify({ "latest-version": version }),
    };
  } catch (err) {
    return {
      statusCode: 503,
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ error: "unavailable" }),
    };
  }
};
