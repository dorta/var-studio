const $ = (id) => document.getElementById(id);
const state = {
  diagnostics: null,
  demos: [],
  demoStatus: {
    state: "idle",
  },
  mlDemo: null,
  displayDemos: [],
  displaySelection: "",
  displayError: "",
  cameras: [],
  activeDevice: "",
  captureBlob: null,
};
const escapeHTML = (value) =>
  String(value ?? "").replace(
    /[&<>"']/g,
    (c) =>
      ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#039;",
      })[c],
  );
const capability = (id) =>
  (state.diagnostics?.capabilities || []).find((item) => item.id === id);
function updateClock() {
  $("clock").textContent = new Date().toLocaleTimeString([], {
    hour12: false,
  });
}
function card(kind, title, description, available, detail, action) {
  return (
    '<article class="demo-card ' +
    (available ? "available" : "unavailable") +
    '" data-kind="' +
    kind +
    '"><div class="demo-card-visual"></div><div class' +
    '="demo-card-copy"><div class="demo-card-head"><h' +
    "2>" +
    escapeHTML(title) +
    '</h2><span class="capability-state">' +
    (available ? "READY" : "UNAVAILABLE") +
    "</span></div><p>" +
    escapeHTML(description) +
    "</p>" +
    (available
      ? action
      : '<p class="unavailable-reason">' +
        escapeHTML(
          detail || "Required capability is not exposed by this board image.",
        ) +
        '</p><button type="button" disabled>Not available' +
        "</button>") +
    "</div></article>"
  );
}
function renderCatalog() {
  const camera = capability("camera"),
    gpu = capability("gpu") || capability("drm"),
    ml = capability("npu"),
    mlDemo = state.demos.find((item) => item.kind === "ml" && item.installed),
    mlReady = Boolean(mlDemo),
    audio = capability("audio"),
    gpio = capability("gpio"),
    remote = capability("remoteproc"),
    displayCards = state.displayDemos.map((demo) =>
      card(
        demo.kind,
        demo.name,
        demo.description,
        demo.installed,
        demo.api + " bundle is not installed on this image.",
        '<button type="button" data-run-display="' +
          escapeHTML(demo.id) +
          '">Launch on board display</button>',
      ),
    );
  const entries = [
    ...displayCards,
    card(
      "camera",
      "Camera capture",
      "Preview a connected V4L2 camera, freeze a frame," +
        " and share it locally.",
      !!camera?.available,
      camera?.detail,
      '<button type="button" data-open-camera>Open expe' + "rience</button>",
    ),
    card(
      "gpu",
      "GPU graphics",
      "Run allowlisted, time-limited graphics workloads" +
        " and inspect CPU, GPU, and memory impact.",
      !!gpu?.available,
      gpu?.detail,
      '<a href="/lab.html?view=performance">Open Benchm' + "ark Lab</a>",
    ),
    card(
      "ml",
      "ML inference",
      "Run an image classification experience when a su" +
        "pported NPU runtime and model bundle are present" +
        ".",
      mlReady,
      mlDemo?.api || ml?.detail,
      '<button type="button" data-open-ml>Open ML demo<' + "/button>",
    ),
    card(
      "io",
      "Audio loopback",
      "Inspect and exercise an exposed ALSA audio devic" +
        "e with an explicit local test.",
      !!audio?.available,
      audio?.detail,
      '<a href="/diagnostics.html">Inspect audio capabi' + "lity</a>",
    ),
    card(
      "io",
      "GPIO explorer",
      "Visualize exposed GPIO controllers. Output-chang" +
        "ing actions remain disabled by default.",
      !!gpio?.available,
      gpio?.detail,
      '<a href="/diagnostics.html">Open Hardware Lab</a' + ">",
    ),
    card(
      "io",
      "Remote processor",
      "Inspect Cortex-M/remoteproc state and firmware e" +
        "xposure without changing its running state.",
      !!remote?.available,
      remote?.detail,
      '<a href="/diagnostics.html">Inspect remote proce' + "ssors</a>",
    ),
  ];
  $("demo-catalog").innerHTML = entries.join("");
  $("available-count").textContent =
    [camera, gpu, audio, gpio, remote].filter((item) => item?.available)
      .length +
    (mlReady ? 1 : 0) +
    state.displayDemos.filter((demo) => demo.installed).length;
  document
    .querySelector("[data-open-camera]")
    ?.addEventListener("click", () => {
      $("camera-experience").hidden = false;
      $("camera-experience").scrollIntoView({
        behavior: "smooth",
        block: "start",
      });
      loadCameras();
    });
  document.querySelector("[data-open-ml]")?.addEventListener("click", () => {
    document.getElementById("ml-experience").hidden = false;
    document.getElementById("ml-experience").scrollIntoView({
      behavior: "smooth",
      block: "start",
    });
    renderMLStatus();
  });
  document.querySelectorAll("[data-run-display]").forEach((button) => {
    button.addEventListener("click", () => {
      runDisplayDemo(button.dataset.runDisplay);
    });
  });
}
function currentDisplayDemo() {
  if (state.displayError) {
    return state.displayDemos.find(
      (demo) => demo.id === state.displaySelection,
    );
  }
  const statusDemo = state.displayDemos.find(
    (demo) => demo.id === state.demoStatus.demo_id,
  );
  if (statusDemo) return statusDemo;
  return null;
}

function renderDisplayStatus() {
  const demo = currentDisplayDemo();
  const panel = $("display-experience");
  const status = state.displayError
    ? {
        state: "failed",
        output: state.displayError,
      }
    : state.demoStatus;
  const running = status.state === "running";
  document.querySelectorAll("[data-run-display]").forEach((button) => {
    const isActive = button.dataset.runDisplay === demo?.id && running;
    button.disabled = running;
    button.textContent = isActive
      ? "Running on board display…"
      : "Launch on board display";
  });
  if (!demo) {
    panel.hidden = true;
    return;
  }
  const elapsed = status.started_at
    ? Math.max(0, (Date.now() - Date.parse(status.started_at)) / 1000)
    : 0;
  const duration = status.duration_seconds || demo.duration_seconds;
  const remaining = Math.max(0, Math.ceil(duration - elapsed));
  const progress = Math.min(100, (elapsed / Math.max(1, duration)) * 100);
  panel.hidden = false;
  panel.classList.toggle("running", running);
  $("display-demo-name").textContent = demo.name;
  $("display-demo-state").textContent = status.state.toUpperCase();
  $("display-demo-time").textContent = running
    ? Math.floor(remaining / 60) +
      ":" +
      String(remaining % 60).padStart(2, "0")
    : "—";
  $("display-demo-message").textContent = running
    ? "Now visible on the display connected to this EVK."
    : status.state === "completed"
      ? "The native display session completed successfully."
      : status.output || "The native display session stopped with an error.";
  $("display-demo-progress").style.width = progress + "%";
  $("stop-display-demo").disabled = !running;
}

async function runDisplayDemo(id) {
  state.displaySelection = id;
  state.displayError = "";
  const button = document.querySelector(
    '[data-run-display="' + CSS.escape(id) + '"]',
  );
  if (button) {
    button.disabled = true;
    button.textContent = "Starting on board…";
  }
  try {
    const response = await fetch("/api/v1/gpu-demo-run", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-VAR-Studio-Action": "gpu-demo",
      },
      body: JSON.stringify({ id }),
    });
    if (!response.ok) {
      throw new Error(
        (await response.text()) || "Unable to launch native demo",
      );
    }
    state.demoStatus = await response.json();
    renderDisplayStatus();
    $("display-experience").scrollIntoView({
      behavior: "smooth",
      block: "center",
    });
  } catch (error) {
    state.displayError = error.message;
    if (button) {
      button.disabled = false;
      button.textContent = "Launch on board display";
    }
    renderDisplayStatus();
  }
}

async function stopDisplayDemo() {
  const response = await fetch("/api/v1/gpu-demo-stop", {
    method: "POST",
    headers: {
      "X-VAR-Studio-Action": "gpu-demo",
    },
  });
  if (!response.ok) {
    throw new Error((await response.text()) || "Unable to stop native demo");
  }
  $("display-demo-state").textContent = "STOPPING";
  $("stop-display-demo").disabled = true;
}
function renderCameras(inventory) {
  state.cameras = (inventory.devices || []).filter((device) => device.capture);
  const select = $("camera-device");
  if (!state.cameras.length) {
    select.innerHTML = "<option>No capture devices available</option>";
    select.disabled = true;
    $("open-camera").disabled = true;
    return;
  }
  select.innerHTML = state.cameras
    .map(
      (device) =>
        '<option value="' +
        escapeHTML(device.path) +
        '">' +
        escapeHTML(device.name || device.path) +
        " \xB7 " +
        escapeHTML(device.path) +
        "</option>",
    )
    .join("");
  select.disabled = false;
  $("open-camera").disabled = false;
}
async function loadCameras() {
  try {
    const response = await fetch("/api/v1/cameras", {
      cache: "no-store",
    });
    if (!response.ok) throw new Error("Camera service unavailable");
    renderCameras(await response.json());
  } catch (error) {
    $("camera-device").innerHTML =
      "<option>" + escapeHTML(error.message) + "</option>";
    $("open-camera").disabled = true;
  }
}
function openCamera() {
  const path = $("camera-device").value;
  if (!path) return;
  state.activeDevice = path;
  $("demo-camera-stream").src =
    "/api/v1/camera-stream?device=" +
    encodeURIComponent(path) +
    "&t=" +
    Date.now() +
    "";
  $("demo-preview").classList.remove("empty");
  $("preview-message").hidden = true;
  $("camera-live").textContent = "LIVE";
  $("camera-live").classList.add("live");
  $("capture-frame").disabled = false;
}
function captureFrame() {
  const image = $("demo-camera-stream"),
    canvas = $("capture-canvas");
  if (!image.naturalWidth) return;
  canvas.width = image.naturalWidth;
  canvas.height = image.naturalHeight;
  canvas.getContext("2d").drawImage(image, 0, 0);
  canvas.toBlob(
    (blob) => {
      state.captureBlob = blob;
      $("capture-result").hidden = false;
      $("capture-meta").textContent =
        "" +
        canvas.width +
        " \xD7 " +
        canvas.height +
        " \xB7 " +
        new Date().toLocaleString() +
        "";
    },
    "image/jpeg",
    0.92,
  );
}
function captureName() {
  const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
  return "var-studio-" + timestamp + ".jpg";
}
function downloadCapture() {
  if (!state.captureBlob) return;
  const link = document.createElement("a");
  link.href = URL.createObjectURL(state.captureBlob);
  link.download = captureName();
  link.click();
  setTimeout(() => URL.revokeObjectURL(link.href), 1000);
}
async function shareCapture() {
  if (!state.captureBlob) return;
  const file = new File([state.captureBlob], captureName(), {
    type: "image/jpeg",
  });
  if (
    navigator.canShare?.({
      files: [file],
    })
  ) {
    await navigator.share({
      title: "VAR-Studio camera capture",
      text: "Camera frame captured locally with VAR-Studio.",
      files: [file],
    });
    return;
  }
  downloadCapture();
  window.location.href =
    "mailto:?subject=VAR-Studio%20camera%20capture&bod" +
    "y=The%20capture%20was%20downloaded%20locally.%20" +
    "Please%20attach%20it%20to%20this%20message.";
}
function renderMLStatus() {
  const demo = state.mlDemo,
    status = state.demoStatus,
    button = document.getElementById("run-ml"),
    badge = document.getElementById("ml-state"),
    countdown = document.getElementById("ml-countdown"),
    summary = document.getElementById("ml-result-summary"),
    output = document.getElementById("ml-output");
  if (!demo) return;
  const relevant = status.demo_id === demo.id;
  if (!relevant) {
    badge.textContent = "READY";
    button.disabled = false;
    countdown.textContent = demo.duration_seconds + " second controlled run";
    summary.textContent =
      "Ready to classify the packaged reference image o" + "n the NPU.";
    output.textContent = "No inference run yet.";
    return;
  }
  const running = status.state === "running";
  button.disabled = running;
  badge.textContent = running ? "RUNNING" : status.state.toUpperCase();
  badge.classList.toggle("live", running);
  if (running) {
    const elapsed = Math.max(
        0,
        (Date.now() - Date.parse(status.started_at)) / 1000,
      ),
      remaining = Math.max(0, Math.ceil(status.duration_seconds - elapsed));
    countdown.textContent = remaining + "s remaining";
    summary.textContent =
      "TensorFlow Lite is executing through the Vivante" + " VX delegate.";
  } else {
    countdown.textContent =
      status.state === "completed" ? "Run completed" : "Run failed";
    const average = String(status.output || "").match(
      /average time:[ ]*([0-9.]+)[ ]*ms/i,
    );
    summary.textContent =
      status.state === "completed"
        ? average
          ? "NPU inference completed · " + average[1] + " ms average"
          : "NPU inference completed successfully."
        : "The controlled inference workload returned an er" + "ror.";
  }
  output.textContent = status.output || "Waiting for runner output…";
}
async function refreshDemoStatus() {
  try {
    const response = await fetch("/api/v1/gpu-demo-status", {
      cache: "no-store",
    });
    if (!response.ok) throw new Error("Runner status unavailable");
    state.demoStatus = await response.json();
    if (state.mlDemo) renderMLStatus();
    renderDisplayStatus();
  } catch (error) {
    if (state.mlDemo) {
      $("ml-result-summary").textContent = error.message;
    }
  }
}
async function runML() {
  if (!state.mlDemo) return;
  const button = document.getElementById("run-ml");
  button.disabled = true;
  try {
    const response = await fetch("/api/v1/gpu-demo-run", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-VAR-Studio-Action": "gpu-demo",
      },
      body: JSON.stringify({
        id: state.mlDemo.id,
      }),
    });
    if (!response.ok)
      throw new Error((await response.text()) || "Unable to start inference");
    state.demoStatus = await response.json();
    renderMLStatus();
  } catch (error) {
    button.disabled = false;
    document.getElementById("ml-state").textContent = "FAILED";
    document.getElementById("ml-result-summary").textContent = error.message;
  }
}
async function load() {
  try {
    const responses = await Promise.all([
      fetch("/api/v1/diagnostics", {
        cache: "no-store",
      }),
      fetch("/api/v1/gpu-demos", {
        cache: "no-store",
      }),
    ]);
    if (!responses[0].ok)
      throw new Error("Diagnostics HTTP " + responses[0].status);
    state.diagnostics = await responses[0].json();
    state.demos = responses[1].ok ? await responses[1].json() : [];
    state.mlDemo =
      state.demos.find((item) => item.kind === "ml" && item.installed) || null;
    state.displayDemos = state.demos.filter((item) =>
      item.kind.startsWith("display-"),
    );
    document.getElementById("connection").textContent = "Live";
    renderCatalog();
    await refreshDemoStatus();
  } catch (error) {
    document.getElementById("connection").textContent = "Unavailable";
    document.getElementById("demo-catalog").innerHTML =
      '<p class="demo-loading">Capability detection fai' +
      "led: " +
      escapeHTML(error.message) +
      "</p>";
  }
}
document.getElementById("run-ml").addEventListener("click", runML);
setInterval(refreshDemoStatus, 1000);
$("stop-display-demo").addEventListener("click", () => {
  stopDisplayDemo().catch((error) => {
    $("display-demo-message").textContent = error.message;
  });
});
document.getElementById("open-camera").addEventListener("click", openCamera);
$("capture-frame").addEventListener("click", captureFrame);
$("download-capture").addEventListener("click", downloadCapture);
$("share-capture").addEventListener("click", () =>
  shareCapture().catch(() => {}),
);
window.addEventListener("beforeunload", () => {
  $("demo-camera-stream").removeAttribute("src");
});
updateClock();
setInterval(updateClock, 1000);
load();
