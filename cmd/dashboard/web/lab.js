const areas = {
  performance: {
    title: "Performance Lab",
    source: "/benchmark.html",
    script: "/benchmark.js",
    styles: ["/benchmark.css", "/benchmark-focus.css"],
    shellClass: "benchmark-page",
  },
  camera: {
    title: "Camera Lab",
    source: "/camera.html",
    script: "/camera.js",
    styles: ["/camera.css"],
    shellClass: "camera-page",
  },
  demos: {
    title: "Demo Lab",
    source: "/demo.html",
    script: "/demo.js",
    styles: ["/demo.css"],
    shellClass: "demo-page",
  },
  guides: {
    title: "Test Guides",
    source: "/tests.html",
    script: "/tests.js",
    styles: ["/tests.css"],
    shellClass: "tests-page",
  },
};
function selectedArea() {
  const requested =
    new URLSearchParams(window.location.search).get("view") || "performance";
  return Object.hasOwn(areas, requested) ? requested : "performance";
}
function loadStyle(path) {
  if (document.querySelector('link[href="' + path + '"]')) return;
  const link = document.createElement("link");
  link.rel = "stylesheet";
  link.href = path;
  document.head.append(link);
}
function showError(error) {
  document.getElementById("connection").textContent = "Unavailable";
  document.getElementById("lab-content").innerHTML =
    '\n    <section class="lab-error">\n      <p class=' +
    '"eyebrow">LAB UNAVAILABLE</p>\n      <h1>This wor' +
    "kspace could not be opened</h1>\n      <p>" +
    String(error?.message || error).replace(
      /[&<>"']/g,
      (character) =>
        ({
          "&": "&amp;",
          "<": "&lt;",
          ">": "&gt;",
          '"': "&quot;",
          "'": "&#039;",
        })[character],
    ) +
    '</p>\n      <button type="button" onclick="window' +
    '.location.reload()">Try again</button>\n    </sec' +
    "tion>";
}
async function initialize() {
  const key = selectedArea();
  const area = areas[key];
  document
    .querySelector('[data-lab-view="' + key + '"]')
    ?.setAttribute("aria-current", "page");
  document.title = "" + area.title + " \xB7 VAR-Studio";
  area.styles.forEach(loadStyle);
  try {
    const response = await fetch(area.source, {
      cache: "no-store",
    });
    if (!response.ok)
      throw new Error(
        "Workspace template returned HTTP " + response.status + ".",
      );
    const page = new DOMParser().parseFromString(
      await response.text(),
      "text/html",
    );
    const sourceMain = page.querySelector(".shell > main");
    if (!sourceMain) throw new Error("Workspace template is incomplete.");
    const shell = document.getElementById("lab-shell");
    shell.className = "shell lab-page " + area.shellClass + "";
    const target = document.getElementById("lab-content");
    target.innerHTML = sourceMain.innerHTML;
    page.querySelectorAll(".shell > dialog").forEach((dialog) => {
      target.insertAdjacentHTML("beforeend", dialog.outerHTML);
    });
    await import(area.script);
  } catch (error) {
    console.error(error);
    showError(error);
  }
}
initialize();
