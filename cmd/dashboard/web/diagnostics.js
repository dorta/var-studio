const $ = (id) => document.getElementById(id);
let dashboard = null;
let selectedGuide = "";
let showAllEvents = false;

const escapeHTML = (value) =>
  String(value ?? "").replace(
    /[&<>"']/g,
    (character) =>
      ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;",
      })[character],
  );

const setText = (id, value) => {
  const element = $(id);
  if (element) element.textContent = value ?? "—";
};

const statusName = (status) =>
  ({
    passed: "Passed",
    healthy: "Healthy",
    warning: "Warning",
    critical: "Critical",
    unavailable: "Unavailable",
  })[status] || status;

function renderHealth() {
  const health = dashboard.health;
  const copy = {
    healthy: {
      title: "All checks passed",
      summary: "No action is required. The board is operating normally.",
    },
    warning: {
      title: "Review recommended",
      summary: "One or more checks may require your attention.",
    },
    critical: {
      title: "Action required",
      summary: "A critical board condition was detected.",
    },
  };
  const message = copy[health.status] || {
    title: "Status unavailable",
    summary: health.summary,
  };
  $("health-orb").className = "health-orb " + health.status;
  setText("health-status", message.title);
  setText("health-summary", message.summary);
  setText("health-passed", health.passed);
  setText("health-warnings", health.warnings);
  setText("health-critical", health.critical);
  setText(
    "health-time",
    "Last checked at " +
      new Date(health.evaluated_at).toLocaleTimeString(),
  );
}

function renderAttention() {
  const checks = dashboard.health.checks || [];
  const attention = checks.filter((check) => check.status !== "passed");
  setText(
    "attention-count",
    attention.length ? attention.length + " FOUND" : "NONE",
  );
  if (!attention.length) {
    $("attention-list").innerHTML =
      '<div class="attention-clear"><span>✓</span><div>' +
      "<strong>No problems detected</strong>" +
      "<p>All automatic checks passed. No action is required.</p>" +
      "</div></div>";
    return;
  }
  $("attention-list").innerHTML = attention
    .map(
      (check) =>
        '<article class="attention-item ' +
        escapeHTML(check.status) +
        '"><span>!</span><div><strong>' +
        escapeHTML(check.name) +
        "</strong><p>" +
        escapeHTML(check.summary) +
        "</p></div><small>" +
        escapeHTML(check.evidence) +
        "</small></article>",
    )
    .join("");
}

function renderChecks() {
  const checks = dashboard.health.checks || [];
  setText("check-count", checks.length + " CHECKS");
  $("health-checks").innerHTML = checks
    .map(
      (check) =>
        '<details class="health-check ' +
        escapeHTML(check.status) +
        '"><summary><span class="check-icon">' +
        (check.status === "passed" ? "✓" : "!") +
        "</span><strong>" +
        escapeHTML(check.name) +
        "</strong><em>" +
        escapeHTML(statusName(check.status)) +
        '</em></summary><div class="check-detail"><p>' +
        escapeHTML(check.summary) +
        "</p><small>Evidence: " +
        escapeHTML(check.evidence) +
        "</small></div></details>",
    )
    .join("");
}

function isNoisyEvent(event) {
  const content = (event.title + " " + event.detail).toLowerCase();
  return (
    content.includes("callbacks suppressed") ||
    content.includes("kauditd_printk_skb") ||
    content.includes("docker0 link changed down")
  );
}

function uniqueEvents(events) {
  const seen = new Set();
  return events.filter((event) => {
    const key = event.severity + "\n" + event.title + "\n" + event.detail;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function visibleEvents() {
  const events = dashboard.events || [];
  if (showAllEvents) return uniqueEvents(events).slice(0, 40);
  return uniqueEvents(events)
    .filter((event) => event.severity !== "info")
    .filter((event) => !isNoisyEvent(event))
    .slice(0, 8);
}

function renderEvents() {
  const events = visibleEvents();
  $("event-list").innerHTML = events.length
    ? events
        .map(
          (event) =>
            '<article class="event-row"><header><strong>' +
            escapeHTML(event.title) +
            "</strong><time>" +
            new Date(event.timestamp).toLocaleTimeString() +
            "</time></header><p>" +
            escapeHTML(event.detail) +
            "</p><small>" +
            escapeHTML(event.category.toUpperCase()) +
            "</small></article>",
        )
        .join("")
    : '<p class="event-empty">No relevant warnings were recorded. ' +
      "Enable “Show all events” to inspect routine system activity.</p>";
}

function renderGuide() {
  const guides = dashboard.guides || [];
  if (!selectedGuide && guides.length) selectedGuide = guides[0].id;
  $("guide-list").innerHTML = guides
    .map(
      (guide) =>
        '<button type="button" data-guide="' +
        escapeHTML(guide.id) +
        '" class="' +
        (guide.id === selectedGuide ? "active" : "") +
        '">' +
        escapeHTML(guide.name) +
        "</button>",
    )
    .join("");
  document.querySelectorAll("[data-guide]").forEach((button) =>
    button.addEventListener("click", () => {
      selectedGuide = button.dataset.guide;
      renderGuide();
    }),
  );
  const guide = guides.find((item) => item.id === selectedGuide);
  if (!guide) {
    $("guide-detail").innerHTML =
      '<p class="loading-copy">No troubleshooting guides available.</p>';
    return;
  }
  $("guide-detail").innerHTML =
    "<h3>" +
    escapeHTML(guide.name) +
    "</h3><p>" +
    escapeHTML(guide.description) +
    "</p>" +
    guide.steps
      .map(
        (step) =>
          '<div class="guide-step ' +
          escapeHTML(step.status) +
          '"><i></i><strong>' +
          escapeHTML(step.name) +
          "</strong><small>" +
          escapeHTML(step.evidence) +
          "</small></div>",
      )
      .join("") +
    '<div class="guide-conclusion"><strong>Conclusion</strong><p>' +
    escapeHTML(guide.conclusion) +
    "</p></div>";
}

function renderActions() {
  $("action-list").innerHTML = (dashboard.actions || [])
    .map(
      (action) =>
        '<article class="action-card"><strong>' +
        escapeHTML(action.name) +
        "</strong><p>" +
        escapeHTML(
          action.available ? action.description : action.unavailable_reason,
        ) +
        '</p><button type="button" data-action="' +
        escapeHTML(action.id) +
        '" ' +
        (action.available ? "" : "disabled") +
        ">" +
        (action.available ? "Run test" : "Unavailable") +
        "</button></article>",
    )
    .join("");
  document.querySelectorAll("[data-action]").forEach((button) =>
    button.addEventListener("click", () =>
      runAction(button.dataset.action, button),
    ),
  );
}

async function runAction(id, button) {
  button.disabled = true;
  button.textContent = "Running…";
  try {
    const response = await fetch("/api/v1/diagnostic-run", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-VAR-Studio-Action": "diagnostic",
      },
      body: JSON.stringify({ id }),
    });
    if (!response.ok) {
      const detail = (await response.text()).trim();
      throw new Error(detail || "Diagnostic failed");
    }
    const result = await response.json();
    $("action-result").classList.remove("hidden");
    $("action-result").innerHTML =
      "<strong>" +
      escapeHTML(result.summary) +
      "</strong><p>Status: " +
      escapeHTML(statusName(result.status)) +
      " · " +
      result.duration_ms +
      " ms</p><ul>" +
      (result.checks || [])
        .map(
          (check) =>
            "<li>" +
            escapeHTML(check.name) +
            " — " +
            escapeHTML(statusName(check.status)) +
            (check.evidence ? " · " + escapeHTML(check.evidence) : "") +
            "</li>",
        )
        .join("") +
      "</ul>";
    await loadDiagnostics();
  } catch (error) {
    $("action-result").classList.remove("hidden");
    $("action-result").innerHTML =
      "<strong>Test could not run</strong><p>" +
      escapeHTML(error.message) +
      "</p>";
  } finally {
    button.disabled = false;
    button.textContent = "Run test";
  }
}

async function loadDiagnostics() {
  const refresh = $("refresh-diagnostics");
  refresh.disabled = true;
  refresh.textContent = "Checking…";
  try {
    const response = await fetch("/api/v1/diagnostics", {
      cache: "no-store",
    });
    if (!response.ok) throw new Error("Diagnostics unavailable");
    dashboard = await response.json();
    renderHealth();
    renderAttention();
    renderChecks();
    renderEvents();
    renderGuide();
    renderActions();
    setText("connection", "Live updates");
  } catch (error) {
    setText("connection", "Unavailable");
    setText("health-status", "Diagnostics unavailable");
    setText("health-summary", "The board did not return diagnostic data.");
    console.error(error);
  } finally {
    refresh.disabled = false;
    refresh.textContent = "Run diagnostics";
  }
}

$("show-all-events").addEventListener("change", (event) => {
  showAllEvents = event.target.checked;
  if (dashboard) renderEvents();
});
$("refresh-diagnostics").addEventListener("click", loadDiagnostics);
setInterval(() => setText("clock", new Date().toLocaleTimeString()), 1000);
setInterval(loadDiagnostics, 10000);
loadDiagnostics();
