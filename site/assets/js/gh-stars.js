(function () {
  var CACHE_KEY = 'sq-gh-stars-cache-v1';
  var CACHE_TTL_MS = 6 * 60 * 60 * 1000;

  function formatCount(n) {
    if (n >= 1000) {
      return (n / 1000).toFixed(1).replace(/\.0$/, '') + 'k';
    }
    return String(n);
  }

  function getCachedCount() {
    try {
      var cached = localStorage.getItem(CACHE_KEY);
      if (!cached) return null;

      var parsed = JSON.parse(cached);
      if (!parsed || typeof parsed.count !== 'number' || typeof parsed.at !== 'number') return null;
      if (Date.now() - parsed.at > CACHE_TTL_MS) return null;
      return parsed.count;
    } catch (_) {
      return null;
    }
  }

  function setCachedCount(count) {
    try {
      localStorage.setItem(CACHE_KEY, JSON.stringify({ count: count, at: Date.now() }));
    } catch (_) {
      // Ignore storage errors.
    }
  }

  function run() {
    var el = document.getElementById('gh-stars-count');
    if (!el) return;
    var cachedCount = getCachedCount();
    if (cachedCount !== null) {
      el.textContent = formatCount(cachedCount);
      return;
    }

    fetch('https://api.github.com/repos/neilotoole/sq', { method: 'GET', headers: { Accept: 'application/vnd.github.v3+json' } })
      .then(function (r) { return r.ok ? r.json() : Promise.reject(new Error(r.status)); })
      .then(function (d) {
        if (d && typeof d.stargazers_count === 'number') {
          setCachedCount(d.stargazers_count);
          el.textContent = formatCount(d.stargazers_count);
        }
      })
      .catch(function () { /* leave fallback ··· or clear */ });
  }

  function init() {
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', run);
    } else {
      run();
    }
  }
  if (typeof window !== 'undefined') {
    init();
  }
})();
