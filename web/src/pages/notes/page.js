// This file renders the owner-only Notes API workflow and safe Markdown preview.
import { deleteJSON, getJSON, patchJSON, postJSON } from "../../shared/api/client.js";
import { renderButton, renderNotice, renderStatusBadge, renderSurface, renderTag } from "../../shared/components/primitives.js";
import { el } from "../../shared/utils/dom.js";
import { parseNoteMarkdown, renderMarkdown } from "../../shared/utils/markdown.js";

const pageSize = 10;
const newNoteMarkdown = "Untitled Note\n\nStart writing your Markdown body here.";

function formatDate(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "--";
  }
  return date.toLocaleDateString("en-CA", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  });
}

function formatDateTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "--";
  }
  return date.toLocaleString("en-CA", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function versionOperationLabel(operation) {
  switch (operation) {
    case "create":
      return "Created";
    case "restore":
      return "Restored";
    default:
      return "Updated";
  }
}

function renderNoteCard(note, onOpen) {
  const card = el("button", "note-card");
  card.type = "button";
  card.addEventListener("click", () => onOpen(note.ref_code));

  const main = el("span", "note-card__main");
  const refCode = el("span", "ref-code");
  refCode.textContent = note.ref_code;
  const title = el("span", "note-card__title");
  title.textContent = note.title || "Untitled Note";
  const excerpt = el("span", "note-card__excerpt");
  excerpt.textContent = "Open to read or edit the Markdown source.";
  const tags = el("span", "note-card__tags");
  (note.tags ?? []).forEach((tag) => tags.append(renderTag(tag)));
  main.append(refCode, title, excerpt, tags);

  const side = el("span", "note-card__side");
  const updated = el("span", "note-updated");
  updated.textContent = `Updated ${formatDate(note.updated_at)}`;
  side.append(updated, renderStatusBadge("Saved", { state: "off" }));

  card.append(main, side);
  return card;
}

function renderPreview(target, markdown) {
  const parsed = parseNoteMarkdown(markdown);
  const header = el("header", "note-preview__head");
  const title = el("h1", "note-preview__title");
  title.textContent = parsed.title || "Untitled Note";
  const tags = el("div", "note-preview__tags");
  tags.setAttribute("aria-label", "Note tags");
  if (parsed.tags.length === 0) {
    tags.append(renderTag("untagged"));
  } else {
    parsed.tags.forEach((tag) => tags.append(renderTag(tag)));
  }
  header.append(title, tags);

  const body = el("div", "note-preview__body");
  renderMarkdown(body, parsed.body.trim() || "_No body content yet._");
  target.replaceChildren(header, body);
}

function renderPager({ currentPage, hasPrevious, hasNext, onPage }) {
  const pager = el("nav", "pager");
  pager.setAttribute("aria-label", "Notes pagination");

  const previous = el("button", "page-button");
  previous.type = "button";
  previous.textContent = "<-";
  previous.disabled = !hasPrevious;
  previous.setAttribute("aria-label", "Previous note page");
  previous.addEventListener("click", () => onPage(currentPage - 1));

  const current = el("button", "page-button");
  current.type = "button";
  current.textContent = String(currentPage);
  current.setAttribute("aria-current", "page");

  const next = el("button", "page-button");
  next.type = "button";
  next.textContent = "->";
  next.disabled = !hasNext;
  next.setAttribute("aria-label", "Next note page");
  next.addEventListener("click", () => onPage(currentPage + 1));

  pager.append(previous, current, next);
  return pager;
}

function renderVersionCard(version, currentVersionRef, onOpen) {
  const card = el("button", "note-history-card");
  card.type = "button";
  card.setAttribute("aria-label", `Open version ${version.version_number}`);
  card.addEventListener("click", () => onOpen(version.ref_code));

  const heading = el("span", "note-history-card__heading");
  const versionNumber = el("strong", "note-history-card__version");
  versionNumber.textContent = `V${version.version_number}`;
  const operation = el("span", "note-history-card__operation");
  operation.textContent = versionOperationLabel(version.operation);
  heading.append(versionNumber, operation);
  if (version.ref_code === currentVersionRef) {
    heading.append(renderStatusBadge("Current", { state: "off" }));
  }

  const title = el("span", "note-history-card__title");
  title.textContent = version.title || "Untitled Note";
  const metadata = el("span", "note-history-card__meta");
  metadata.textContent = `${version.ref_code} · ${formatDateTime(version.created_at)}`;
  card.append(heading, title, metadata);
  return card;
}

export function renderNotesPage(target) {
  const state = {
    notes: [],
    currentNote: null,
    currentPage: 1,
    hasMore: false,
    draftMarkdown: "",
    dirty: false,
    editorBusy: false,
    mutating: false,
    editorMode: "edit",
    historyVersions: [],
    selectedVersionRef: null,
    listRequestNumber: 0,
    detailRequestNumber: 0,
    historyRequestNumber: 0,
    versionRequestNumber: 0,
  };

  const module = renderSurface("section", { className: "section notes-module", label: "Notes" });
  module.dataset.view = "list";

  const notice = renderNotice({
    title: "Notes API",
    message: "Loading your private notes.",
    tone: "info",
  });
  notice.classList.add("notes-api-notice");
  const noticeTitle = notice.querySelector("strong");
  const noticeMessage = notice.querySelector("p");

  const listView = el("section", "notes-list-view");
  listView.setAttribute("aria-label", "Note list view");
  const list = el("div", "notes-grid");
  list.setAttribute("aria-label", "Notes");

  const empty = renderSurface("div", { className: "notes-empty", raised: true });
  const emptyTitle = document.createElement("strong");
  emptyTitle.textContent = "No Notes Found";
  const emptyMessage = document.createElement("p");
  emptyMessage.textContent = "Create a note to store Markdown in your private Notes collection.";

  const footer = el("div", "notes-footer-actions");
  const pagerSlot = el("div", "notes-pager-slot");
  const newButton = renderButton("NEW", { variant: "primary", label: "New note" });
  newButton.classList.add("notes-new-button");
  footer.append(pagerSlot, newButton);
  listView.append(list, empty, footer);

  const editorView = el("section", "notes-editor-view");
  editorView.setAttribute("aria-label", "Note edit and preview view");
  const editorHeader = el("header", "note-editor-head");
  const editorRef = el("span", "ref-code");
  editorRef.textContent = "DRAFT";
  const backButton = renderButton("RETURN", { label: "Return to note list" });
  const modeSwitch = el("div", "note-mode-switch");
  modeSwitch.setAttribute("aria-label", "Editor mode");
  const editButton = renderButton("EDIT", { chip: true, variant: "primary", pressed: true });
  const previewButton = renderButton("VIEW", { chip: true, pressed: false, label: "Preview note" });
  const historyButton = renderButton("HISTORY", { chip: true, pressed: false, label: "Show note history" });
  modeSwitch.append(editButton, previewButton, historyButton);
  const editorStatus = renderStatusBadge("Loading", { state: "off" });
  editorStatus.classList.add("note-editor-status");
  editorHeader.append(editorRef, modeSwitch, editorStatus);

  const editorPanel = el("div", "note-editor-panel");
  editorPanel.dataset.mode = "edit";
  const sourceField = el("label", "control-field");
  const source = el("textarea", "textarea note-editor-source");
  source.spellcheck = false;
  source.setAttribute("aria-label", "Markdown note source");
  sourceField.append(source);
  const preview = el("article", "note-preview");
  preview.setAttribute("aria-label", "Markdown preview");
  const history = el("section", "note-history");
  history.setAttribute("aria-label", "Note version history");
  const historyLayout = el("div", "note-history__layout");
  const historyList = el("div", "note-history__list");
  historyList.setAttribute("aria-label", "Note versions");
  const historyDetail = renderSurface("article", { className: "note-history__detail", label: "Selected note version" });
  const historyDetailEmpty = document.createElement("p");
  historyDetailEmpty.textContent = "Select a version to inspect its immutable content snapshot.";
  historyDetail.append(historyDetailEmpty);
  historyLayout.append(historyList, historyDetail);
  history.append(historyLayout);
  editorPanel.append(sourceField, preview, history);

  const editorActions = el("div", "note-editor-actions");
  const saveButton = renderButton("SAVE", { variant: "primary" });
  const deleteButton = renderButton("DELETE", { variant: "danger" });
  editorActions.append(backButton, saveButton, deleteButton);
  editorView.append(editorHeader, editorPanel, editorActions);
  module.append(listView, editorView);
  target.append(module);

  function setNotice(title, message, tone = "info") {
    noticeTitle.textContent = title;
    noticeMessage.textContent = message;
    notice.dataset.tone = tone;
  }

  function renderList() {
    list.replaceChildren(...state.notes.map((note) => renderNoteCard(note, openEditor)));
    empty.dataset.visible = String(state.notes.length === 0);
    renderPagerSlot();
  }

  function renderPagerSlot() {
    pagerSlot.hidden = false;
    pagerSlot.replaceChildren(renderPager({
      currentPage: state.currentPage,
      hasPrevious: state.currentPage > 1,
      hasNext: state.hasMore,
      onPage(page) {
        void loadNotes(page);
      },
    }));
  }

  async function loadNotes(page) {
    const requestNumber = ++state.listRequestNumber;
    const offset = (page - 1) * pageSize;
    setNotice("Notes API", `Loading page ${page} from your private notes.`, "info");
    try {
      const result = await getJSON(`/api/notes?limit=${pageSize}&offset=${offset}`);
      if (requestNumber !== state.listRequestNumber) {
        return;
      }
      if ((result.notes ?? []).length === 0 && page > 1) {
        await loadNotes(page - 1);
        return;
      }
      state.notes = result.notes ?? [];
      state.currentPage = page;
      state.hasMore = Boolean(result.pagination?.has_more);
      renderList();
      setNotice("Notes API", `Showing up to ${pageSize} notes per page from your private collection.`, "info");
    } catch (error) {
      if (requestNumber !== state.listRequestNumber) {
        return;
      }
      state.notes = [];
      state.currentPage = 1;
      state.hasMore = false;
      list.replaceChildren();
      empty.dataset.visible = "false";
      renderPagerSlot();
      setNotice("Unable to Load Notes", error.message, "warning");
    }
  }

  function setMode(mode) {
    state.editorMode = mode;
    editorPanel.dataset.mode = mode;
    [
      [editButton, "edit"],
      [previewButton, "preview"],
      [historyButton, "history"],
    ].forEach(([button, buttonMode]) => {
      button.classList.toggle("button--primary", mode === buttonMode);
      button.setAttribute("aria-pressed", String(mode === buttonMode));
    });
  }

  function setEditorBusy(busy) {
    state.editorBusy = busy;
    source.disabled = busy;
    saveButton.disabled = busy;
    deleteButton.disabled = busy || !state.currentNote?.ref_code;
    historyButton.disabled = busy || !state.currentNote?.ref_code;
  }

  function setMutating(mutating) {
    state.mutating = mutating;
    backButton.disabled = mutating;
  }

  function setSavedStatus() {
    editorRef.textContent = state.currentNote?.ref_code || "DRAFT";
    editorStatus.textContent = state.currentNote?.ref_code ? "Saved" : "New Draft";
    editorStatus.dataset.state = state.currentNote?.ref_code ? "off" : "warning";
  }

  function resetHistory() {
    state.historyRequestNumber += 1;
    state.versionRequestNumber += 1;
    state.historyVersions = [];
    state.selectedVersionRef = null;
    historyList.replaceChildren();
    historyDetail.replaceChildren(historyDetailEmpty);
  }

  function renderHistoryList() {
    historyList.replaceChildren(...state.historyVersions.map((version) => renderVersionCard(
      version,
      state.currentNote?.current_version_ref,
      openHistoryVersion,
    )));
  }

  async function loadHistory() {
    if (!state.currentNote?.ref_code) {
      return;
    }
    const noteRefCode = state.currentNote.ref_code;
    const requestNumber = ++state.historyRequestNumber;
    historyList.replaceChildren();
    historyDetail.replaceChildren(historyDetailEmpty);
    try {
      const result = await getJSON(`/api/notes/${encodeURIComponent(noteRefCode)}/versions`);
      if (requestNumber !== state.historyRequestNumber || state.currentNote?.ref_code !== noteRefCode) {
        return;
      }
      state.historyVersions = result.versions ?? [];
      renderHistoryList();
    } catch (error) {
      if (requestNumber !== state.historyRequestNumber) {
        return;
      }
      setNotice("Unable to Load Note History", error.message, "warning");
    }
  }

  async function openHistoryVersion(refCode) {
    const requestNumber = ++state.versionRequestNumber;
    state.selectedVersionRef = refCode;
    historyDetail.replaceChildren();
    const loading = document.createElement("p");
    loading.textContent = `Loading ${refCode}...`;
    historyDetail.append(loading);
    try {
      const result = await getJSON(`/api/notes/versions/by-ref/${encodeURIComponent(refCode)}`);
      if (requestNumber !== state.versionRequestNumber || state.selectedVersionRef !== refCode) {
        return;
      }
      const version = result.version;
      const metadata = el("header", "note-history-version__head");
      const identity = document.createElement("strong");
      identity.textContent = `V${version.version_number} · ${versionOperationLabel(version.operation)}`;
      const reference = el("span", "ref-code");
      reference.textContent = version.ref_code;
      const created = el("span", "note-history-version__date");
      created.textContent = formatDateTime(version.created_at);
      metadata.append(identity, reference, created);
      const versionPreview = el("div", "note-preview note-history-version__preview");
      renderPreview(versionPreview, version.content);
      historyDetail.replaceChildren(metadata, versionPreview);
    } catch (error) {
      if (requestNumber !== state.versionRequestNumber) {
        return;
      }
      const failed = document.createElement("p");
      failed.textContent = error.message;
      historyDetail.replaceChildren(failed);
      setNotice("Unable to Load Note Version", error.message, "warning");
    }
  }

  function showHistory() {
    if (!state.currentNote?.ref_code || state.editorBusy) {
      return;
    }
    setMode("history");
    if (state.historyVersions.length === 0) {
      void loadHistory();
    }
  }

  async function openEditor(refCode) {
    const requestNumber = ++state.detailRequestNumber;
    state.currentNote = null;
    state.draftMarkdown = "";
    state.dirty = false;
    resetHistory();
    source.value = "";
    renderPreview(preview, "");
    setMode("edit");
    module.dataset.view = "edit";
    editorRef.textContent = refCode || "DRAFT";
    editorStatus.textContent = "Loading";
    editorStatus.dataset.state = "off";
    setEditorBusy(true);
    try {
      const result = await getJSON(`/api/notes/${encodeURIComponent(refCode)}`);
      if (requestNumber !== state.detailRequestNumber) {
        return;
      }
      state.currentNote = result.note;
      state.draftMarkdown = result.note.markdown;
      source.value = result.note.markdown;
      renderPreview(preview, state.draftMarkdown);
      setSavedStatus();
      setEditorBusy(false);
    } catch (error) {
      if (requestNumber !== state.detailRequestNumber) {
        return;
      }
      editorStatus.textContent = "Load Failed";
      editorStatus.dataset.state = "warning";
      setEditorBusy(true);
      setNotice("Unable to Load Note", error.message, "warning");
    }
  }

  function returnToList() {
    if (state.mutating) {
      return;
    }
    if (state.dirty && !window.confirm("Discard unsaved changes?")) {
      return;
    }
    state.detailRequestNumber += 1;
    state.currentNote = null;
    state.draftMarkdown = "";
    state.dirty = false;
    resetHistory();
    module.dataset.view = "list";
    renderList();
  }

  function createNote() {
    state.detailRequestNumber += 1;
    state.currentNote = {
      ref_code: null,
      markdown: newNoteMarkdown,
    };
    state.draftMarkdown = newNoteMarkdown;
    state.dirty = true;
    resetHistory();
    source.value = newNoteMarkdown;
    renderPreview(preview, state.draftMarkdown);
    setMode("edit");
    module.dataset.view = "edit";
    setSavedStatus();
    setEditorBusy(false);
  }

  async function saveNote() {
    if (!state.currentNote || state.editorBusy) {
      return;
    }
    setMutating(true);
    setEditorBusy(true);
    editorStatus.textContent = "Saving";
    editorStatus.dataset.state = "off";
    try {
      const result = state.currentNote.ref_code
        ? await patchJSON(`/api/notes/${encodeURIComponent(state.currentNote.ref_code)}`, { markdown: state.draftMarkdown })
        : await postJSON("/api/notes", { markdown: state.draftMarkdown });
      state.currentNote = result.note;
      state.draftMarkdown = result.note.markdown;
      state.dirty = false;
      resetHistory();
      source.value = result.note.markdown;
      renderPreview(preview, state.draftMarkdown);
      setSavedStatus();
      setMutating(false);
      setEditorBusy(false);
      if (state.editorMode === "history") {
        void loadHistory();
      }
      await loadNotes(1);
    } catch (error) {
      editorStatus.textContent = "Save Failed";
      editorStatus.dataset.state = "warning";
      setMutating(false);
      setEditorBusy(false);
      setNotice("Unable to Save Note", error.message, "warning");
    }
  }

  async function deleteNote() {
    if (!state.currentNote?.ref_code || state.editorBusy) {
      return;
    }
    if (!window.confirm(`Delete ${state.currentNote.ref_code}?`)) {
      return;
    }
    setMutating(true);
    setEditorBusy(true);
    editorStatus.textContent = "Deleting";
    editorStatus.dataset.state = "warning";
    try {
      await deleteJSON(`/api/notes/${encodeURIComponent(state.currentNote.ref_code)}`);
      state.detailRequestNumber += 1;
      state.currentNote = null;
      state.dirty = false;
      resetHistory();
      module.dataset.view = "list";
      setMutating(false);
      setEditorBusy(false);
      await loadNotes(state.currentPage);
    } catch (error) {
      editorStatus.textContent = "Delete Failed";
      editorStatus.dataset.state = "warning";
      setMutating(false);
      setEditorBusy(false);
      setNotice("Unable to Delete Note", error.message, "warning");
    }
  }

  source.addEventListener("input", () => {
    state.draftMarkdown = source.value;
    state.dirty = state.draftMarkdown !== state.currentNote?.markdown;
    if (state.dirty) {
      editorStatus.textContent = "Unsaved Changes";
      editorStatus.dataset.state = "warning";
    } else {
      setSavedStatus();
    }
    renderPreview(preview, state.draftMarkdown);
  });
  editButton.addEventListener("click", () => setMode("edit"));
  previewButton.addEventListener("click", () => setMode("preview"));
  historyButton.addEventListener("click", showHistory);
  backButton.addEventListener("click", returnToList);
  newButton.addEventListener("click", createNote);
  empty.append(emptyTitle, emptyMessage);
  saveButton.addEventListener("click", saveNote);
  deleteButton.addEventListener("click", deleteNote);

  empty.dataset.visible = "false";
  pagerSlot.hidden = true;
  void loadNotes(1);
}
