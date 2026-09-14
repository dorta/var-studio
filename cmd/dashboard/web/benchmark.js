const $ = (id) => document.getElementById(id);
let demos = [],
  status = {
    state: "idle",
  },
  pendingDemo = null,
  history = null,
  gpuHistory = null,
  npuHistory = null;
const colors = {
  cpu: "#ff5f46",
  gpu: "#00b875",
  npu: "#8b5cf6",
  memory: "#5f7ea8",
};
const setText = (id, value) => {
  $(id).textContent = value ?? "—";
};
const node = (tag, className, text) => {
  const element = document.createElement(tag);
  if (className) element.className = className;
  if (text !== undefined) element.textContent = text;
  return element;
};
const svgNode = (tag, attributes = {}) => {
  const element = document.createElementNS("http://www.w3.org/2000/svg", tag);
  Object.entries(attributes).forEach(([key, value]) =>
    element.setAttribute(key, value),
  );
  return element;
};
function percent(used, total) {
  return total ? Math.min(100, (used / total) * 100) : 0;
}
function testWindow() {
  const started = Date.parse(status.started_at);
  const finished = Date.parse(status.finished_at);
  if (!Number.isNaN(started)) {
    const demo = demos.find((item) => item.id === status.demo_id);
    const duration =
      Number(status.duration_seconds || demo?.duration_seconds || 20) * 1000;
    const scheduledEnd = started + duration;
    const actualEnd =
      !Number.isNaN(finished) && finished > started ? finished : scheduledEnd;
    return {
      start: started - 10000,
      testStart: started,
      testEnd: actualEnd,
      end: actualEnd + 10000,
    };
  }
  return {
    start: Date.now() - 120000,
    end: Date.now(),
  };
}
function pointsFor(samples, field, window) {
  return (samples || [])
    .map((sample) => [Date.parse(sample.timestamp), Number(sample[field] || 0)])
    .filter(
      ([time]) =>
        !Number.isNaN(time) && time >= window.start && time <= window.end,
    );
}
function sampleStats(values) {
  if (!values.length)
    return {
      average: null,
      peak: null,
      count: 0,
    };
  const numbers = values.map(([, value]) => value);
  return {
    average: numbers.reduce((sum, value) => sum + value, 0) / numbers.length,
    peak: Math.max(...numbers),
    count: numbers.length,
  };
}
function renderChart() {
  const root = $("benchmark-chart");
  root.replaceChildren();
  const window = testWindow();
  const series = [
    ["cpu", pointsFor(history?.samples, "cpu_percent", window)],
    ["gpu", pointsFor(gpuHistory?.samples, "usage_percent", window)],
    ["npu", pointsFor(npuHistory?.samples, "usage_percent", window)],
    ["memory", pointsFor(history?.samples, "memory_percent", window)],
  ];
  const populated = series.filter(([, values]) => values.length > 0);
  if (!populated.length) {
    root.append(
      node("span", "chart-empty", "Collecting synchronized metrics…"),
    );
    return;
  }
  const width = 1000,
    height = 280,
    left = 44,
    right = 986,
    top = 14,
    bottom = 246;
  const svg = svgNode("svg", {
    viewBox: "0 0 1000 280",
    preserveAspectRatio: "none",
    role: "img",
    "aria-label": "CPU GPU NPU and memory utilization during benchm" + "ark",
  });
  const span = Math.max(1, window.end - window.start);
  const xFor = (time) =>
    left +
    Math.max(0, Math.min(1, (time - window.start) / span)) * (right - left);
  if (window.testStart) {
    const phaseStart = xFor(window.testStart),
      phaseEnd = xFor(window.testEnd);
    svg.append(
      svgNode("rect", {
        x: phaseStart,
        y: top,
        width: Math.max(2, phaseEnd - phaseStart),
        height: bottom - top,
        class: "benchmark-phase",
      }),
    );
    [
      ["START", phaseStart],
      ["END", phaseEnd],
    ].forEach(([label, x]) => {
      svg.append(
        svgNode("line", {
          x1: x,
          y1: top,
          x2: x,
          y2: bottom,
          class: "benchmark-marker",
        }),
      );
      const text = svgNode("text", {
        x: x + (label === "START" ? 5 : -5),
        y: top + 12,
        class: "benchmark-marker-label",
        "text-anchor": label === "START" ? "start" : "end",
      });
      text.textContent = label;
      svg.append(text);
    });
    if (status.state === "running")
      svg.append(
        svgNode("line", {
          x1: xFor(Math.min(Date.now(), window.testEnd)),
          y1: top,
          x2: xFor(Math.min(Date.now(), window.testEnd)),
          y2: bottom,
          class: "benchmark-cursor",
        }),
      );
  }
  [0, 25, 50, 75, 100].forEach((value) => {
    const y = top + ((100 - value) * (bottom - top)) / 100;
    svg.append(
      svgNode("line", {
        x1: left,
        y1: y,
        x2: right,
        y2: y,
        class: "benchmark-grid",
      }),
    );
    const label = svgNode("text", {
      x: left - 9,
      y: y + 4,
      class: "benchmark-axis",
      "text-anchor": "end",
    });
    label.textContent = value + "%";
    svg.append(label);
  });
  populated.forEach(([name, values]) => {
    const points = values
      .map(([time, value]) => {
        const x = xFor(time);
        const y =
          top +
          ((100 - Math.max(0, Math.min(100, value))) * (bottom - top)) / 100;
        return x.toFixed(1) + "," + y.toFixed(1);
      })
      .join(" ");
    svg.append(
      svgNode("polyline", {
        points,
        fill: "none",
        stroke: colors[name],
        "stroke-width": 2.2,
        "vector-effect": "non-scaling-stroke",
      }),
    );
  });
  root.append(svg);
  const sessionRoot = $("session-chart");
  if (sessionRoot && status.state === "running") {
    sessionRoot.replaceChildren(svg.cloneNode(true));
  }
  const startLabel = new Date(window.start).toLocaleTimeString([], {
    hour12: false,
  });
  const endLabel = new Date(window.end).toLocaleTimeString([], {
    hour12: false,
  });
  setText(
    "chart-range",
    status.started_at
      ? "" +
          (status.demo_name || "Benchmark") +
          " \xB7 " +
          startLabel +
          "\u2013" +
          endLabel +
          ""
      : "Live preview \xB7 last 2 minutes",
  );
}
function renderLive(snapshot) {
  const metrics = snapshot.metrics || {};
  const cpu = Number(metrics.cpu_percent || 0).toFixed(0) + "%";
  const memory =
    percent(metrics.memory_used_bytes, metrics.memory_total_bytes).toFixed(0) +
    "%";
  setText("live-cpu", cpu);
  setText("live-memory", memory);
  const samples = gpuHistory?.samples || [];
  const gpu =
    gpuHistory?.available && samples.length
      ? Number(samples[samples.length - 1].usage_percent || 0).toFixed(0) + "%"
      : "SAMPLING";
  setText("live-gpu", gpu);
  setText("session-cpu", cpu);
  setText("session-memory", memory);
  setText("session-gpu", gpu);
  const npuSamples = npuHistory?.samples || [],
    npuAvailable = Boolean(npuHistory?.available);
  const npu =
    npuAvailable && npuSamples.length
      ? Number(npuSamples[npuSamples.length - 1].usage_percent || 0).toFixed(
          0,
        ) + "%"
      : "SAMPLING";
  ["live-npu-card", "benchmark-npu-legend", "session-npu-card"].forEach((id) =>
    $(id).classList.toggle("hidden", !npuAvailable),
  );
  setText("live-npu", npu);
  setText("session-npu", npu);
}
function renderSummary() {
  const root = $("benchmark-summary");
  root.replaceChildren();
  const window = testWindow();
  const strictWindow = {
    start: window.testStart || window.start,
    end: window.testEnd || window.end,
  };
  const rows = [
    [
      "CPU",
      "cpu",
      sampleStats(pointsFor(history?.samples, "cpu_percent", strictWindow)),
    ],
    [
      "GPU",
      "gpu",
      sampleStats(
        pointsFor(gpuHistory?.samples, "usage_percent", strictWindow),
      ),
    ],
    ...(npuHistory?.available
      ? [
          [
            "NPU",
            "npu",
            sampleStats(
              pointsFor(npuHistory?.samples, "usage_percent", strictWindow),
            ),
          ],
        ]
      : []),
    [
      "Memory",
      "memory",
      sampleStats(pointsFor(history?.samples, "memory_percent", strictWindow)),
    ],
  ];
  rows.forEach(([label, type, stats]) => {
    const card = node("article", "benchmark-summary-card " + type + "");
    card.append(
      node("span", "", label),
      node(
        "strong",
        "",
        stats.average === null
          ? "No samples"
          : "" + stats.average.toFixed(1) + "% average",
      ),
      node(
        "small",
        "",
        stats.peak === null
          ? "Collector did not report during this window"
          : "" +
              stats.peak.toFixed(1) +
              "% peak \xB7 " +
              stats.count +
              " sample" +
              (stats.count === 1 ? "" : "s") +
              "",
      ),
    );
    root.append(card);
  });
}
function renderDemos() {
  const root = $("gpu-demo-list");
  root.replaceChildren();
  const running = status.state === "running";
  setText("runner-state", running ? "TEST RUNNING" : "READY");
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
  $("gpu-demo-progress").classList.toggle("hidden", !running);
  if (running) setText("gpu-demo-progress-name", status.demo_name);
  if (!demos.length) {
    root.append(
      node("span", "chart-empty", "Benchmark runner is unavailable."),
    );
    setText("runner-state", "UNAVAILABLE");
    return;
  }
  demos.forEach((demo) => {
    const card = node("article", "gpu-demo-card"),
      header = node("header");
    header.append(
      node("h4", "", demo.name),
      node("span", "", "" + demo.api + " \xB7 " + demo.duration_seconds + "s"),
    );
    const button = node(
      "button",
      "",
      !demo.installed
        ? "Not installed"
        : running && status.demo_id === demo.id
          ? "Running…"
          : "Run benchmark",
    );
    button.type = "button";
    button.disabled = running || !demo.installed;
    button.addEventListener("click", () => openConfirmation(demo));
    const visual = node("div", "gpu-demo-visual placeholder");
    visual.append(
      node("span", "", "Preview"),
      node("small", "", "Benchmark artwork coming soon"),
    );
    card.append(visual, header, node("p", "", demo.description), button);
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
      finished +
        (status.state === "failed"
          ? " \xB7 exit code " + status.exit_code + ""
          : ""),
    );
    result.classList.remove("hidden");
    renderSummary();
  } else if (running) result.classList.add("hidden");
}
function updateProgress() {
  if (status.state !== "running") return;
  const demo = demos.find((item) => item.id === status.demo_id);
  const duration = Number(
      status.duration_seconds || demo?.duration_seconds || 0,
    ),
    started = Date.parse(status.started_at);
  if (!duration || Number.isNaN(started)) return;
  const elapsed = Math.max(0, (Date.now() - started) / 1000),
    remaining = Math.max(0, Math.ceil(duration - elapsed));
  setText(
    "gpu-demo-countdown",
    "" +
      Math.floor(remaining / 60) +
      ":" +
      String(remaining % 60).padStart(2, "0") +
      "",
  );
  setText("gpu-demo-countdown-label", remaining ? "remaining" : "finishing…");
  setText(
    "gpu-demo-progress-detail",
    "" +
      (demo?.api || "GPU") +
      " workload \xB7 recording CPU, GPU, NPU, and memory",
  );
  $("gpu-demo-progress-bar").style.width =
    Math.min(100, (elapsed / duration) * 100) + "%";
}
function openConfirmation(demo) {
  pendingDemo = demo;
  setText("gpu-demo-modal-title", demo.name);
  setText("gpu-demo-modal-description", demo.description);
  setText("gpu-demo-modal-api", demo.api);
  setText("gpu-demo-modal-duration", demo.duration_seconds + " seconds");
  $("gpu-demo-modal").showModal();
}
async function runBenchmark() {
  const demo = pendingDemo;
  if (!demo) return;
  $("gpu-demo-modal").close();
  pendingDemo = null;
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
    return showError(
      (await response.text()).trim() || "HTTP " + response.status + "",
    );
  status = await response.json();
  renderDemos();
  await refreshMetrics();
}
async function stopBenchmark() {
  const response = await fetch("/api/v1/gpu-demo-stop", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-VAR-Studio-Action": "gpu-demo",
    },
    body: "{}",
  });
  if (!response.ok)
    showError((await response.text()).trim() || "HTTP " + response.status + "");
}
function showError(message) {
  setText("gpu-demo-state", "ACTION FAILED");
  setText("gpu-demo-result-title", "Benchmark action failed");
  setText("gpu-demo-result-meta", new Date().toLocaleString("en-US"));
  $("gpu-demo-output").textContent = message;
  $("gpu-demo-result").classList.add("failed");
  $("gpu-demo-result").classList.remove("hidden");
  $("gpu-demo-result").open = true;
}
async function refreshMetrics() {
  try {
    const [snapshotResponse, historyResponse, gpuResponse, npuResponse] =
      await Promise.all([
        fetch("/api/v1/snapshot", {
          cache: "no-store",
        }),
        fetch("/api/v1/history?range=hour", {
          cache: "no-store",
        }),
        fetch("/api/v1/gpu-history?range=hour", {
          cache: "no-store",
        }),
        fetch("/api/v1/npu-history?range=hour", {
          cache: "no-store",
        }),
      ]);
    if (
      !snapshotResponse.ok ||
      !historyResponse.ok ||
      !gpuResponse.ok ||
      !npuResponse.ok
    )
      throw new Error("metrics API unavailable");
    history = await historyResponse.json();
    gpuHistory = await gpuResponse.json();
    npuHistory = await npuResponse.json();
    renderLive(await snapshotResponse.json());
    renderChart();
    setText("connection", "Live updates");
  } catch (error) {
    setText("connection", "Disconnected");
    console.error(error);
  }
}
async function refreshStatus() {
  try {
    const response = await fetch("/api/v1/gpu-demo-status", {
      cache: "no-store",
    });
    if (!response.ok) throw new Error("HTTP " + response.status + "");
    status = await response.json();
    renderDemos();
    updateProgress();
    renderChart();
  } catch (error) {
    console.error(error);
  }
}
async function initialize() {
  try {
    const response = await fetch("/api/v1/gpu-demos", {
      cache: "no-store",
    });
    if (!response.ok) throw new Error("HTTP " + response.status + "");
    demos = (await response.json()).filter(
      (item) => !item.kind || item.kind === "gpu",
    );
  } catch (error) {
    console.error(error);
  }
  await Promise.all([refreshStatus(), refreshMetrics()]);
}
$("gpu-demo-modal-close").addEventListener("click", () =>
  $("gpu-demo-modal").close(),
);
$("gpu-demo-modal-cancel").addEventListener("click", () =>
  $("gpu-demo-modal").close(),
);
$("gpu-demo-modal-run").addEventListener("click", runBenchmark);
$("gpu-demo-modal").addEventListener("close", () => {
  pendingDemo = null;
});
$("gpu-demo-modal").addEventListener("click", (event) => {
  if (event.target === $("gpu-demo-modal")) $("gpu-demo-modal").close();
});
$("gpu-demo-stop").addEventListener("click", stopBenchmark);
setInterval(
  () => setText("clock", new Date().toLocaleTimeString("en-US")),
  1000,
);
setInterval(updateProgress, 250);
setInterval(refreshStatus, 1000);
setInterval(refreshMetrics, 2000);
window.addEventListener("resize", renderChart);
initialize();
