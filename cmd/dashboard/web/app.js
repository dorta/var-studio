const $ = (id) => document.getElementById(id);
const liveHistory = Array(28).fill(0);
let metricHistory = null;
let gpuHistory = null;
let npuHistory = null;
let gpuDemos = [];
let selectedRange = "hour";
let pendingGPUDemo = null;
let activeGPUDemoStatus = null;
const chartColors = [
  "#ff5f46",
  "#00b368",
  "#5f7ea8",
  "#e0ad00",
  "#9b59b6",
  "#00a6a6",
  "#d35400",
  "#68737d",
];
const formatBytes = (value = 0) => {
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let n = Number(value),
    i = 0;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i++;
  }
  return (
    "" +
    (n >= 10 || i === 0 ? n.toFixed(0) : n.toFixed(1)) +
    " " +
    units[i] +
    ""
  );
};
const percent = (used, total) =>
  total ? Math.min(100, (used / total) * 100) : 0;
const duration = (seconds = 0) => {
  const days = Math.floor(seconds / 86400),
    hours = Math.floor((seconds % 86400) / 3600),
    mins = Math.floor((seconds % 3600) / 60);
  return days
    ? "" + days + "d " + hours + "h uptime"
    : "" + hours + "h " + mins + "min uptime";
};
const setText = (id, value) => {
  $(id).textContent = value ?? "—";
};
const node = (tag, className, text) => {
  const el = document.createElement(tag);
  if (className) el.className = className;
  if (text !== undefined) el.textContent = text;
  return el;
};
function render(snapshot) {
  const { system, board, bsp, metrics, thermals, networks } = snapshot;
  setText("hostname", board?.name || system.hostname);
  setText("model", system.model);
  setText("arch", system.architecture.toUpperCase());
  setText("distro", bsp.distro_version || system.os_version);
  setText("uptime", duration(system.uptime_seconds));
  setText("cpu", "" + metrics.cpu_percent.toFixed(0) + "%");
  setText(
    "load",
    "Load " + metrics.load.map((v) => v.toFixed(2)).join(" · ") + "",
  );
  const memoryPct = percent(
    metrics.memory_used_bytes,
    metrics.memory_total_bytes,
  );
  setText("memory", "" + memoryPct.toFixed(0) + "%");
  setText(
    "memory-detail",
    "" +
      formatBytes(metrics.memory_used_bytes) +
      " of " +
      formatBytes(metrics.memory_total_bytes) +
      "",
  );
  const storagePct = percent(
    metrics.storage_used_bytes,
    metrics.storage_total_bytes,
  );
  setText("storage", "" + storagePct.toFixed(0) + "%");
  setText(
    "storage-detail",
    "" +
      formatBytes(metrics.storage_used_bytes) +
      " of " +
      formatBytes(metrics.storage_total_bytes) +
      "",
  );
  $("storage-bar").style.width = "" + storagePct + "%";
  const hottestSensor = thermals.reduce(
    (hottest, item) =>
      !hottest || item.celsius > hottest.celsius ? item : hottest,
    null,
  );
  const hottest = hottestSensor?.celsius || 0;
  setText(
    "temperature",
    hottestSensor ? "" + hottest.toFixed(0) + "\xB0C" : "—",
  );
  liveHistory.push(metrics.cpu_percent);
  liveHistory.shift();
  renderSpark();
  setText("core-count", "" + metrics.per_core.length + " CORES");
  renderProduct(board, system);
  renderHealth(snapshot);
  renderActiveAddresses(networks);
  renderThermals(thermals);
  renderBuild(bsp, system);
  document.body.classList.remove("dashboard-loading");
}
function renderHealth(snapshot) {
  const { metrics, thermals, warnings = [] } = snapshot;
  const memoryPct = percent(
    metrics.memory_used_bytes,
    metrics.memory_total_bytes,
  );
  const storagePct = percent(
    metrics.storage_used_bytes,
    metrics.storage_total_bytes,
  );
  const hottest = thermals.reduce(
    (value, sensor) => Math.max(value, sensor.celsius),
    0,
  );
  const issues = [];
  if (hottest >= 90)
    issues.push("High temperature (" + hottest.toFixed(0) + "\xB0C)");
  if (memoryPct >= 95)
    issues.push("Memory pressure (" + memoryPct.toFixed(0) + "%)");
  if (storagePct >= 95)
    issues.push("Storage nearly full (" + storagePct.toFixed(0) + "%)");
  warnings.forEach((warning) => issues.push(warning));
  const critical = hottest >= 100 || memoryPct >= 99 || storagePct >= 99;
  const health = $("hero-health");
  health.classList.toggle("attention", issues.length > 0 && !critical);
  health.classList.toggle("critical", critical);
  setText(
    "health-status",
    critical ? "Critical" : issues.length ? "Attention" : "Healthy",
  );
  setText("health-reason", issues[0] || "No active diagnostic warnings");
  health.title =
    "Health checks: thermal < 90\xB0C, memory < 95%, sto" +
    "rage < 95%, and no collector warnings. " +
    (issues.length
      ? "Current issue: " + issues.join("; ") + ""
      : "All checks passed.") +
    "";
}
function renderExplainableHealth(report) {
  const hero = $("hero-health");
  const critical = report.status === "critical";
  const attention = report.status === "warning";
  hero.classList.toggle("attention", attention);
  hero.classList.toggle("critical", critical);
  setText(
    "health-status",
    critical ? "Critical" : attention ? "Attention" : "Healthy",
  );
  setText("health-reason", report.summary || "Health evaluation unavailable");
  hero.title = (report.checks || [])
    .map((check) => "" + check.name + ": " + check.summary + "")
    .join("\n");
}
function setDashboardConnection(connected) {
  const hero = $("hero-health");
  const pulse = document.querySelector(".pulse");
  hero.classList.toggle("offline", !connected);
  setText("connection", connected ? "Live updates" : "Disconnected");
  setText("health-connection", connected ? "ONLINE" : "OFFLINE");
  pulse.style.background = connected ? "var(--green)" : "var(--danger)";
  if (!connected) {
    setText("health-status", "Connection lost");
    setText("health-reason", "Showing the last data received");
    hero.title = "Live diagnostics are unavailable until reconnection.";
  }
}
function renderProduct(board, system) {
  const actions = $("product-actions");
  actions.replaceChildren();
  const meta = $("product-meta");
  meta.replaceChildren();
  const image = $("product-image");
  if (!board?.name) {
    image.classList.add("hidden");
    return;
  }
  if (board.image_url) {
    image.src = board.image_url;
    image.alt = "" + board.name + " system on module";
    image.classList.remove("hidden");
  }
  const carrier = board.carrier || (board.carriers || []).join(", ");
  [
    [board.soc || "SoC unavailable", "SoC"],
    [carrier || "Carrier unavailable", "Carrier board"],
    [
      board.confidence === "exact"
        ? "Exact Device Tree match"
        : "Compatible Device Tree match",
      "Detection",
    ],
  ].forEach(([value, title]) => {
    const item = node("span", "", value);
    item.title = title;
    meta.append(item);
  });
  setText("product-modal-title", board.name);
  setText(
    "product-modal-description",
    board.description || "Product information is available from Variscite.",
  );
  const modalFacts = $("product-modal-facts");
  modalFacts.replaceChildren();
  [
    ["SoC", board.soc],
    ["Carrier board", carrier],
    ["Device Tree model", system.model],
    ["Detection source", board.detection_source],
  ].forEach(([label, value]) => {
    const row = node("div");
    row.append(node("dt", "", label), node("dd", "", value || "—"));
    modalFacts.append(row);
  });
  const about = node("button", "product-about", "About this product");
  about.type = "button";
  about.addEventListener("click", () => $("product-modal").showModal());
  actions.append(about);
  const links = [
    ["Documentation", board.documentation_url],
    ["Specifications", board.specifications_url],
    ["Quick start", board.quick_start_url],
    ["Buy evaluation kit", board.shop_url],
  ];
  links.forEach(([label, url], index) => {
    if (!url) return;
    const link = node(
      "a",
      index === 0 ? "primary" : "",
      "" + label + " \u2197",
    );
    link.href = url;
    link.target = "_blank";
    link.rel = "noopener noreferrer";
    actions.append(link);
  });
}
function renderSpark() {
  const root = $("cpu-spark");
  root.replaceChildren();
  liveHistory.forEach((value) => {
    const bar = node("i");
    bar.style.height = "" + Math.max(2, value) + "%";
    root.append(bar);
  });
}
function svgNode(tag, attributes = {}) {
  const element = document.createElementNS("http://www.w3.org/2000/svg", tag);
  Object.entries(attributes).forEach(([key, value]) =>
    element.setAttribute(key, value),
  );
  return element;
}
function chartPoints(
  values,
  width,
  height,
  left = 42,
  top = 12,
  right = 14,
  bottom = 27,
) {
  const plotWidth = width - left - right,
    plotHeight = height - top - bottom;
  return values.map((value, index) => ({
    x:
      left +
      (values.length === 1 ? 0 : (index * plotWidth) / (values.length - 1)),
    y: top + ((100 - Math.max(0, Math.min(100, value))) * plotHeight) / 100,
  }));
}
function renderHistory(report) {
  metricHistory = report;
  const samples = report.samples || [];
  if (!samples.length) return;
  liveHistory.splice(
    0,
    liveHistory.length,
    ...samples.slice(-28).map((sample) => sample.cpu_percent),
  );
  while (liveHistory.length < 28) liveHistory.unshift(0);
  renderSpark();
  renderCPUHistory(samples);
  renderMemoryHistory(samples);
  const started = new Date(report.started_at);
  const rangeLabels = {
    hour: "Last hour",
    day: "Last 24 hours",
    week: "Last 7 days",
    month: "Last 30 days",
    boot: Number.isNaN(started.getTime())
      ? "Since boot"
      : "Since " + started.toLocaleString("en-US") + "",
  };
  const label =
    "" +
    rangeLabels[selectedRange] +
    " \xB7 " +
    samples.length +
    " plotted points \xB7 " +
    report.sample_interval_seconds +
    "s samples";
  setText("history-range", label);
  setText("memory-history-range", label);
}
function renderCPUHistory(samples) {
  const root = $("cpu-history");
  root.replaceChildren();
  const coreCount = samples.reduce(
    (count, sample) => Math.max(count, (sample.per_core || []).length),
    0,
  );
  setText("core-count", "" + coreCount + " CORES");
  if (samples.length < 2 || coreCount === 0) {
    root.append(node("div", "chart-empty", "Collecting the first samples…"));
    return;
  }
  const width = 1000,
    height = 260,
    svg = svgNode("svg", {
      viewBox: "0 0 " + width + " " + height + "",
      preserveAspectRatio: "none",
      role: "img",
    });
  [0, 25, 50, 75, 100].forEach((value) => {
    const y = 12 + ((100 - value) * 221) / 100;
    svg.append(
      svgNode("line", {
        x1: 42,
        y1: y,
        x2: 986,
        y2: y,
        class: "chart-grid",
      }),
    );
    const text = svgNode("text", {
      x: 34,
      y: y + 4,
      class: "chart-axis",
      "text-anchor": "end",
    });
    text.textContent = "" + value + "%";
    svg.append(text);
  });
  for (let core = 0; core < coreCount; core++) {
    const values = samples.map((sample) =>
      Number((sample.per_core || [])[core] || 0),
    );
    const points = chartPoints(values, width, height)
      .map((point) => "" + point.x.toFixed(1) + "," + point.y.toFixed(1) + "")
      .join(" ");
    svg.append(
      svgNode("polyline", {
        points,
        class: "chart-line",
        stroke: chartColors[core % chartColors.length],
      }),
    );
  }
  root.append(svg);
  const legend = $("cpu-legend");
  legend.replaceChildren();
  const latest = samples[samples.length - 1].per_core || [];
  for (let core = 0; core < coreCount; core++) {
    const item = node("span");
    const dot = node("i");
    dot.style.background = chartColors[core % chartColors.length];
    item.append(
      dot,
      document.createTextNode("CPU " + core + " "),
      node("b", "", "" + Number(latest[core] || 0).toFixed(0) + "%"),
    );
    legend.append(item);
  }
}
function renderMemoryHistory(samples) {
  const root = $("memory-history");
  root.replaceChildren();
  if (samples.length < 2) {
    root.append(node("span", "chart-empty", "Collecting history…"));
    return;
  }
  const width = 400,
    height = 60,
    points = chartPoints(
      samples.map((sample) => sample.memory_percent),
      width,
      height,
      1,
      3,
      1,
      3,
    );
  const svg = svgNode("svg", {
    viewBox: "0 0 " + width + " " + height + "",
    preserveAspectRatio: "none",
    role: "img",
  });
  const line = points
    .map((point) => "" + point.x.toFixed(1) + "," + point.y.toFixed(1) + "")
    .join(" ");
  const area =
    "M " +
    points[0].x +
    " " +
    (height - 2) +
    " L " +
    line.replaceAll(" ", " L ") +
    " L " +
    points[points.length - 1].x +
    " " +
    (height - 2) +
    " Z";
  svg.append(
    svgNode("path", {
      d: area,
      class: "memory-area",
    }),
    svgNode("polyline", {
      points: line,
      class: "memory-line",
    }),
  );
  root.append(svg);
  renderMemoryPanel(samples);
}
function renderMemoryPanel(samples) {
  const root = $("memory-history-chart");
  root.replaceChildren();
  const latest = Number(samples[samples.length - 1]?.memory_percent || 0);
  setText("memory-current", "" + latest.toFixed(0) + "% USED");
  setText("memory-history-value", "" + latest.toFixed(0) + "%");
  if (samples.length < 2) {
    root.append(node("div", "chart-empty", "Collecting the first samples…"));
    return;
  }
  const width = 1000,
    height = 260,
    values = samples.map((sample) => Number(sample.memory_percent || 0));
  const points = chartPoints(values, width, height),
    svg = svgNode("svg", {
      viewBox: "0 0 " + width + " " + height + "",
      preserveAspectRatio: "none",
      role: "img",
    });
  [0, 25, 50, 75, 100].forEach((value) => {
    const y = 12 + ((100 - value) * 221) / 100;
    svg.append(
      svgNode("line", {
        x1: 42,
        y1: y,
        x2: 986,
        y2: y,
        class: "chart-grid",
      }),
    );
    const text = svgNode("text", {
      x: 34,
      y: y + 4,
      class: "chart-axis",
      "text-anchor": "end",
    });
    text.textContent = "" + value + "%";
    svg.append(text);
  });
  const line = points
    .map((point) => "" + point.x.toFixed(1) + "," + point.y.toFixed(1) + "")
    .join(" ");
  const baseline = height - 27;
  const area =
    "M " +
    points[0].x +
    " " +
    baseline +
    " L " +
    line.replaceAll(" ", " L ") +
    " L " +
    points[points.length - 1].x +
    " " +
    baseline +
    " Z";
  svg.append(
    svgNode("path", {
      d: area,
      class: "memory-panel-area",
    }),
    svgNode("polyline", {
      points: line,
      class: "memory-panel-line",
    }),
  );
  root.append(svg);
}
function renderGPUHistory(report) {
  gpuHistory = report;
  const root = $("gpu-history-chart"),
    legend = $("gpu-legend");
  root.replaceChildren();
  legend.replaceChildren();
  if (!report.available) {
    setText("gpu-status", "UNAVAILABLE");
    setText(
      "gpu-history-range",
      "No compatible GPU metrics source was detected",
    );
    root.append(
      node(
        "div",
        "chart-empty",
        "GPU monitoring is not available on this SoM.",
      ),
    );
    return;
  }
  const samples = report.samples || [],
    started = new Date(report.started_at);
  const rangeLabels = {
    hour: "Last hour",
    day: "Last 24 hours",
    week: "Last 7 days",
    month: "Last 30 days",
    boot: Number.isNaN(started.getTime())
      ? "Since boot"
      : "Since " + started.toLocaleString("en-US") + "",
  };
  setText("gpu-status", "GPUTOP LIVE");
  setText(
    "gpu-history-range",
    "" +
      rangeLabels[selectedRange] +
      " \xB7 " +
      samples.length +
      " plotted points \xB7 " +
      report.sample_interval_seconds +
      "s samples",
  );
  if (samples.length < 2) {
    root.append(
      node("div", "chart-empty", "Collecting the first GPU samples…"),
    );
    return;
  }
  const width = 1000,
    height = 260,
    svg = svgNode("svg", {
      viewBox: "0 0 " + width + " " + height + "",
      preserveAspectRatio: "none",
      role: "img",
    });
  [0, 25, 50, 75, 100].forEach((value) => {
    const y = 12 + ((100 - value) * 221) / 100;
    svg.append(
      svgNode("line", {
        x1: 42,
        y1: y,
        x2: 986,
        y2: y,
        class: "chart-grid",
      }),
    );
    const text = svgNode("text", {
      x: 34,
      y: y + 4,
      class: "chart-axis",
      "text-anchor": "end",
    });
    text.textContent = "" + value + "%";
    svg.append(text);
  });
  const series = [
    [
      "GPU usage",
      samples.map((sample) => Number(sample.usage_percent || 0)),
      "#ff5f46",
    ],
    [
      "Memory controller",
      samples.map((sample) => Number(sample.memory_controller_percent || 0)),
      "#5f7ea8",
    ],
  ];
  series.forEach(([label, values, color]) => {
    const points = chartPoints(values, width, height)
      .map((point) => "" + point.x.toFixed(1) + "," + point.y.toFixed(1) + "")
      .join(" ");
    svg.append(
      svgNode("polyline", {
        points,
        class: "chart-line",
        stroke: color,
      }),
    );
    const item = node("span"),
      dot = node("i");
    dot.style.background = color;
    item.append(
      dot,
      document.createTextNode("" + label + " "),
      node("b", "", "" + values[values.length - 1].toFixed(0) + "%"),
    );
    legend.append(item);
  });
  root.append(svg);
}
function formatClock(clockHz = 0) {
  const value = Number(clockHz);
  if (!value) return "Clock unavailable";
  return value >= 1e9
    ? (value / 1e9).toFixed(value % 1e9 ? 2 : 0) + " GHz"
    : Math.round(value / 1e6) + " MHz";
}
function renderNPUHistory(report) {
  npuHistory = report;
  const panel = $("npu-panel");
  if (!report.available) {
    panel.classList.remove("hidden");
    setText("npu-model", "NOT AVAILABLE");
    setText("npu-clock", "NO NPU");
    setText(
      "npu-history-range",
      report.reason || "No compatible neural accelerator was detected",
    );
    const root = $("npu-history-chart"),
      legend = $("npu-legend");
    root.replaceChildren(
      node(
        "div",
        "chart-empty",
        "NPU acceleration is not available on this SoM.",
      ),
    );
    legend.replaceChildren();
    return;
  }
  panel.classList.remove("hidden");
  const root = $("npu-history-chart"),
    legend = $("npu-legend");
  root.replaceChildren();
  legend.replaceChildren();
  const samples = report.samples || [],
    device = report.device || {};
  const latest = samples[samples.length - 1] || {};
  const started = new Date(report.started_at);
  const rangeLabels = {
    hour: "Last hour",
    day: "Last 24 hours",
    week: "Last 7 days",
    month: "Last 30 days",
    boot: Number.isNaN(started.getTime())
      ? "Since boot"
      : "Since " + started.toLocaleString("en-US"),
  };
  setText("npu-model", (device.model || "NPU") + " · CORE " + device.core_id);
  setText("npu-clock", formatClock(latest.clock_hz));
  setText(
    "npu-history-range",
    rangeLabels[selectedRange] +
      " · " +
      samples.length +
      " plotted points · " +
      report.sample_interval_seconds +
      "s samples",
  );
  if (samples.length < 2) {
    root.append(
      node(
        "div",
        "chart-empty",
        "NPU detected \xB7 collecting the first utilization " + "samples\u2026",
      ),
    );
    return;
  }
  const width = 1000,
    height = 260;
  const values = samples.map((sample) => Number(sample.usage_percent || 0));
  const points = chartPoints(values, width, height);
  const svg = svgNode("svg", {
    viewBox: "0 0 " + width + " " + height,
    preserveAspectRatio: "none",
    role: "img",
  });
  [0, 25, 50, 75, 100].forEach((value) => {
    const y = 12 + ((100 - value) * 221) / 100;
    svg.append(
      svgNode("line", {
        x1: 42,
        y1: y,
        x2: 986,
        y2: y,
        class: "chart-grid",
      }),
    );
    const text = svgNode("text", {
      x: 34,
      y: y + 4,
      class: "chart-axis",
      "text-anchor": "end",
    });
    text.textContent = value + "%";
    svg.append(text);
  });
  const line = points
    .map((point) => point.x.toFixed(1) + "," + point.y.toFixed(1))
    .join(" ");
  const baseline = height - 27;
  const area =
    "M " +
    points[0].x +
    " " +
    baseline +
    " L " +
    line.replaceAll(" ", " L ") +
    " L " +
    points[points.length - 1].x +
    " " +
    baseline +
    " Z";
  svg.append(
    svgNode("path", {
      d: area,
      class: "npu-area",
    }),
    svgNode("polyline", {
      points: line,
      class: "chart-line npu-line",
    }),
  );
  root.append(svg);
  const item = node("span"),
    dot = node("i");
  dot.style.background = "#00b875";
  item.append(
    dot,
    document.createTextNode("NPU utilization "),
    node("b", "", values[values.length - 1].toFixed(0) + "%"),
  );
  const active =
    Boolean(latest.clock_enabled) || Number(latest.usage_percent || 0) > 0;
  const state = node(
    "span",
    "accelerator-state " + (active ? "active" : ""),
    active ? "ACCELERATOR ACTIVE" : "IDLE · READY FOR INFERENCE",
  );
  legend.append(item, state);
}
function renderActiveAddresses(networks) {
  const root = $("active-address-list");
  root.replaceChildren();
  const activeInterfaces = (networks || []).filter((item) => {
    const hasIPv4 = (item.addresses || []).some((address) =>
      address.includes("."),
    );
    return item.state === "up" && item.name !== "lo" && hasIPv4;
  });
  const physical = activeInterfaces.filter(
    (item) => !/^(docker|br-|veth|virbr|tailscale)/.test(item.name),
  );
  const visible = physical.length ? physical : activeInterfaces;
  const active = visible.flatMap((item) => {
    return (item.addresses || [])
      .filter((address) => address.includes("."))
      .map((address) => ({ name: item.name, address }));
  });
  if (!active.length) {
    root.append(node("span", "address-unavailable", "No active IPv4 address"));
    return;
  }
  active.forEach((item) => {
    const address = item.address.split("/")[0];
    const link = node("a", "active-address");
    link.href = "http://" + address + ":9090/";
    link.title = "Open VAR-Studio through " + item.name;
    link.append(
      node("strong", "", address),
      node("small", "", item.name),
    );
    root.append(link);
  });
}
function renderThermals(thermals) {
  const root = $("thermal-summary");
  root.replaceChildren();
  if (!thermals.length) {
    root.append(node("span", "thermal-unavailable", "No sensors exposed"));
    return;
  }
  thermals.forEach((item) => {
    const row = node("div", "thermal-summary-row"),
      icon = node("img");
    icon.src = "/icons/thermometer.svg";
    icon.alt = "";
    row.append(
      icon,
      node("span", "", item.name),
      node("strong", "", "" + item.celsius.toFixed(1) + "\xB0C"),
    );
    root.append(row);
  });
}
function renderBuild(bsp, system) {
  if (!$("layers")) return;
  setText("bsp-distro", bsp.distro || "—");
  setText("bsp-version", bsp.distro_version || "—");
  setText("kernel", system.kernel);
  setText("layers-count", bsp.layers.length);
  const root = $("layers");
  root.replaceChildren();
  bsp.layers.forEach((layer) => {
    const row = node("tr"),
      nameCell = node("td"),
      revisionCell = node("td");
    nameCell.append(
      layerLink(layer.name, layer.repository_url, "Open repository"),
    );
    revisionCell.append(
      layerLink(layer.revision, layer.revision_url, "Open this commit"),
    );
    const status = node(
      "td",
      layer.modified ? "modified" : "",
      layer.modified ? "Modified" : "Clean",
    );
    row.append(nameCell, revisionCell, status);
    root.append(row);
  });
}
function layerLink(text, url, title) {
  if (!url) return document.createTextNode(text);
  const link = node("a", "layer-link");
  link.href = url;
  link.target = "_blank";
  link.rel = "noopener noreferrer";
  link.title = title;
  link.append(document.createTextNode(text), node("span", "", "↗"));
  return link;
}
async function refresh() {
  try {
    const [snapshotResponse, healthResponse] = await Promise.all([
      fetch("/api/v1/snapshot", {
        cache: "no-store",
      }),
      fetch("/api/v1/health", {
        cache: "no-store",
      }),
    ]);
    if (!snapshotResponse.ok)
      throw new Error("HTTP " + snapshotResponse.status + "");
    render(await snapshotResponse.json());
    if (healthResponse.ok) renderExplainableHealth(await healthResponse.json());
    setDashboardConnection(true);
  } catch (error) {
    setDashboardConnection(false);
    console.error(error);
  }
}
async function refreshHistory() {
  try {
    const response = await fetch(
      "/api/v1/history?range=" + encodeURIComponent(selectedRange) + "",
      {
        cache: "no-store",
      },
    );
    if (!response.ok) throw new Error("HTTP " + response.status + "");
    renderHistory(await response.json());
  } catch (error) {
    console.error("Unable to load metrics history", error);
  }
}
async function refreshGPUHistory() {
  try {
    const response = await fetch(
      "/api/v1/gpu-history?range=" + encodeURIComponent(selectedRange) + "",
      {
        cache: "no-store",
      },
    );
    if (!response.ok) throw new Error("HTTP " + response.status + "");
    renderGPUHistory(await response.json());
  } catch (error) {
    console.error("Unable to load GPU history", error);
  }
}
async function refreshNPUHistory() {
  try {
    const response = await fetch(
      "/api/v1/npu-history?range=" + encodeURIComponent(selectedRange),
      {
        cache: "no-store",
      },
    );
    if (!response.ok) throw new Error("HTTP " + response.status);
    renderNPUHistory(await response.json());
  } catch (error) {
    renderNPUHistory({
      available: false,
      reason: "NPU collector is unavailable",
      samples: [],
    });
    console.error("Unable to load NPU history", error);
  }
}
function renderGPUDemos(
  status = {
    state: "idle",
  },
) {
  const root = $("gpu-demo-list");
  root.replaceChildren();
  if (!gpuDemos.length) {
    root.append(node("span", "chart-empty", "GPU demo runner is unavailable."));
    setText("gpu-demo-state", "UNAVAILABLE");
    return;
  }
  const running = status.state === "running";
  setText(
    "gpu-demo-state",
    running
      ? "RUNNING \xB7 " + status.demo_name + ""
      : status.state === "completed"
        ? "LAST RUN COMPLETED"
        : status.state === "failed"
          ? "LAST RUN FAILED"
          : "READY",
  );
  activeGPUDemoStatus = running ? status : null;
  $("gpu-demo-progress").classList.toggle("hidden", !running);
  if (running) {
    setText("gpu-demo-progress-name", status.demo_name);
    updateGPUDemoProgress();
  }
  gpuDemos.forEach((demo) => {
    const card = node("article", "gpu-demo-card"),
      head = node("header"),
      title = node("h4", "", demo.name);
    head.append(
      title,
      node("span", "", "" + demo.api + " \xB7 " + demo.duration_seconds + "s"),
    );
    const button = node(
      "button",
      "",
      !demo.installed
        ? "Not installed"
        : running && status.demo_id === demo.id
          ? "Running…"
          : "Run demo",
    );
    button.type = "button";
    button.disabled = running || !demo.installed;
    button.addEventListener("click", () => runGPUDemo(demo));
    card.append(head, node("p", "", demo.description), button);
    root.append(card);
  });
  const result = $("gpu-demo-result");
  if (!running && status.output) {
    $("gpu-demo-output").textContent = status.output;
    result.classList.toggle("failed", status.state === "failed");
    setText(
      "gpu-demo-result-title",
      status.state === "failed"
        ? "" + status.demo_name + " failed"
        : "" + status.demo_name + " completed",
    );
    const finished = status.finished_at
      ? new Date(status.finished_at).toLocaleString("en-US")
      : "";
    setText(
      "gpu-demo-result-meta",
      "" +
        finished +
        "" +
        (status.state === "failed"
          ? " \xB7 exit code " + status.exit_code + ""
          : "") +
        "",
    );
    result.classList.remove("hidden");
  } else if (running) {
    result.classList.add("hidden");
  }
}
function updateGPUDemoProgress() {
  const status = activeGPUDemoStatus;
  if (!status) return;
  const catalogDemo = gpuDemos.find((demo) => demo.id === status.demo_id);
  const durationSeconds = Number(
    status.duration_seconds || catalogDemo?.duration_seconds || 0,
  );
  const started = Date.parse(status.started_at);
  if (!durationSeconds || Number.isNaN(started)) {
    setText("gpu-demo-countdown", "—");
    setText("gpu-demo-countdown-label", "running");
    $("gpu-demo-progress-bar").style.width = "0%";
    return;
  }
  const elapsedSeconds = Math.max(0, (Date.now() - started) / 1000);
  const remainingSeconds = Math.max(
    0,
    Math.ceil(durationSeconds - elapsedSeconds),
  );
  const minutes = Math.floor(remainingSeconds / 60);
  const seconds = String(remainingSeconds % 60).padStart(2, "0");
  setText("gpu-demo-countdown", "" + minutes + ":" + seconds + "");
  setText(
    "gpu-demo-countdown-label",
    remainingSeconds ? "remaining" : "finishing…",
  );
  setText(
    "gpu-demo-progress-detail",
    "" +
      (catalogDemo?.api || "GPU") +
      " workload \xB7 monitoring live performance",
  );
  $("gpu-demo-progress-bar").style.width =
    "" + Math.min(100, (elapsedSeconds / durationSeconds) * 100) + "%";
}
async function loadGPUDemos() {
  try {
    const response = await fetch("/api/v1/gpu-demos", {
      cache: "no-store",
    });
    if (!response.ok) throw new Error("HTTP " + response.status + "");
    gpuDemos = await response.json();
    await refreshGPUDemoStatus();
  } catch (error) {
    gpuDemos = [];
    renderGPUDemos();
    console.error("Unable to load GPU demos", error);
  }
}
async function refreshGPUDemoStatus() {
  if (!gpuDemos.length) return;
  try {
    const response = await fetch("/api/v1/gpu-demo-status", {
      cache: "no-store",
    });
    if (!response.ok) throw new Error("HTTP " + response.status + "");
    renderGPUDemos(await response.json());
  } catch (error) {
    console.error("Unable to read GPU demo status", error);
  }
}
async function runGPUDemo(demo) {
  pendingGPUDemo = demo;
  setText("gpu-demo-modal-title", demo.name);
  setText("gpu-demo-modal-description", demo.description);
  setText("gpu-demo-modal-api", demo.api);
  setText("gpu-demo-modal-duration", "" + demo.duration_seconds + " seconds");
  $("gpu-demo-modal").showModal();
}
async function confirmGPUDemo() {
  const demo = pendingGPUDemo;
  if (!demo) return;
  $("gpu-demo-modal").close();
  pendingGPUDemo = null;
  try {
    const response = await fetch("/api/v1/gpu-demo-run", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-VAR-Studio-Action": "gpu-demo",
      },
      body: JSON.stringify({
        id: demo.id,
      }),
    });
    if (!response.ok)
      throw new Error(
        (await response.text()).trim() || "HTTP " + response.status + "",
      );
    renderGPUDemos(await response.json());
  } catch (error) {
    showGPUDemoError("Unable to start demo: " + error.message + "");
  }
}
function showGPUDemoError(message) {
  const result = $("gpu-demo-result");
  setText("gpu-demo-state", "ACTION FAILED");
  setText("gpu-demo-result-title", "GPU demo action failed");
  setText("gpu-demo-result-meta", new Date().toLocaleString("en-US"));
  $("gpu-demo-output").textContent = message;
  result.classList.add("failed");
  result.classList.remove("hidden");
  result.open = true;
}
async function stopGPUDemo() {
  try {
    const response = await fetch("/api/v1/gpu-demo-stop", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-VAR-Studio-Action": "gpu-demo",
      },
      body: "{}",
    });
    if (!response.ok)
      throw new Error(
        (await response.text()).trim() || "HTTP " + response.status + "",
      );
    setText("gpu-demo-state", "STOPPING");
  } catch (error) {
    showGPUDemoError("Unable to stop demo: " + error.message + "");
  }
}
setInterval(
  () => setText("clock", new Date().toLocaleTimeString("en-US")),
  1000,
);
renderSpark();
refresh();
refreshHistory();
refreshGPUHistory();
refreshNPUHistory();
setInterval(refresh, 2000);
setInterval(refreshHistory, 10000);
setInterval(refreshGPUHistory, 5000);
setInterval(refreshNPUHistory, 5000);
window.addEventListener("resize", () => {
  if (metricHistory) renderHistory(metricHistory);
  if (gpuHistory) renderGPUHistory(gpuHistory);
  if (npuHistory) renderNPUHistory(npuHistory);
});
document.querySelectorAll(".range-selector button").forEach((button) =>
  button.addEventListener("click", () => {
    selectedRange = button.dataset.range;
    document
      .querySelectorAll(".range-selector button")
      .forEach((item) => item.classList.toggle("active", item === button));
    refreshHistory();
    refreshGPUHistory();
    refreshNPUHistory();
  }),
);
$("product-modal-close").addEventListener("click", () =>
  $("product-modal").close(),
);
$("product-modal").addEventListener("click", (event) => {
  if (event.target === $("product-modal")) $("product-modal").close();
});
