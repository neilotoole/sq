const globalDark = window.matchMedia?.("(prefers-color-scheme: dark)").matches;
const localMode = localStorage.getItem("theme");

if (globalDark && localMode === null) {
  localStorage.setItem("theme", "dark");
  document.documentElement.setAttribute("data-dark-mode", "");
}

if (globalDark && localMode === "dark") {
  document.documentElement.setAttribute("data-dark-mode", "");
}

if (localMode === "dark") {
  document.documentElement.setAttribute("data-dark-mode", "");
}
