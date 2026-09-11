// Adds scroll position lock for default docs sidebar

if (document.querySelector("#sidebar-default") !== null) {
  const sidebar = document.getElementById("sidebar-default");

  const pos = sessionStorage.getItem("sidebar-scroll");
  if (pos !== null) {
    sidebar.scrollTop = parseInt(pos, 10);
  }

  window.addEventListener("beforeunload", () => {
    sessionStorage.setItem("sidebar-scroll", sidebar.scrollTop);
  });
}
