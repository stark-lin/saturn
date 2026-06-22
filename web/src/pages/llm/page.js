// This file renders the authenticated LLM conversation and request workflow.
import { deleteJSON, getJSON, postJSON } from "../../shared/api/client.js";
import {
  renderButton,
  renderNotice,
  renderStatusBadge,
  renderSurface,
  renderTag,
} from "../../shared/components/primitives.js";
import { el } from "../../shared/utils/dom.js";
import { renderMarkdown } from "../../shared/utils/markdown.js";

const sessionPageSize = 10;
const requestChunkSize = 100;

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
    return "--";
  }
  return `${date.toLocaleDateString("en-CA")} ${date.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  })}`;
}

function splitReferences(value) {
  const references = new Set();
  String(value ?? "")
    .split(/[\s,]+/)
    .map((item) => item.trim().toUpperCase())
    .filter(Boolean)
    .forEach((item) => references.add(item));
  return Array.from(references);
}

function splitTags(value) {
  const tags = new Set();
  String(value ?? "")
    .split(",")
    .map((tag) => tag.trim())
    .filter(Boolean)
    .forEach((tag) => tags.add(tag));
  return Array.from(tags);
}

function renderControlLabel(text) {
  const label = el("span", "control-label");
  label.textContent = text;
  return label;
}

function renderField({ label, name, type = "text", placeholder = "", value = "", min = "", required = false }) {
  const field = el("label", "control-field");
  const input = el("input", "field");
  input.name = name;
  input.type = type;
  input.placeholder = placeholder;
  input.value = value;
  input.required = required;
  if (min !== "") {
    input.min = String(min);
  }
  field.append(renderControlLabel(label), input);
  return field;
}

function renderTextarea({ label, name, placeholder = "", rows = 5, required = false }) {
  const field = el("label", "control-field");
  const textarea = el("textarea", "textarea");
  textarea.name = name;
  textarea.placeholder = placeholder;
  textarea.rows = rows;
  textarea.required = required;
  field.append(renderControlLabel(label), textarea);
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

function renderTableCell(row, text, className = "", { title = "" } = {}) {
  const cell = document.createElement("td");
  cell.className = className;
  cell.textContent = text;
  if (title) {
    cell.title = title;
  }
  row.append(cell);
  return cell;
}

function renderEmptyRow(body, columnCount, text) {
  const row = document.createElement("tr");
  const cell = renderTableCell(row, text, "llm-empty-cell");
  cell.colSpan = columnCount;
  body.replaceChildren(row);
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

function renderPager({ currentPage, hasPrevious, hasNext, onPage }) {
  const pager = el("nav", "pager");
  pager.setAttribute("aria-label", "LLM conversations pagination");
  const previous = el("button", "page-button");
  previous.type = "button";
  previous.textContent = "<-";
  previous.disabled = !hasPrevious;
  previous.setAttribute("aria-label", "Previous conversation page");
  previous.addEventListener("click", () => onPage(currentPage - 1));
  const current = el("button", "page-button");
  current.type = "button";
  current.textContent = String(currentPage);
  current.setAttribute("aria-current", "page");
  const next = el("button", "page-button");
  next.type = "button";
  next.textContent = "->";
  next.disabled = !hasNext;
  next.setAttribute("aria-label", "Next conversation page");
  next.addEventListener("click", () => onPage(currentPage + 1));
  pager.append(previous, current, next);
  return pager;
}

function renderStat(label) {
  const card = renderSurface("article", { className: "llm-stat" });
  appendText(card, "span", "", label);
  const value = appendText(card, "strong", "", "--");
  const note = appendText(card, "p", "", "");
  return { card, value, note };
}

function responseBadge(request) {
  const status = request?.response_status ?? "queued";
  const state = status === "success" ? "online" : status === "error" ? "warning" : "off";
  return renderStatusBadge(titleCase(status), { state });
}

function isPendingRequest(request) {
  return request?.response_status === "queued" || request?.response_status === "running";
}

function truncateListText(value, fallback, maxLength = 72) {
  const text = String(value ?? "").trim().replace(/\s+/g, " ");
  if (!text) {
    return { text: fallback, title: "" };
  }
  return {
    text: text.length > maxLength ? `${text.slice(0, maxLength - 3)}...` : text,
    title: text,
  };
}

function renderReferenceCollection(references, { emptyLabel = "no references" } = {}) {
  const collection = el("div", "llm-tags");
  if (!references?.length) {
    collection.append(renderTag(emptyLabel));
    return collection;
  }

  references.forEach((reference) => {
    const link = document.createElement("a");
    link.className = "tag llm-reference";
    link.href = `#search?ref_code=${encodeURIComponent(reference.ref_code)}`;
    link.title = [reference.module, reference.object_type, reference.status].filter(Boolean).join(" / ");
    link.textContent = reference.title
      ? `${reference.ref_code} / ${reference.title}`
      : reference.ref_code;
    link.addEventListener("click", (event) => event.stopPropagation());
    collection.append(link);
    (reference.tags ?? []).forEach((tag) => collection.append(renderTag(tag)));
  });
  return collection;
}

function renderTagCollection(tags, { emptyLabel = "untagged" } = {}) {
  const collection = el("div", "llm-tags");
  (tags?.length ? tags : [emptyLabel]).forEach((tag) => collection.append(renderTag(tag)));
  return collection;
}

function renderResponseBody(target, request) {
  target.replaceChildren();
  if (request?.response_status === "error") {
    const error = el("p", "llm-response-error");
    error.textContent = [request.error_code, request.error_message].filter(Boolean).join(": ") || "LLM request failed.";
    target.append(error);
    return;
  }

  if (isPendingRequest(request)) {
    renderMarkdown(target, request.response_status === "running" ? "_Generating response._" : "_Queued for processing._");
    return;
  }

  renderMarkdown(target, request?.content?.trim() || "_No content returned._");
}

export function renderLLMPage(target, _health, route) {
  const state = {
    sessions: [],
    selectedSession: null,
    requests: [],
    currentRequest: null,
    currentPage: 1,
    hasMore: false,
    listRequestNumber: 0,
    detailRequestNumber: 0,
    requestDetailRequestNumber: 0,
    pollTimer: 0,
  };

  const module = el("div", "llm-module");
  const notice = renderNotice({
    title: "LLM API",
    message: "Loading your private conversations.",
    tone: "info",
  });
  notice.classList.add("llm-notice");
  const noticeTitle = notice.querySelector("strong");
  const noticeMessage = notice.querySelector("p");

  const sessionsView = el("section", "llm-view");
  sessionsView.setAttribute("aria-label", "LLM conversations");
  const summary = el("section", "llm-summary-grid");
  summary.setAttribute("aria-label", "LLM conversation summary");
  const pageSessions = renderStat("Page Conversations");
  const latestSession = renderStat("Latest Conversation");
  const pageState = renderStat("Conversation Page");
  summary.append(pageSessions.card, latestSession.card, pageState.card);

  const sessionsPanel = renderSurface("section", { className: "section llm-panel", label: "LLM conversations" });
  const sessionsTable = renderTable("LLM conversations", [
    { label: "Ref", className: "llm-list-ref" },
    { label: "Title", className: "llm-list-title" },
    { label: "Tags", className: "llm-list-tags" },
    { label: "Updated", className: "llm-list-date" },
  ]);
  const sessionsFooter = el("footer", "llm-footer");
  const sessionPagerSlot = el("div", "llm-pager-slot");
  const newSessionButton = renderButton("NEW", { variant: "primary", label: "New LLM conversation" });
  sessionsFooter.append(sessionPagerSlot, newSessionButton);
  sessionsPanel.append(sessionsTable.element, sessionsFooter);
  sessionsView.append(summary, sessionsPanel);

  const newSessionView = el("section", "llm-view");
  newSessionView.hidden = true;
  newSessionView.setAttribute("aria-label", "New LLM conversation");
  const sessionForm = renderSurface("form", { className: "llm-form", label: "Create LLM conversation" });
  const sessionFormHeader = el("header", "llm-form__head");
  appendText(sessionFormHeader, "span", "eyebrow", "LLM / New");
  appendText(sessionFormHeader, "h2", "llm-form__title", "Create Conversation");
  appendText(sessionFormHeader, "p", "llm-form__text", "Conversations keep immutable prompts, authorized references and provider results together.");
  const sessionFields = el("div", "control-stack llm-form__fields");
  sessionFields.append(renderField({
    label: "Conversation Title",
    name: "title",
    placeholder: "Monthly Review",
    required: true,
  }));
  sessionFields.append(renderField({
    label: "Tags / Comma Separated",
    name: "tags",
    placeholder: "review, planning",
  }));
  const sessionActions = el("footer", "llm-form__actions");
  const cancelSessionButton = renderButton("RETURN");
  const saveSessionButton = renderButton("SAVE", { type: "submit", variant: "primary" });
  sessionActions.append(cancelSessionButton, saveSessionButton);
  sessionForm.append(sessionFormHeader, sessionFields, sessionActions);
  newSessionView.append(sessionForm);

  const sessionView = el("section", "llm-view");
  sessionView.hidden = true;
  sessionView.setAttribute("aria-label", "LLM conversation detail");
  const sessionInfoGrid = el("section", "llm-session-info-grid");
  sessionInfoGrid.setAttribute("aria-label", "Selected LLM conversation summary");
  const sessionIdentity = renderSurface("article", { className: "llm-stat llm-session-identity" });
  appendText(sessionIdentity, "span", "", "Conversation");
  const detailTitle = appendText(sessionIdentity, "strong", "llm-session-title", "---");
  const sessionMeta = el("div", "llm-session-meta");
  const detailRef = appendText(sessionMeta, "span", "ref-code llm-session-ref", "---");
  sessionIdentity.append(sessionMeta);
  const sessionTagsSlot = el("div", "llm-tags");
  sessionIdentity.append(sessionTagsSlot);
  const requestCount = renderStat("Requests");
  const lastResponse = renderStat("Last Response");
  sessionInfoGrid.append(sessionIdentity, requestCount.card, lastResponse.card);

  const requestsPanel = renderSurface("section", { className: "section llm-panel", label: "LLM requests" });
  const requestsTable = renderTable("Selected LLM conversation requests", [
    { label: "Ref", className: "llm-list-ref" },
    { label: "Prompt", className: "llm-list-prompt" },
    { label: "Status", className: "llm-list-status" },
    { label: "Tags", className: "llm-list-tags" },
    { label: "Updated", className: "llm-list-date" },
  ]);
  const requestsFooter = el("footer", "llm-footer");
  const requestActions = el("div", "actions llm-session-actions");
  const backToSessionsButton = renderButton("RETURN", { label: "Return to LLM conversations" });
  const newRequestButton = renderButton("NEW", { variant: "primary", label: "New LLM request" });
  const deleteSessionButton = renderButton("DELETE", { variant: "danger", label: "Delete LLM conversation" });
  requestActions.append(backToSessionsButton, newRequestButton, deleteSessionButton);
  requestsFooter.append(requestActions);
  requestsPanel.append(requestsTable.element, requestsFooter);
  sessionView.append(sessionInfoGrid, requestsPanel);

  const newRequestView = el("section", "llm-view");
  newRequestView.hidden = true;
  newRequestView.setAttribute("aria-label", "New LLM request");
  const requestForm = renderSurface("form", { className: "llm-form", label: "Create LLM request" });
  const requestFormHeader = el("header", "llm-form__head");
  appendText(requestFormHeader, "span", "eyebrow", "LLM / Request");
  const requestFormTitle = appendText(requestFormHeader, "h2", "llm-form__title", "Ask With References");
  appendText(requestFormHeader, "p", "llm-form__text", "References are resolved by the backend through normal module services before the provider is called.");
  const requestFields = el("div", "control-stack llm-form__fields");
  const promptField = renderTextarea({
    label: "Prompt",
    name: "prompt",
    placeholder: "Summarize ACC-00000001 and call out notable changes.",
    rows: 7,
    required: true,
  });
  const referencesField = renderField({
    label: "References / Comma Or Space Separated",
    name: "references",
    placeholder: "ACC-00000001 NTE-00000002",
  });
  const requestTagsField = renderField({
    label: "Tags / Comma Separated",
    name: "tags",
    placeholder: "summary, monthly",
  });
  const requestOptions = el("div", "control-split");
  requestOptions.append(
    renderField({ label: "Model / Optional", name: "model", placeholder: "Default" }),
    renderField({ label: "Max Tokens / Optional", name: "max_tokens", type: "number", min: 0, placeholder: "Default" }),
  );
  requestFields.append(promptField, referencesField, requestTagsField, requestOptions);
  const requestFormActions = el("footer", "llm-form__actions");
  const cancelRequestButton = renderButton("RETURN");
  const sendRequestButton = renderButton("SEND", { type: "submit", variant: "primary" });
  requestFormActions.append(cancelRequestButton, sendRequestButton);
  requestForm.append(requestFormHeader, requestFields, requestFormActions);
  newRequestView.append(requestForm);

  const requestDetailView = el("section", "llm-view");
  requestDetailView.hidden = true;
  requestDetailView.setAttribute("aria-label", "LLM request detail");
  const requestDetail = renderSurface("section", { className: "llm-request-detail", label: "LLM request detail" });
  const requestDetailHead = el("header", "llm-detail__head");
  const requestDetailCopy = el("div", "llm-detail__copy");
  const requestDetailRef = appendText(requestDetailCopy, "span", "ref-code", "---");
  const requestDetailTitle = appendText(requestDetailCopy, "h2", "llm-detail__title", "---");
  const requestDetailBadgeSlot = el("div", "llm-detail__badge");
  requestDetailHead.append(requestDetailCopy, requestDetailBadgeSlot);
  const requestDetailGrid = el("div", "llm-readonly-grid");
  const requestDetailValues = {
    session: renderReadonlyDetail("Conversation"),
    model: renderReadonlyDetail("Model"),
    maxTokens: renderReadonlyDetail("Max Tokens"),
    created: renderReadonlyDetail("Created"),
    completed: renderReadonlyDetail("Completed"),
    tags: renderReadonlyDetail("Tags", "llm-readonly--wide"),
    references: renderReadonlyDetail("References", "llm-readonly--wide"),
    prompt: renderReadonlyDetail("Prompt", "llm-readonly--wide llm-readonly--note"),
    response: renderReadonlyDetail("Response", "llm-readonly--wide llm-readonly--response"),
  };
  Object.values(requestDetailValues).forEach((item) => requestDetailGrid.append(item.element));
  const requestDetailFooter = el("footer", "llm-footer");
  const requestDetailActions = el("div", "actions llm-session-actions");
  const backToSessionButton = renderButton("RETURN", { label: "Return to LLM conversation" });
  requestDetailActions.append(backToSessionButton);
  requestDetailFooter.append(requestDetailActions);
  requestDetail.append(requestDetailHead, requestDetailGrid, requestDetailFooter);
  requestDetailView.append(requestDetail);

  module.append(sessionsView, newSessionView, sessionView, newRequestView, requestDetailView);
  target.append(module);

  function renderReadonlyDetail(label, className = "") {
    const element = el("div", `llm-readonly ${className}`.trim());
    element.append(renderControlLabel(label));
    const value = appendText(element, "div", "llm-readonly__value", "---");
    return { element, value };
  }

  function setNotice(title, message, tone = "info") {
    noticeTitle.textContent = title;
    noticeMessage.textContent = message;
    notice.dataset.tone = tone;
  }

  function setView(view) {
    sessionsView.hidden = view !== "sessions";
    newSessionView.hidden = view !== "new-session";
    sessionView.hidden = view !== "session";
    newRequestView.hidden = view !== "new-request";
    requestDetailView.hidden = view !== "request-detail";
    if (view !== "session" && view !== "request-detail") {
      stopPolling();
    }
  }

  function setFormBusy(form, busy) {
    Array.from(form.elements).forEach((element) => {
      element.disabled = busy;
    });
  }

  function renderOverview() {
    pageSessions.value.textContent = String(state.sessions.length);
    pageSessions.note.textContent = state.sessions.length === 1
      ? "One conversation on the current page."
      : "Conversations on the current page.";

    const latest = state.sessions[0];
    latestSession.value.textContent = latest?.title || latest?.ref_code || "--";
    latestSession.note.textContent = latest
      ? `${latest.ref_code} / updated ${formatDateTime(latest.updated_at)}`
      : "Create a conversation to start asking over authorized context.";

    pageState.value.textContent = `Page ${state.currentPage}`;
    pageState.note.textContent = state.hasMore ? "More conversations are available." : "No more conversations after this page.";
  }

  function renderSessions() {
    if (state.sessions.length === 0) {
      renderEmptyRow(sessionsTable.body, 4, "No LLM conversations found. Create a conversation to start asking over your local data.");
    } else {
      sessionsTable.body.replaceChildren(...state.sessions.map((session) => {
        const row = el("tr", "llm-row");
        row.setAttribute("aria-label", `Open ${session.title} LLM conversation`);
        const title = truncateListText(session.title, session.ref_code, 80);
        renderTableCell(row, session.ref_code, "ref-code llm-list-ref");
        renderTableCell(row, title.text, "llm-name llm-list-title", { title: title.title });
        const tagsCell = document.createElement("td");
        tagsCell.className = "llm-list-tags";
        tagsCell.append(renderTagCollection(session.tags));
        row.append(tagsCell);
        renderTableCell(row, formatDateTime(session.updated_at), "llm-list-date");
        enableRow(row, () => void openSession(session.ref_code));
        return row;
      }));
    }
    sessionPagerSlot.replaceChildren(renderPager({
      currentPage: state.currentPage,
      hasPrevious: state.currentPage > 1,
      hasNext: state.hasMore,
      onPage(page) {
        void loadSessions(page);
      },
    }));
  }

  function renderSessionDetail() {
    const session = state.selectedSession;
    if (!session) {
      return;
    }
    const latestRequest = state.requests[state.requests.length - 1];
    detailRef.textContent = session.ref_code;
    detailTitle.textContent = session.title;
    sessionTagsSlot.replaceChildren(renderTagCollection(session.tags));
    requestCount.value.textContent = String(state.requests.length);
    requestCount.note.textContent = `Immutable requests. Updated ${formatDateTime(session.updated_at)}.`;
    lastResponse.value.textContent = latestRequest ? titleCase(latestRequest.response_status) : "--";
    lastResponse.note.textContent = latestRequest
      ? `${latestRequest.ref_code ?? "---"} / ${formatDateTime(latestRequest.completed_at ?? latestRequest.updated_at)}`
      : "No requests have been submitted yet.";

    if (state.requests.length === 0) {
      renderEmptyRow(requestsTable.body, 5, "No requests found for this conversation.");
      return;
    }

    requestsTable.body.replaceChildren(...state.requests.map((request, index) => {
      const row = el("tr", "llm-row");
      row.setAttribute("aria-label", `Open ${request.ref_code} LLM request`);
      const promptTitle = truncateListText(request.prompt, `Request ${index + 1}`, 96);
      renderTableCell(row, request.ref_code, "ref-code llm-list-ref");
      renderTableCell(row, promptTitle.text, "llm-name llm-list-prompt", { title: promptTitle.title });
      renderTableCell(row, titleCase(request.status || "pending"), "llm-list-status");
      const tagsCell = document.createElement("td");
      tagsCell.className = "llm-list-tags";
      tagsCell.append(renderTagCollection(request.tags));
      row.append(tagsCell);
      renderTableCell(row, formatDateTime(request.completed_at ?? request.updated_at), "llm-list-date");
      enableRow(row, () => void openRequest(request.ref_code));
      return row;
    }));
  }

  function renderRequestDetail() {
    const request = state.currentRequest;
    const session = state.selectedSession;
    if (!request || !session) {
      return;
    }
    requestDetailRef.textContent = request.ref_code;
    requestDetailTitle.textContent = "Request Detail";
    requestDetailBadgeSlot.replaceChildren(responseBadge(request));
    requestDetailValues.session.value.textContent = `${session.title} / ${session.ref_code}`;
    requestDetailValues.model.value.textContent = request.model || "Default";
    requestDetailValues.maxTokens.value.textContent = request.max_tokens ? String(request.max_tokens) : "Default";
    requestDetailValues.created.value.textContent = formatDateTime(request.created_at);
    requestDetailValues.completed.value.textContent = formatDateTime(request.completed_at ?? request.updated_at);
    requestDetailValues.tags.value.replaceChildren(renderTagCollection(request.tags));
    requestDetailValues.references.value.replaceChildren(
      renderReferenceCollection(request.references ?? [], { emptyLabel: "no references" }),
    );
    requestDetailValues.prompt.value.textContent = request.prompt || "--";
    renderResponseBody(requestDetailValues.response.value, request);
  }

  function stopPolling() {
    if (state.pollTimer) {
      window.clearTimeout(state.pollTimer);
      state.pollTimer = 0;
    }
  }

  function isSessionContextVisible() {
    return !sessionView.hidden || !requestDetailView.hidden;
  }

  function mergeRequest(updatedRequest) {
    if (!updatedRequest?.ref_code) {
      return;
    }
    const index = state.requests.findIndex((request) => request.ref_code === updatedRequest.ref_code);
    if (index === -1) {
      state.requests.push(updatedRequest);
    } else {
      state.requests[index] = updatedRequest;
    }
    if (state.currentRequest?.ref_code === updatedRequest.ref_code) {
      state.currentRequest = updatedRequest;
    }
  }

  function scheduleRequestPolling() {
    stopPolling();
    const sessionRefCode = state.selectedSession?.ref_code;
    if (!sessionRefCode || !isSessionContextVisible()) {
      return;
    }
    const pendingRequests = state.requests.filter(isPendingRequest);
    if (pendingRequests.length === 0) {
      return;
    }
    state.pollTimer = window.setTimeout(async () => {
      if (state.selectedSession?.ref_code !== sessionRefCode || !isSessionContextVisible()) {
        return;
      }
      try {
        const results = await Promise.all(pendingRequests.map((request) => (
          getJSON(`/api/llm/requests/${encodeURIComponent(request.ref_code)}`)
        )));
        if (state.selectedSession?.ref_code !== sessionRefCode || !isSessionContextVisible()) {
          return;
        }
        results.forEach((result) => mergeRequest(result.request));
        renderSessionDetail();
        if (!requestDetailView.hidden) {
          renderRequestDetail();
        }
        if (!state.requests.some(isPendingRequest)) {
          setNotice("LLM Response Ready", "All queued LLM requests in this conversation have finished.", "info");
        }
      } catch (error) {
        if (state.selectedSession?.ref_code === sessionRefCode && isSessionContextVisible()) {
          setNotice("Unable to Refresh LLM Request", error.message, "warning");
        }
      } finally {
        scheduleRequestPolling();
      }
    }, 1000);
  }

  async function loadSessions(page, { announce = true } = {}) {
    const requestNumber = ++state.listRequestNumber;
    const offset = (page - 1) * sessionPageSize;
    if (announce) {
      setNotice("LLM API", `Loading conversation page ${page}.`, "info");
    }
    try {
      const result = await getJSON(`/api/llm/sessions?limit=${sessionPageSize}&offset=${offset}`);
      if (requestNumber !== state.listRequestNumber) {
        return false;
      }
      if ((result.sessions ?? []).length === 0 && page > 1) {
        return loadSessions(page - 1, { announce });
      }
      state.sessions = result.sessions ?? [];
      state.currentPage = page;
      state.hasMore = Boolean(result.has_more);
      renderOverview();
      renderSessions();
      if (announce) {
        setNotice("LLM API", `Showing up to ${sessionPageSize} LLM conversations per page.`, "info");
      }
      return true;
    } catch (error) {
      if (requestNumber !== state.listRequestNumber) {
        return false;
      }
      state.sessions = [];
      state.currentPage = 1;
      state.hasMore = false;
      renderOverview();
      renderSessions();
      setNotice("Unable to Load LLM Conversations", error.message, "warning");
      return false;
    }
  }

  async function getSessionDetail(refCode) {
    const requests = [];
    let session = null;
    let offset = 0;
    while (true) {
      const result = await getJSON(
        `/api/llm/sessions/${encodeURIComponent(refCode)}?limit=${requestChunkSize}&offset=${offset}`,
      );
      session = result.session;
      const pageRequests = result.requests ?? [];
      requests.push(...pageRequests);
      if (pageRequests.length < requestChunkSize || pageRequests.length === 0) {
        break;
      }
      offset += requestChunkSize;
    }
    return { session, requests };
  }

  async function openSession(refCode, { announce = true } = {}) {
    const requestNumber = ++state.detailRequestNumber;
    state.currentRequest = null;
    setView("session");
    if (announce) {
      setNotice("LLM API", `Loading conversation ${refCode}.`, "info");
    }
    try {
      const detail = await getSessionDetail(refCode);
      if (requestNumber !== state.detailRequestNumber) {
        return false;
      }
      state.selectedSession = detail.session;
      state.requests = detail.requests;
      renderSessionDetail();
      scheduleRequestPolling();
      if (announce) {
        setNotice("LLM Conversation", `Showing ${detail.requests.length} request${detail.requests.length === 1 ? "" : "s"} for ${detail.session.title}.`, "info");
      }
      return true;
    } catch (error) {
      if (requestNumber !== state.detailRequestNumber) {
        return false;
      }
      setNotice("Unable to Load LLM Conversation", error.message, "warning");
      if (!state.selectedSession) {
        setView("sessions");
      }
      return false;
    }
  }

  async function openRequest(refCode, { announce = true } = {}) {
    const requestNumber = ++state.requestDetailRequestNumber;
    const existingRequest = state.requests.find((request) => request.ref_code === refCode);
    if (existingRequest) {
      state.currentRequest = existingRequest;
      renderRequestDetail();
    }
    setView("request-detail");
    if (announce) {
      setNotice("LLM API", `Loading request ${refCode}.`, "info");
    }
    try {
      const result = await getJSON(`/api/llm/requests/${encodeURIComponent(refCode)}`);
      if (requestNumber !== state.requestDetailRequestNumber) {
        return false;
      }
      state.currentRequest = result.request;
      mergeRequest(result.request);
      renderSessionDetail();
      renderRequestDetail();
      scheduleRequestPolling();
      if (announce) {
        setNotice("LLM Request", `${refCode} detail is ready.`, "info");
      }
      return true;
    } catch (error) {
      if (requestNumber !== state.requestDetailRequestNumber) {
        return false;
      }
      setNotice("Unable to Load LLM Request", error.message, "warning");
      if (!state.currentRequest) {
        setView("session");
      }
      return false;
    }
  }

  function showSessionForm() {
    sessionForm.reset();
    setView("new-session");
    setNotice("New LLM Conversation", "Create a conversation before submitting prompts and references.", "info");
  }

  function showRequestForm() {
    const session = state.selectedSession;
    if (!session) {
      return;
    }
    requestForm.reset();
    requestFormTitle.textContent = `Ask With References / ${session.title}`;
    setView("new-request");
    setNotice("New LLM Request", "Prompts are immutable once queued.", "info");
  }

  async function createSession(event) {
    event.preventDefault();
    const title = sessionForm.elements.title.value.trim();
    if (!title) {
      setNotice("Unable to Create Conversation", "Conversation title is required.", "warning");
      return;
    }
    setFormBusy(sessionForm, true);
    try {
      const result = await postJSON("/api/llm/sessions", {
        title,
        tags: splitTags(sessionForm.elements.tags.value),
      });
      await loadSessions(1, { announce: false });
      await openSession(result.session.ref_code, { announce: false });
      setNotice("Conversation Created", `${result.session.title} is ready for prompts.`, "info");
    } catch (error) {
      setNotice("Unable to Create Conversation", error.message, "warning");
    } finally {
      setFormBusy(sessionForm, false);
    }
  }

  async function createRequest(event) {
    event.preventDefault();
    const session = state.selectedSession;
    if (!session) {
      return;
    }
    const prompt = requestForm.elements.prompt.value.trim();
    if (!prompt) {
      setNotice("Unable to Send Request", "Prompt is required.", "warning");
      return;
    }
    const maxTokensValue = requestForm.elements.max_tokens.value.trim();
    const maxTokens = maxTokensValue ? Number(maxTokensValue) : 0;
    if (!Number.isInteger(maxTokens) || maxTokens < 0) {
      setNotice("Unable to Send Request", "Max tokens must be a positive integer or blank.", "warning");
      return;
    }
    setFormBusy(requestForm, true);
    setNotice("LLM Request", "Submitting prompt to the queue.", "info");
    try {
      const result = await postJSON(`/api/llm/sessions/${encodeURIComponent(session.ref_code)}/requests`, {
        prompt,
        references: splitReferences(requestForm.elements.references.value),
        model: requestForm.elements.model.value.trim(),
        max_tokens: maxTokens,
        tags: splitTags(requestForm.elements.tags.value),
      });
      requestForm.reset();
      await openSession(session.ref_code, { announce: false });
      const createdRequest = result.request;
      if (createdRequest?.ref_code) {
        mergeRequest(createdRequest);
        state.currentRequest = createdRequest;
        setView("request-detail");
        renderSessionDetail();
        renderRequestDetail();
        scheduleRequestPolling();
      }
      setNotice("LLM Request Queued", `${createdRequest?.ref_code ?? "Request"} is queued for processing.`, "info");
    } catch (error) {
      setNotice("Unable to Send Request", error.message, "warning");
    } finally {
      setFormBusy(requestForm, false);
    }
  }

  async function deleteCurrentSession() {
    const session = state.selectedSession;
    if (!session) {
      return;
    }
    if (!window.confirm(`Delete ${session.ref_code} and all of its requests?`)) {
      return;
    }
    deleteSessionButton.disabled = true;
    stopPolling();
    setNotice("Deleting LLM Conversation", `Deleting ${session.ref_code}.`, "info");
    try {
      await deleteJSON(`/api/llm/sessions/${encodeURIComponent(session.ref_code)}`);
      state.selectedSession = null;
      state.currentRequest = null;
      state.requests = [];
      setView("sessions");
      await loadSessions(state.currentPage, { announce: false });
      setNotice("LLM Conversation Deleted", `${session.ref_code} and its requests were deleted.`, "info");
    } catch (error) {
      setNotice("Unable to Delete LLM Conversation", error.message, "warning");
    } finally {
      deleteSessionButton.disabled = false;
    }
  }

  function returnToSessions() {
    state.currentRequest = null;
    setView("sessions");
    setNotice("LLM API", "Showing your private LLM conversations.", "info");
  }

  function returnToSelectedSession() {
    setView("session");
    renderSessionDetail();
    scheduleRequestPolling();
    setNotice("LLM Conversation", `Showing requests for ${state.selectedSession?.title ?? "this conversation"}.`, "info");
  }

  newSessionButton.addEventListener("click", showSessionForm);
  cancelSessionButton.addEventListener("click", returnToSessions);
  sessionForm.addEventListener("submit", (event) => void createSession(event));
  backToSessionsButton.addEventListener("click", returnToSessions);
  newRequestButton.addEventListener("click", showRequestForm);
  deleteSessionButton.addEventListener("click", () => void deleteCurrentSession());
  cancelRequestButton.addEventListener("click", returnToSelectedSession);
  requestForm.addEventListener("submit", (event) => void createRequest(event));
  backToSessionButton.addEventListener("click", returnToSelectedSession);

  setView("sessions");
  void loadSessions(1).then(() => {
    const requestedSession = route?.searchParameters?.get("session")
      || route?.searchParameters?.get("ref_code")
      || "";
    if (requestedSession) {
      void openSession(requestedSession.trim().toUpperCase());
    }
  });
}
