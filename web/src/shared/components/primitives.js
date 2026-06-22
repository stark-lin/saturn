// This file renders shared Saturn UI primitives with native DOM APIs.
function addClassNames(node, classNames) {
  classNames
    .filter(Boolean)
    .flatMap((className) => className.split(/\s+/).filter(Boolean))
    .forEach((className) => node.classList.add(className));
}

function appendChildren(parent, children) {
  children.filter(Boolean).forEach((child) => parent.append(child));
}

function setText(node, value) {
  node.textContent = value ?? "";
  return node;
}

function sectionID(title) {
  return `${String(title).toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "")}-title`;
}

function appendCellValue(cell, value) {
  if (value instanceof Node) {
    cell.append(value);
    return;
  }

  if (value && value.node instanceof Node) {
    addClassNames(cell, [value.className]);
    cell.append(value.node);
    return;
  }

  if (value && typeof value === "object") {
    addClassNames(cell, [value.className]);
    cell.textContent = value.text ?? "";
    return;
  }

  cell.textContent = value ?? "";
}

export function renderSurface(tagName = "section", options = {}) {
  const surface = document.createElement(tagName);
  addClassNames(surface, ["surface", options.raised ? "surface--raised" : "", options.fixed ? "fixed-panel" : "", options.className]);

  if (options.labelledBy) {
    surface.setAttribute("aria-labelledby", options.labelledBy);
  }

  if (options.label) {
    surface.setAttribute("aria-label", options.label);
  }

  appendChildren(surface, options.children ?? []);
  return surface;
}

function renderSectionHeader({ title, note, id, actions = [] }) {
  const header = document.createElement("header");
  header.className = "section-head";
  if (actions.length > 0) {
    header.classList.add("section-head--actions");
  }

  const titleNode = document.createElement("h2");
  titleNode.className = "section-title";
  titleNode.id = id ?? sectionID(title);
  titleNode.textContent = title;
  header.append(titleNode);

  if (note) {
    const noteNode = document.createElement("p");
    noteNode.className = "section-note";
    noteNode.textContent = note;
    header.append(noteNode);
  }

  appendChildren(header, actions);
  return header;
}

export function renderSection({ title, note, id, actions = [], children = [] }) {
  const titleID = id ?? sectionID(title);
  const section = renderSurface("section", {
    className: "section",
    labelledBy: titleID,
  });

  section.append(renderSectionHeader({ title, note, id: titleID, actions }));
  appendChildren(section, children);
  return section;
}

export function renderButton(label, options = {}) {
  const button = document.createElement("button");
  button.type = options.type ?? "button";
  addClassNames(button, [
    "button",
    options.variant === "primary" ? "button--primary" : "",
    options.variant === "danger" ? "button--danger" : "",
    options.flat ? "button--flat" : "",
    options.chip ? "button--chip" : "",
    options.className,
  ]);
  button.textContent = label;

  if (typeof options.pressed === "boolean") {
    button.setAttribute("aria-pressed", String(options.pressed));
  }

  if (options.label) {
    button.setAttribute("aria-label", options.label);
  }

  if (options.onClick) {
    button.addEventListener("click", options.onClick);
  }

  return button;
}

export function renderInputGroupField({
  label,
  name,
  prefix,
  suffix,
  placeholder = "",
  value = "",
  type = "text",
}) {
  const field = document.createElement("label");
  field.className = "control-field";

  const labelNode = document.createElement("span");
  labelNode.className = "control-label";
  labelNode.textContent = label;

  const group = document.createElement("span");
  group.className = "input-group";

  const prefixNode = document.createElement("span");
  prefixNode.setAttribute("aria-hidden", "true");
  prefixNode.textContent = prefix;

  const input = document.createElement("input");
  input.name = name;
  input.type = type;
  input.placeholder = placeholder;
  input.value = value;
  input.setAttribute("aria-label", label);

  const suffixNode = document.createElement("span");
  suffixNode.setAttribute("aria-hidden", "true");
  suffixNode.textContent = suffix;

  group.append(prefixNode, input, suffixNode);
  field.append(labelNode, group);
  return field;
}

export function renderSelectField({ label, name, options, value = "" }) {
  const field = document.createElement("label");
  field.className = "control-field";

  const labelNode = document.createElement("span");
  labelNode.className = "control-label";
  labelNode.textContent = label;

  const select = document.createElement("select");
  select.className = "select";
  select.name = name;
  select.setAttribute("aria-label", label);
  options.forEach(([optionValue, text]) => {
    const option = document.createElement("option");
    option.value = optionValue;
    option.textContent = text;
    option.selected = optionValue === value;
    select.append(option);
  });

  field.append(labelNode, select);
  return field;
}

export function renderTag(label) {
  const tag = document.createElement("span");
  tag.className = "tag";
  tag.textContent = label;
  return tag;
}

export function renderStatusPill(label, options = {}) {
  const pill = document.createElement("span");
  pill.className = "pill";
  pill.dataset.state = options.state ?? "online";

  const dot = document.createElement("span");
  dot.className = "status-dot";
  dot.setAttribute("aria-hidden", "true");
  pill.append(dot, document.createTextNode(label));
  return pill;
}

export function renderStatusBadge(label, options = {}) {
  const badge = document.createElement("span");
  badge.className = "status-badge";
  badge.setAttribute("role", "status");
  badge.textContent = label;

  if (options.state) {
    badge.dataset.state = options.state;
  }

  return badge;
}

export function renderDataTable({ caption, columns, rows }) {
  const tableScroll = document.createElement("div");
  tableScroll.className = "table-scroll";

  const table = document.createElement("table");
  const captionNode = document.createElement("caption");
  captionNode.textContent = caption;
  table.append(captionNode);

  const thead = document.createElement("thead");
  const headRow = document.createElement("tr");
  columns.forEach((column) => {
    const headerCell = document.createElement("th");
    headerCell.scope = "col";
    headerCell.textContent = column.label;
    headRow.append(headerCell);
  });
  thead.append(headRow);
  table.append(thead);

  const tbody = document.createElement("tbody");
  rows.forEach((row) => {
    const bodyRow = document.createElement("tr");

    columns.forEach((column) => {
      const cell = document.createElement("td");
      addClassNames(cell, [column.className]);
      appendCellValue(cell, row[column.key]);
      bodyRow.append(cell);
    });

    tbody.append(bodyRow);
  });
  table.append(tbody);

  tableScroll.append(table);
  return tableScroll;
}

export function renderNotice({ title, message, tone = "warning" }) {
  const notice = renderSurface("div", { className: "notice", label: title });
  notice.setAttribute("role", "status");
  notice.dataset.tone = tone;

  const dot = document.createElement("span");
  dot.className = "notice-dot";
  dot.setAttribute("aria-hidden", "true");

  const copy = document.createElement("div");
  copy.append(setText(document.createElement("strong"), title), setText(document.createElement("p"), message));
  notice.append(dot, copy);
  return notice;
}

export function renderMeter({ label, value, tone }) {
  const numericValue = Number(value);
  const percent = Number.isFinite(numericValue) ? Math.max(0, Math.min(100, numericValue)) : 0;
  const meter = document.createElement("div");
  meter.className = "meter";
  meter.style.setProperty("--value", `${percent}%`);
  meter.setAttribute("role", "progressbar");
  meter.setAttribute("aria-label", label);
  meter.setAttribute("aria-valuemin", "0");
  meter.setAttribute("aria-valuemax", "100");
  meter.setAttribute("aria-valuenow", String(percent));

  if (tone) {
    meter.dataset.tone = tone;
  }

  const head = document.createElement("div");
  head.className = "meter-head";
  head.append(setText(document.createElement("span"), label), setText(document.createElement("span"), `${percent}%`));

  const track = document.createElement("div");
  track.className = "meter-track";
  const fill = document.createElement("div");
  fill.className = "meter-fill";
  track.append(fill);

  meter.append(head, track);
  return meter;
}
