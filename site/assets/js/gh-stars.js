(function () {
  function formatCount(n) {
    if (n >= 1000) {
      return (n / 1000).toFixed(1).replace(/\.0$/, '') + 'k';
    }
    return String(n);
  }

  function run() {
    var el = document.getElementById('gh-stars-count');
    if (!el) return;
    fetch('https://api.github.com/repos/neilotoole/sq', { method: 'GET', headers: { Accept: 'application/vnd.github.v3+json' } })
      .then(function (r) { return r.ok ? r.json() : Promise.reject(new Error(r.status)); })
      .then(function (d) {
        if (d && typeof d.stargazers_count === 'number') {
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
