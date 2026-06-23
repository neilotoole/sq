const announcement = document.getElementById("announcement");

if (announcement !== null) {
  const id = announcement.dataset.id;

  Object.keys(localStorage).forEach((key) => {
    if (/^global-alert-/.test(key)) {
      if (key !== id) {
        localStorage.removeItem(key);
        document.documentElement.removeAttribute("data-global-alert");
      }
    }
  });

  announcement.addEventListener("closed.bs.alert", () => {
    localStorage.setItem(id, "closed");
  });
}
