// This file renders ObjectRef metadata search and detail views.
import { getJSON, postJSON } from "../../shared/api/client.js";
import {
  renderButton,
  renderNotice,
  renderStatusBadge,
  renderSurface,
  renderTag,
} from "../../shared/components/primitives.js";
import { el } from "../../shared/utils/dom.js";

const moduleOptions = [
  ["", "ANY"],
  ["notes", "Notes"],
  ["files", "Files"],
  ["accounting", "Accounting"],
  ["calendar", "Calendar"],
  ["llm", "LLM"],
];

const objectTypeOptions = [
  ["", "ANY"],
  ["note", "Note"],
  ["file_collection", "File Collection"],
  ["file", "File"],
  ["event_aggregate", "Event Aggregate"],
  ["event", "Event"],
  ["account", "Account"],
  ["transaction", "Transaction"],
  ["llm_session", "LLM Session"],
  ["llm_request", "LLM Request"],
];

const sortFieldOptions = [
  ["updated_at", "Updated"],
  ["created_at", "Created"],
  ["ref_code", "Ref Code"],
];

const sortDirectionOptions = [
  ["desc", "Descending"],
  ["asc", "Ascending"],
];

const defaultLimit = 50;

function appendText(parent, tagName, className, text) {
  const node = el(tagName, className);
  node.textContent = text;
  parent.append(node);
  return node;
}

function titleCase(value) {
  return String(value ?? "")
    .split("_")
    .filter(Boolean)
    .map((item) => `${item.charAt(0).toUpperCase()}${item.slice(1)}`)
    .join(" ");
}

function formatDateTime(value) {
  if (!value) {
    return "--";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return String(value);
  }
  return new Intl.DateTimeFormat("en-AU", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function splitList(value) {
  const values = [];
  const seen = new Set();
  String(value ?? "")
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
    .forEach((item) => {
      if (seen.has(item)) {
        return;
      }
      seen.add(item);
      values.push(item);
    });
  return values;
}

function selectedValues(select) {
  return Array.from(select.selectedOptions).map((option) => option.value).filter(Boolean);
}

function localDateToRFC3339(value, endExclusive = false) {
  if (!value) {
    return "";
  }
  const date = new Date(`${value}T00:00:00Z`);
  if (endExclusive) {
    date.setUTCDate(date.getUTCDate() + 1);
  }
  return date.toISOString();
}

function readListParameter(parameters, name) {
  return parameters.getAll(name).flatMap(splitList);
}

function setSelectValues(select, values) {
  const allowed = new Set(values);
  Array.from(select.options).forEach((option) => {
    option.selected = allowed.has(option.value);
  });
  if (!select.multiple && values.length === 0 && select.options.length > 0) {
    select.options[0].selected = true;
  }
}

function statusTone(status) {
  switch (status) {
    case "active":
    case "posted":
    case "finished":
      return "online";
    case "voided":
    case "deleted":
      return "off";
    default:
      return "warning";
  }
}

function renderControlLabel(text) {
  const label = el("span", "control-label");
  label.textContent = text;
  return label;
}

function renderField({ label, name, type = "text", placeholder = "", value = "", min, max }) {
  const field = el("label", "control-field");
  const input = el("input", "field");
  input.name = name;
  input.type = type;
  input.placeholder = placeholder;
  input.value = value;
  if (min) {
    input.min = min;
  }
  if (max) {
    input.max = max;
  }
  field.append(renderControlLabel(label), input);
  return field;
}

function renderSelect({ label, name, options, value = "" }) {
  const field = el("label", "control-field");
  const select = el("select", "select");
  select.name = name;
  options.forEach(([optionValue, optionLabel]) => {
    const option = document.createElement("option");
    option.value = optionValue;
    option.textContent = optionLabel;
    option.selected = optionValue === value;
    select.append(option);
  });
  field.append(renderControlLabel(label), select);
  return field;
}

function renderTable(caption, columns) {
  const scroll = el("div", "table-scroll");
  const table = document.createElement("table");
  const captionNode = document.createElement("caption");
  captionNode.textContent = caption;
  const head = document.createElement("thead");
  const headRow = document.createElement("tr");
  columns.forEach(({ label, className }) => {
    const cell = document.createElement("th");
    cell.scope = "col";
    cell.className = className ?? "";
    cell.textContent = label;
    headRow.append(cell);
  });
  head.append(headRow);
  const body = document.createElement("tbody");
  table.append(captionNode, head, body);
  scroll.append(table);
  return { element: scroll, body };
}

function renderTableCell(row, value, className = "") {
  const cell = document.createElement("td");
  cell.className = className;
  if (value instanceof Node) {
    cell.append(value);
  } else {
    cell.textContent = value ?? "";
  }
  row.append(cell);
  return cell;
}

function renderEmptyRow(body, columnCount, text) {
  const row = document.createElement("tr");
  const cell = renderTableCell(row, text, "search-empty-cell");
  cell.colSpan = columnCount;
  body.replaceChildren(row);
}

function renderTagCollection(tags) {
  const collection = el("div", "search-tags");
  (tags?.length ? tags : ["untagged"]).forEach((tag) => collection.append(renderTag(tag)));
  return collection;
}

function enableRow(row, onOpen) {
  row.tabIndex = 0;
  row.addEventListener("click", onOpen);
  row.addEventListener("keydown", (event) => {
    if (event.key !== "Enter" && event.key !== " ") {
      return;
    }
    event.preventDefault();
    onOpen();
  });
}

function detailHash(refCode) {
  return `#search?ref_code=${encodeURIComponent(refCode)}`;
}

function renderReadonlyDetail(label, className = "") {
  const element = el("div", `search-readonly ${className}`.trim());
  element.append(renderControlLabel(label));
  const value = appendText(element, "div", "search-readonly__value", "---");
  return { element, value };
}

function buildParametersFromForm(form) {
  const parameters = new URLSearchParams();
  selectedValues(form.elements.modules).forEach((value) => parameters.append("modules", value));
  selectedValues(form.elements.object_types).forEach((value) => parameters.append("object_types", value));
  splitList(form.elements.statuses.value).forEach((value) => parameters.append("statuses", value));
  splitList(form.elements.tags.value).forEach((value) => parameters.append("tags", value));
  [
    "created_from",
    "created_to",
    "updated_from",
    "updated_to",
    "sort_field",
    "sort_direction",
    "limit",
  ].forEach((name) => {
    const value = String(form.elements[name].value ?? "").trim();
    if (value) {
      parameters.set(name, value);
    }
  });
  return parameters;
}

function searchRequestFromParameters(parameters) {
  const createdFrom = parameters.get("created_from") ?? "";
  const createdTo = parameters.get("created_to") ?? "";
  const updatedFrom = parameters.get("updated_from") ?? "";
  const updatedTo = parameters.get("updated_to") ?? "";
  return {
    modules: readListParameter(parameters, "modules"),
    object_types: readListParameter(parameters, "object_types"),
    statuses: readListParameter(parameters, "statuses"),
    tags: readListParameter(parameters, "tags"),
    created_at: {
      from: localDateToRFC3339(createdFrom),
      to: localDateToRFC3339(createdTo, true),
    },
    updated_at: {
      from: localDateToRFC3339(updatedFrom),
      to: localDateToRFC3339(updatedTo, true),
    },
    sort: {
      field: parameters.get("sort_field") || "updated_at",
      direction: parameters.get("sort_direction") || "desc",
    },
    limit: Number(parameters.get("limit") || defaultLimit),
  };
}

function compactSearchRequest(request) {
  return {
    ...request,
    modules: request.modules.length ? request.modules : undefined,
    object_types: request.object_types.length ? request.object_types : undefined,
    statuses: request.statuses.length ? request.statuses : undefined,
    tags: request.tags.length ? request.tags : undefined,
    created_at: request.created_at.from || request.created_at.to ? request.created_at : undefined,
    updated_at: request.updated_at.from || request.updated_at.to ? request.updated_at : undefined,
  };
}

function fillFormFromParameters(form, parameters) {
  setSelectValues(form.elements.modules, readListParameter(parameters, "modules"));
  setSelectValues(form.elements.object_types, readListParameter(parameters, "object_types"));
  form.elements.statuses.value = readListParameter(parameters, "statuses").join(", ");
  form.elements.tags.value = readListParameter(parameters, "tags").join(", ");
  form.elements.created_from.value = parameters.get("created_from") ?? "";
  form.elements.created_to.value = parameters.get("created_to") ?? "";
  form.elements.updated_from.value = parameters.get("updated_from") ?? "";
  form.elements.updated_to.value = parameters.get("updated_to") ?? "";
  form.elements.sort_field.value = parameters.get("sort_field") || "updated_at";
  form.elements.sort_direction.value = parameters.get("sort_direction") || "desc";
  form.elements.limit.value = parameters.get("limit") || String(defaultLimit);
}

export function renderSearchPage(target, _health, route) {
  const refCode = route?.searchParameters?.get("ref_code")?.trim() ?? "";
  if (refCode) {
    renderDetailView(target, refCode);
    return;
  }
  renderSearchView(target, route?.searchParameters ?? new URLSearchParams());
}

function renderSearchView(target, parameters) {
  let requestNumber = 0;
  const module = el("div", "search-module");
  const notice = renderNotice({
    title: "Search API",
    message: "Loading object metadata with the current filters.",
    tone: "info",
  });
  const noticeTitle = notice.querySelector("strong");
  const noticeMessage = notice.querySelector("p");

  const form = renderSurface("form", { className: "search-form", label: "Search object refs" });
  const formHead = el("header", "search-form__head");
  appendText(formHead, "span", "eyebrow", "Search / ObjectRefs");
  appendText(formHead, "h2", "search-form__title", "Object Metadata Search");
  appendText(formHead, "p", "search-form__text", "Query owner-visible ObjectRef metadata by module, type, status, tags and date range.");
  const fields = el("div", "control-stack search-form__fields");
  const scopeFields = el("div", "control-split");
  scopeFields.append(
    renderSelect({ label: "Modules", name: "modules", options: moduleOptions }),
    renderSelect({ label: "Object Types", name: "object_types", options: objectTypeOptions }),
  );
  const textFields = el("div", "control-split");
  textFields.append(
    renderField({ label: "Statuses / Comma Separated", name: "statuses", placeholder: "draft, active" }),
    renderField({ label: "Tags / Comma Separated", name: "tags", placeholder: "backend, release" }),
  );
  const createdFields = el("div", "control-split");
  createdFields.append(
    renderField({ label: "Created From", name: "created_from", type: "date" }),
    renderField({ label: "Created To", name: "created_to", type: "date" }),
  );
  const updatedFields = el("div", "control-split");
  updatedFields.append(
    renderField({ label: "Updated From", name: "updated_from", type: "date" }),
    renderField({ label: "Updated To", name: "updated_to", type: "date" }),
  );
  const resultFields = el("div", "control-split search-form__result-controls");
  resultFields.append(
    renderSelect({ label: "Sort Field", name: "sort_field", options: sortFieldOptions, value: "updated_at" }),
    renderSelect({ label: "Direction", name: "sort_direction", options: sortDirectionOptions, value: "desc" }),
    renderField({ label: "Limit", name: "limit", type: "number", value: String(defaultLimit), min: "1", max: "100" }),
  );
  fields.append(scopeFields, textFields, createdFields, updatedFields, resultFields);
  const actions = el("footer", "search-form__actions");
  const resetButton = renderButton("RESET");
  const searchButton = renderButton("SEARCH", { type: "submit", variant: "primary" });
  actions.append(resetButton, searchButton);
  form.append(formHead, fields, actions);
  fillFormFromParameters(form, parameters);

  const resultsPanel = renderSurface("section", { className: "section search-results-panel", label: "Search results" });
  const resultsHead = el("header", "search-results-head");
  const resultCount = appendText(resultsHead, "strong", "search-results-count", "0 results");
  const resultSummary = appendText(resultsHead, "span", "search-results-summary", "Current filters");
  resultsPanel.append(resultsHead);
  const table = renderTable("ObjectRef search results", [
    { label: "Ref" },
    { label: "Title" },
    { label: "Module" },
    { label: "Type" },
    { label: "Created" },
  ]);
  resultsPanel.append(table.element);

  module.append(form, resultsPanel);
  target.append(module);

  function setNotice(title, message, tone = "info") {
    noticeTitle.textContent = title;
    noticeMessage.textContent = message;
    notice.dataset.tone = tone;
  }

  function renderResults(objects) {
    resultCount.textContent = `${objects.length} result${objects.length === 1 ? "" : "s"}`;
    resultSummary.textContent = `Sorted by ${titleCase(form.elements.sort_field.value)} ${form.elements.sort_direction.value.toUpperCase()}`;
    if (objects.length === 0) {
      renderEmptyRow(table.body, 5, "No object metadata matched the current filters.");
      return;
    }
    table.body.replaceChildren(...objects.map((object) => {
      const row = el("tr", "search-result-row");
      row.setAttribute("aria-label", `Open ${object.ref_code}`);
      renderTableCell(row, object.ref_code, "ref-code");
      renderTableCell(row, object.title, "search-result-title");
      renderTableCell(row, titleCase(object.module));
      renderTableCell(row, titleCase(object.object_type));
      renderTableCell(row, formatDateTime(object.created_at), "search-date-cell");
      enableRow(row, () => {
        window.location.hash = detailHash(object.ref_code);
      });
      return row;
    }));
  }

  async function runSearch(nextParameters) {
    const currentRequest = ++requestNumber;
    fillFormFromParameters(form, nextParameters);
    const request = searchRequestFromParameters(nextParameters);
    setNotice("Search API", "Loading object metadata with the current filters.", "info");
    renderEmptyRow(table.body, 5, "Loading object metadata...");
    try {
      const objects = await postJSON("/api/platform/object-refs/search", compactSearchRequest(request));
      if (currentRequest !== requestNumber) {
        return;
      }
      renderResults(objects);
      setNotice("Search Complete", `Loaded ${objects.length} object metadata record${objects.length === 1 ? "" : "s"}.`, "info");
    } catch (error) {
      if (currentRequest !== requestNumber) {
        return;
      }
      renderEmptyRow(table.body, 5, error.body?.error?.message ?? error.message);
      setNotice("Search Failed", error.body?.error?.message ?? error.message, "warning");
    }
  }

  form.addEventListener("submit", (event) => {
    event.preventDefault();
    const nextParameters = buildParametersFromForm(form);
    const nextHash = nextParameters.toString() ? `#search?${nextParameters}` : "#search";
    if (window.location.hash === nextHash) {
      void runSearch(nextParameters);
      return;
    }
    window.location.hash = nextHash;
  });

  resetButton.addEventListener("click", () => {
    form.reset();
    if (window.location.hash === "#search" || window.location.hash === "") {
      void runSearch(new URLSearchParams());
      return;
    }
    window.location.hash = "#search";
  });

  void runSearch(parameters);
}

function renderDetailView(target, refCode) {
  const module = el("div", "search-module");
  const notice = renderNotice({
    title: "ObjectRef",
    message: `Loading ${refCode}.`,
    tone: "info",
  });
  const noticeTitle = notice.querySelector("strong");
  const noticeMessage = notice.querySelector("p");

  const detail = renderSurface("section", { className: "search-detail", label: "Object metadata detail" });
  const head = el("header", "search-detail__head");
  const copy = el("div", "search-detail__copy");
  const detailRef = appendText(copy, "span", "ref-code", refCode);
  const detailTitle = appendText(copy, "h2", "search-detail__title", "---");
  const badgeSlot = el("div", "search-detail__badge");
  head.append(copy, badgeSlot);

  const grid = el("div", "search-readonly-grid");
  const values = {
    module: renderReadonlyDetail("Module"),
    objectType: renderReadonlyDetail("Object Type"),
    createdAt: renderReadonlyDetail("Created"),
    updatedAt: renderReadonlyDetail("Updated"),
    tags: renderReadonlyDetail("Tags", "search-readonly--wide"),
    raw: renderReadonlyDetail("Raw Metadata", "search-readonly--wide search-readonly--raw"),
  };
  Object.values(values).forEach((item) => grid.append(item.element));
  const footer = el("footer", "search-detail__actions");
  const backButton = renderButton("RETURN", { label: "Return to search" });
  footer.append(backButton);
  detail.append(head, grid, footer);
  module.append(detail);
  target.append(module);

  function setNotice(title, message, tone = "info") {
    noticeTitle.textContent = title;
    noticeMessage.textContent = message;
    notice.dataset.tone = tone;
  }

  function renderMetadata(metadata) {
    detailRef.textContent = metadata.ref_code;
    detailTitle.textContent = metadata.title;
    badgeSlot.replaceChildren(renderStatusBadge(metadata.status, { state: statusTone(metadata.status) }));
    values.module.value.textContent = titleCase(metadata.module);
    values.objectType.value.textContent = titleCase(metadata.object_type);
    values.createdAt.value.textContent = formatDateTime(metadata.created_at);
    values.updatedAt.value.textContent = formatDateTime(metadata.updated_at);
    values.tags.value.replaceChildren(renderTagCollection(metadata.tags));
    values.raw.value.textContent = JSON.stringify(metadata, null, 2);
  }

  backButton.addEventListener("click", () => {
    window.location.hash = "#search";
  });

  getJSON(`/api/platform/object-refs/${encodeURIComponent(refCode)}`)
    .then((metadata) => {
      renderMetadata(metadata);
      setNotice("ObjectRef Loaded", `${metadata.ref_code} metadata is visible to the current owner.`, "info");
    })
    .catch((error) => {
      detailTitle.textContent = "Object not found";
      badgeSlot.replaceChildren(renderStatusBadge("unavailable", { state: "off" }));
      values.raw.value.textContent = JSON.stringify(error.body ?? {
        error: {
          code: "request_failed",
          message: error.message,
        },
      }, null, 2);
      setNotice("ObjectRef Failed", error.body?.error?.message ?? error.message, "warning");
    });
}
