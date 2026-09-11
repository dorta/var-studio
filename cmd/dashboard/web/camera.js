const $ = (id) => document.getElementById(id);
let activeDevice = "";
function escapeHTML(value) {
  return String(value ?? "").replace(
    /[&<>"']/g,
    (character) =>
      ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#039;",
      })[character],
  );
}
function updateClock() {
  $("clock").textContent = new Date().toLocaleTimeString([], {
    hour12: false,
  });
}
function stopStream() {
  activeDevice = "";
  $("camera-stream").removeAttribute("src");
  $("preview-stage").classList.add("empty");
  $("preview-empty").hidden = false;
  $("close-stream").hidden = true;
  $("preview-title").textContent = "No camera selected";
  $("live-badge").textContent = "STANDBY";
  $("live-badge").classList.remove("live");
}
function startStream(path, name) {
  activeDevice = path;
  $("preview-title").textContent = name;
  $("preview-empty").hidden = true;
  $("preview-stage").classList.remove("empty");
  $("camera-stream").src =
    "/api/v1/camera-stream?device=" +
    encodeURIComponent(path) +
    "&t=" +
    Date.now() +
    "";
  $("close-stream").hidden = false;
  $("live-badge").textContent = "LIVE";
  $("live-badge").classList.add("live");
  $("preview-stage").scrollIntoView({
    behavior: "smooth",
    block: "center",
  });
}
function renderDevices(inventory) {
  const devices = inventory.devices || [],
    captureCount = Number(inventory.capture_count || 0);
  $("device-count").textContent =
    "" + devices.length + " DEVICE" + (devices.length === 1 ? "" : "S") + "";
  $("camera-message").textContent =
    inventory.message || "Camera status is unavailable.";
  $("camera-status").textContent = captureCount
    ? "" + captureCount + " camera" + (captureCount === 1 ? "" : "s") + " ready"
    : "No camera ready";
  $("status-icon").className =
    "status-orb " + (captureCount ? "ready" : "warning") + "";
  if (!devices.length) {
    $("device-list").innerHTML =
      '<div class="device-empty"><strong>No V4L2 device' +
      "s detected</strong><p>Connect a supported camera" +
      " and refresh this page.</p></div>";
    return;
  }
  $("device-list").innerHTML = devices
    .map((device) => {
      const capabilities = (device.capabilities || [])
        .map((item) => "<span>" + escapeHTML(item) + "</span>")
        .join("");
      return (
        '<article class="video-device ' +
        (device.capture ? "capture" : "accelerator") +
        '"><div class="device-icon">' +
        (device.capture ? "CAM" : "V4L") +
        '</div><div class="device-copy"><div class="devic' +
        'e-heading"><h3>' +
        escapeHTML(device.name || device.path) +
        "</h3><code>" +
        escapeHTML(device.path) +
        "</code></div><p>" +
        escapeHTML(device.role) +
        " \xB7 Driver " +
        escapeHTML(device.driver || "unknown") +
        '</p><div class="capability-list">' +
        capabilities +
        "</div></div>" +
        (device.capture
          ? '<button class="open-camera" data-path="' +
            escapeHTML(device.path) +
            '" data-name="' +
            escapeHTML(device.name || device.path) +
            '">Open live view</button>'
          : '<span class="not-camera">NOT A CAMERA</span>') +
        "</article>"
      );
    })
    .join("");
  document
    .querySelectorAll(".open-camera")
    .forEach((button) =>
      button.addEventListener("click", () =>
        startStream(button.dataset.path, button.dataset.name),
      ),
    );
}
async function loadDevices() {
  $("refresh").disabled = true;
  try {
    const response = await fetch("/api/v1/cameras", {
      cache: "no-store",
    });
    if (!response.ok) throw new Error("HTTP " + response.status + "");
    renderDevices(await response.json());
    $("connection").textContent = "Live";
  } catch (error) {
    $("connection").textContent = "Unavailable";
    $("camera-status").textContent = "Camera service unavailable";
    $("camera-message").textContent =
      "The host camera service is not running or cannot" + " be reached.";
    $("status-icon").className = "status-orb error";
    $("device-count").textContent = "OFFLINE";
    $("device-list").innerHTML =
      '<div class="device-empty error"><strong>Unable t' +
      "o inspect cameras</strong><p>" +
      escapeHTML(error.message) +
      "</p></div>";
  } finally {
    $("refresh").disabled = false;
  }
}
$("refresh").addEventListener("click", loadDevices);
$("close-stream").addEventListener("click", stopStream);
$("camera-stream").addEventListener("error", () => {
  if (!activeDevice) return;
  $("live-badge").textContent = "STREAM ERROR";
  $("live-badge").classList.remove("live");
});
window.addEventListener("beforeunload", stopStream);
updateClock();
setInterval(updateClock, 1000);
loadDevices();
