const $ = (id) => document.getElementById(id);
const setText = (id, value) => {
  $(id).textContent = value ?? "—";
};
const commitMetadata = new Map();
let metadataLoading = false;
function link(text, url, title) {
  if (!url) return document.createTextNode(text);
  const a = document.createElement("a");
  a.className = "layer-link";
  a.href = url;
  a.target = "_blank";
  a.rel = "noopener noreferrer";
  a.title = title;
  a.append(document.createTextNode(text));
  const arrow = document.createElement("span");
  arrow.textContent = "↗";
  a.append(arrow);
  return a;
}
function renderCommitDetails(cell, layer) {
  cell.className = "commit-cell";
  const metadata = commitMetadata.get(layer.name);
  if (!layer.revision_url) {
    const badge = document.createElement("span");
    badge.className = "metadata-unavailable";
    badge.textContent = "NOT MAPPED";
    badge.title = "This layer does not have a known public reposito" + "ry";
    cell.append(badge);
    return;
  }
  if (!metadata) {
    cell.innerHTML =
      '<span class="metadata-loading"></span><small>Loa' +
      "ding author information\u2026</small>";
    return;
  }
  if (!metadata.available) {
    const badge = document.createElement("span");
    badge.className = "metadata-unavailable";
    badge.textContent = "UNAVAILABLE";
    badge.title = metadata.message || "Commit metadata is unavailable";
    cell.append(badge);
    return;
  }
  const details = document.createElement("div");
  details.className = "commit-details";
  const author = document.createElement("strong");
  author.textContent = metadata.author || "Unknown author";
  const email = document.createElement("a");
  email.href = "mailto:" + metadata.email + "";
  email.textContent = metadata.email || "Email unavailable";
  const date = document.createElement("time");
  date.dateTime = metadata.date;
  date.textContent = metadata.date
    ? new Date(metadata.date).toLocaleString("en-US", {
        dateStyle: "medium",
        timeStyle: "short",
      })
    : "Date unavailable";
  details.append(author, email, date);
  cell.append(details);
}
function render(snapshot) {
  const { bsp, system, timestamp } = snapshot,
    layers = bsp.layers || [],
    modified = layers.filter((layer) => layer.modified).length;
  setText("bsp-distro", bsp.distro || "—");
  setText("bsp-version", bsp.distro_version || system.os_version);
  setText("kernel", system.kernel);
  setText("layers-count", layers.length);
  setText(
    "image-name",
    "" + system.model + " \xB7 " + system.architecture.toUpperCase() + "",
  );
  setText(
    "generated-at",
    "Read from the running board at " +
      new Date(timestamp).toLocaleString("en-US") +
      "",
  );
  setText(
    "modified-count",
    modified ? "" + modified + " MODIFIED" : "ALL CLEAN",
  );
  const root = $("layers");
  root.replaceChildren();
  layers.forEach((layer) => {
    const row = document.createElement("tr"),
      name = document.createElement("td"),
      revision = document.createElement("td"),
      details = document.createElement("td"),
      status = document.createElement("td");
    name.append(link(layer.name, layer.repository_url, "Open repository"));
    revision.append(
      link(layer.revision, layer.revision_url, "Open this commit"),
    );
    renderCommitDetails(details, layer);
    status.textContent = layer.modified ? "Modified" : "Clean";
    if (layer.modified) status.className = "modified";
    row.append(name, revision, details, status);
    root.append(row);
  });
}
async function loadCommitMetadata(snapshot) {
  if (metadataLoading || commitMetadata.size) return;
  metadataLoading = true;
  try {
    const response = await fetch("/api/v1/commit-metadata");
    if (!response.ok) throw new Error("HTTP " + response.status + "");
    for (const metadata of await response.json())
      commitMetadata.set(metadata.name, metadata);
    render(snapshot);
  } catch (error) {
    console.warn("Commit metadata unavailable", error);
  } finally {
    metadataLoading = false;
  }
}
async function refresh() {
  try {
    const response = await fetch("/api/v1/snapshot", {
      cache: "no-store",
    });
    if (!response.ok) throw new Error("HTTP " + response.status + "");
    const snapshot = await response.json();
    render(snapshot);
    loadCommitMetadata(snapshot);
    setText("connection", "Live updates");
  } catch (error) {
    setText("connection", "Disconnected");
    console.error(error);
  }
}
setInterval(
  () => setText("clock", new Date().toLocaleTimeString("en-US")),
  1000,
);
refresh();
setInterval(refresh, 10000);
