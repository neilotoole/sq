Object.keys(localStorage).forEach((key) => {
  if (/^global-alert-/.test(key)) {
    document.documentElement.setAttribute("data-global-alert", "closed");
  }
});
