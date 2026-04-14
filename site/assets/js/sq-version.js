(function () {
  function run() {
    var el = document.getElementById('sq-version');
    if (!el) return;
    fetch('/version')
      .then(function (r) { return r.ok ? r.json() : Promise.reject(new Error(r.status)); })
      .then(function (d) {
        var v = d && d['latest-version'];
        if (typeof v === 'string') {
          el.textContent = (v.startsWith('v') ? v : 'v' + v);
        }
      })
      .catch(function () { /* leave fallback ··· */ });
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
