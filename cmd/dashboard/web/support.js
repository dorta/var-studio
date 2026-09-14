const $ = (id) => document.getElementById(id);
let preview = null,
  section = "summary",
  kernelTimer = null;
const escapeHTML = (value) =>
  String(value ?? "").replace(
    /[&<>"']/g,
    (char) =>
      ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;",
      })[char],
  );
const formatBytes = (bytes) => {
  const units = ["B", "KiB", "MiB", "GiB"];
  let value = Number(bytes) || 0,
    index = 0;
  while (value >= 1024 && index < 3) {
    value /= 1024;
    index++;
  }
  return "" + value.toFixed(index ? 1 : 0) + " " + units[index] + "";
};
const duration = (seconds) => {
  const d = Math.floor(seconds / 86400),
    h = Math.floor((seconds % 86400) / 3600),
    m = Math.floor((seconds % 3600) / 60);
  return [d && "" + d + "d", h && "" + h + "h", "" + m + "m"]
    .filter(Boolean)
    .join(" ");
};
const card = (label, value, detail = "") =>
  '<article class="support-card"><span>' +
  escapeHTML(label) +
  "</span><strong>" +
  escapeHTML(value) +
  "</strong>" +
  (detail ? "<small>" + escapeHTML(detail) + "</small>" : "") +
  "</article>";
function renderSummary() {
  const s = preview.snapshot,
    m = preview.metrics,
    b = s.board || {},
    memory = s.metrics.memory_total_bytes
      ? (100 * s.metrics.memory_used_bytes) / s.metrics.memory_total_bytes
      : 0;
  $("support-body").innerHTML =
    '<div class="support-summary">\n    ' +
    card(
      "Detected product",
      b.name || s.system.model,
      b.soc || "Device Tree detection",
    ) +
    "\n    " +
    card("BSP", s.bsp.distro_version || s.system.os_version, s.bsp.distro) +
    "\n    " +
    card("Kernel", s.system.kernel, s.system.architecture) +
    "\n    " +
    card(
      "Current CPU",
      "" + s.metrics.cpu_percent.toFixed(1) + "%",
      "" + s.system.cores + " logical cores",
    ) +
    "\n    " +
    card(
      "Current memory",
      "" + memory.toFixed(1) + "%",
      "" +
        formatBytes(s.metrics.memory_used_bytes) +
        " of " +
        formatBytes(s.metrics.memory_total_bytes) +
        "",
    ) +
    "\n    " +
    card(
      "Report coverage",
      "" + Object.keys(preview.diagnostics).length + " diagnostic files",
      "" + m.samples.length + " stored metric samples",
    ) +
    '\n  </div><div class="support-note"><strong>Ready' +
    " to review.</strong> This preview uses the same " +
    "redaction rules as the downloadable ZIP. Nothing" +
    " is uploaded automatically.</div>";
}
function renderInventory() {
  $("support-body").innerHTML =
    '<pre class="support-code">' +
    escapeHTML(JSON.stringify(preview.snapshot, null, 2)) +
    "</pre>";
}
function renderMetrics() {
  $("support-body").innerHTML =
    '<div class="support-summary">' +
    card("Started", new Date(preview.metrics.started_at).toLocaleString()) +
    "" +
    card(
      "Sample interval",
      "" + preview.metrics.sample_interval_seconds + " seconds",
    ) +
    "" +
    card("Stored samples", preview.metrics.samples.length) +
    '</div><pre class="support-code" style="margin-to' +
    'p:16px">' +
    escapeHTML(JSON.stringify(preview.metrics, null, 2)) +
    "</pre>";
}
function renderFiles() {
  const names = Object.keys(preview.diagnostics).sort();
  $("support-body").innerHTML =
    '<div class="file-picker"><label for="diagnostic-' +
    'file">Report file</label><select id="diagnostic-' +
    'file">' +
    names.map((n) => "<option>" + escapeHTML(n) + "</option>").join("") +
    '</select></div><pre id="file-content" class="sup' +
    'port-code"></pre>';
  const select = $("diagnostic-file"),
    show = () => {
      $("file-content").textContent = preview.diagnostics[select.value] || "";
    };
  select.addEventListener("change", show);
  show();
}
function renderPrivacy() {
  $("support-body").innerHTML =
    '<div class="privacy-copy"><p>The report is gener' +
    "ated locally on this board and downloaded direct" +
    "ly by your browser. VAR-Studio never uploads it a" +
    "utomatically.</p><h3>Included</h3><ul><li>Detect" +
    "ed product, BSP and hardware inventory</li><li>P" +
    "erformance history collected since boot</li><li>" +
    "Selected read-only proc/sys diagnostics</li><li>" +
    "Persistent kernel messages and pstore crash reco" +
    "rds when available</li></ul><h3>Excluded or reda" +
    "cted</h3><ul><li>Passwords, tokens, API keys, se" +
    "rial numbers and MAC addresses</li><li>Environme" +
    "nt variables, private keys and user home directo" +
    "ries</li><li>Application files, Docker socket co" +
    "ntents and full process arguments</li></ul><h3>B" +
    "efore sharing</h3><p>Review the Inventory, Kerne" +
    "l live and Diagnostic files sections. The downlo" +
    "adable ZIP contains the same categories, with lo" +
    "nger log retention.</p></div>";
}
async function loadKernel() {
  try {
    const response = await fetch("/api/v1/kernel-log", {
        cache: "no-store",
      }),
      data = await response.json(),
      root = $("kernel-log");
    if (!root || section !== "kernel") return;
    $("kernel-updated").textContent =
      "Updated " +
      new Date(data.updated_at).toLocaleTimeString() +
      " \xB7 " +
      data.messages.length +
      " messages";
    if (!data.available) {
      root.innerHTML =
        '<div class="support-empty">Kernel log collection' +
        " is unavailable on this image.</div>";
      return;
    }
    root.innerHTML =
      data.messages
        .map(
          (item) =>
            '<div class="kernel-line"><time>+' +
            item.uptime_seconds.toFixed(3) +
            's</time><span class="kernel-level ' +
            item.level +
            '">' +
            item.level +
            "</span><code>" +
            escapeHTML(item.message) +
            "</code></div>",
        )
        .join("") ||
      '<div class="support-empty">No kernel messages ca' + "ptured yet.</div>";
    if ($("auto-scroll")?.checked) root.scrollTop = root.scrollHeight;
  } catch (error) {
    console.error(error);
  }
}
function renderKernel() {
  $("support-body").innerHTML =
    '<div class="kernel-tools"><div><span class="live' +
    '-dot"></span><strong>Redacted kernel messages</s' +
    "trong><span>refresh every 2 seconds</span></div>" +
    '<label><input id="auto-scroll" type="checkbox" c' +
    'hecked> Follow latest</label></div><div id="kern' +
    'el-log" class="kernel-log"><div class="support-e' +
    'mpty">Reading kernel messages\u2026</div></div><small' +
    ' id="kernel-updated" class="panel-subtitle"></sm' +
    "all>";
  loadKernel();
  kernelTimer = setInterval(loadKernel, 2000);
}
const sections = {
  summary: ["REPORT OVERVIEW", "Summary", renderSummary],
  inventory: ["BOARD & BSP", "Inventory", renderInventory],
  metrics: ["PERFORMANCE", "Metrics history", renderMetrics],
  kernel: ["LIVE DIAGNOSTICS", "Kernel live", renderKernel],
  files: ["REPORT CONTENTS", "Diagnostic files", renderFiles],
  privacy: ["DATA HANDLING", "Privacy", renderPrivacy],
};
function selectSection(next) {
  section = next;
  clearInterval(kernelTimer);
  kernelTimer = null;
  document
    .querySelectorAll("[data-section]")
    .forEach((button) =>
      button.classList.toggle("active", button.dataset.section === next),
    );
  const [kicker, title, render] = sections[next];
  $("section-kicker").textContent = kicker;
  $("section-title").textContent = title;
  render();
}
async function load() {
  $("connection").textContent = "Collecting";
  try {
    const response = await fetch("/api/v1/support-preview", {
      cache: "no-store",
    });
    preview = await response.json();
    $("updated-at").textContent =
      "Generated " + new Date(preview.generated_at).toLocaleString() + "";
    $("connection").textContent = "Live";
    selectSection(section);
  } catch (error) {
    $("connection").textContent = "Unavailable";
    $("support-body").innerHTML =
      '<div class="support-empty">Unable to collect the' +
      " support preview.</div>";
    console.error(error);
  }
}
document
  .querySelectorAll("[data-section]")
  .forEach((button) =>
    button.addEventListener(
      "click",
      () => preview && selectSection(button.dataset.section),
    ),
  );
$("refresh").addEventListener("click", load);
setInterval(() => {
  $("clock").textContent = new Date().toLocaleTimeString();
}, 1000);
load();
