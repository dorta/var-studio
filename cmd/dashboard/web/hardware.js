const $ = (id) => document.getElementById(id);
const make = (tag, className, text) => {
  const el = document.createElement(tag);
  if (className) el.className = className;
  if (text !== undefined) el.textContent = text;
  return el;
};
const icons = {
  overview: "layout-dashboard",
  system: "monitor-cog",
  cpu: "cpu",
  memory: "memory-stick",
  storage: "hard-drive",
  usb: "usb",
  pci: "circuit-board",
  i2c: "cable",
  mmc: "disc-3",
  display: "monitor",
  input: "keyboard",
  sensor: "thermometer",
  network: "network",
  ports: "ethernet-port",
  filesystem: "folders",
  module: "boxes",
  tree: "git-branch",
  boot: "power",
  language: "languages",
  environment: "braces",
  users: "user",
  groups: "users",
  power: "battery-charging",
  resource: "activity",
  connections: "waypoints",
  route: "route",
  arp: "table-2",
  dns: "server",
  statistics: "chart-no-axes-column",
  shared: "folder-symlink",
};
let report = null;
let currentID = location.hash.slice(1) || "summary";
function renderTree(filter = "") {
  const root = $("section-tree");
  root.replaceChildren();
  if (!report) return;
  const query = filter.trim().toLocaleLowerCase("en-US");
  const groups = new Map();
  report.sections.forEach((section) => {
    if (
      query &&
      !("" + section.group + " " + section.label + "")
        .toLocaleLowerCase("en-US")
        .includes(query)
    )
      return;
    if (!groups.has(section.group)) groups.set(section.group, []);
    groups.get(section.group).push(section);
  });
  groups.forEach((sections, groupName) => {
    const group = make("section", "tree-group");
    group.append(make("h3", "", groupName));
    sections.forEach((section) => {
      const button = make(
        "button",
        "tree-item" + (section.id === currentID ? " active" : "") + "",
      );
      button.type = "button";
      button.dataset.id = section.id;
      const icon = make("span", "tree-icon"),
        image = make("img");
      image.src = "/icons/" + (icons[section.icon] || "activity") + ".svg";
      image.alt = "";
      icon.append(image);
      button.append(icon, make("span", "", section.label));
      button.addEventListener("click", () => selectSection(section.id));
      group.append(button);
    });
    root.append(group);
  });
  if (!root.childElementCount)
    root.append(make("p", "loading", "No categories found"));
}
function selectSection(id) {
  const section =
    report?.sections.find((item) => item.id === id) || report?.sections[0];
  if (!section) return;
  currentID = section.id;
  history.replaceState(null, "", "#" + currentID + "");
  renderTree($("section-search").value);
  renderSection(section);
}
function renderSection(section) {
  $("breadcrumb").textContent =
    "" + section.group + " \u2192 " + section.label + "";
  $("detail-title").textContent = section.label;
  const root = $("detail-body");
  root.replaceChildren();
  if (section.fields.length) {
    const list = make("dl", "info-fields");
    section.fields.forEach((field) => {
      const row = make("div", "info-row");
      row.append(make("dt", "", field.label), make("dd", "", field.value));
      list.append(row);
    });
    root.append(list);
  }
  section.tables.forEach((data) => {
    const block = make("section", "hardware-table-block");
    block.append(make("h3", "", data.title));
    if (!data.rows.length) {
      block.append(
        make("div", "no-data", "No devices or records found on this board."),
      );
      root.append(block);
      return;
    }
    const scroll = make("div", "hardware-table-scroll"),
      table = make("table", "hardware-table"),
      head = make("thead"),
      headRow = make("tr");
    data.columns.forEach((column) => headRow.append(make("th", "", column)));
    head.append(headRow);
    table.append(head);
    const body = make("tbody"),
      inspector = make("section", "row-inspector");
    const selectRow = (row, values) => {
      body.querySelector(".selected")?.classList.remove("selected");
      row.classList.add("selected");
      inspector.replaceChildren(make("h4", "", "Item information"));
      const details = make("dl");
      data.columns.forEach((column, index) => {
        const item = make("div");
        item.append(
          make("dt", "", column),
          make("dd", "", values[index] || "—"),
        );
        details.append(item);
      });
      inspector.append(details);
    };
    let firstRow, firstValues;
    data.rows.forEach((values, index) => {
      const row = make("tr");
      values.forEach((value) => row.append(make("td", "", value || "—")));
      row.tabIndex = 0;
      const activate = () => selectRow(row, values);
      row.addEventListener("click", activate);
      row.addEventListener("keydown", (event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          activate();
        }
      });
      if (index === 0) {
        firstRow = row;
        firstValues = values;
      }
      body.append(row);
    });
    table.append(body);
    scroll.append(table);
    block.append(scroll, inspector);
    root.append(block);
    if (firstRow) selectRow(firstRow, firstValues);
  });
  if (!section.fields.length && !section.tables.length)
    root.append(make("div", "no-data", "No information available."));
}
async function loadHardware() {
  $("refresh").disabled = true;
  $("connection").textContent = "Collecting data";
  try {
    const response = await fetch("/api/v1/hardware", {
      cache: "no-store",
    });
    if (!response.ok) throw new Error("HTTP " + response.status + "");
    report = await response.json();
    $("updated-at").textContent =
      "Updated " + new Date(report.timestamp).toLocaleTimeString("en-US") + "";
    $("connection").textContent = "Inventory available";
    document.querySelector(".pulse").style.background = "var(--green)";
    renderTree($("section-search").value);
    selectSection(currentID);
  } catch (error) {
    $("connection").textContent = "Collection failed";
    document.querySelector(".pulse").style.background = "var(--danger)";
    $("detail-body").replaceChildren(
      make("div", "no-data", "Could not load inventory: " + error.message + ""),
    );
  } finally {
    $("refresh").disabled = false;
  }
}
function downloadReport() {
  if (!report) return;
  const blob = new Blob([JSON.stringify(report, null, 2)], {
      type: "application/json",
    }),
    url = URL.createObjectURL(blob),
    link = document.createElement("a");
  link.href = url;
  link.download =
    "var-studio-hardware-" +
    new Date().toISOString().replaceAll(":", "-") +
    ".json";
  link.click();
  URL.revokeObjectURL(url);
}
$("section-search").addEventListener("input", (event) =>
  renderTree(event.target.value),
);
$("refresh").addEventListener("click", loadHardware);
$("report").addEventListener("click", downloadReport);
addEventListener("hashchange", () => selectSection(location.hash.slice(1)));
setInterval(() => {
  $("clock").textContent = new Date().toLocaleTimeString("en-US");
}, 1000);
loadHardware();
