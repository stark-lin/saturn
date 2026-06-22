// This file renders the authenticated Files collection and upload workflow.
import { deleteJSON, getAccessToken, getJSON, postFormData, postJSON } from "../../shared/api/client.js";
import {
  renderButton,
  renderNotice,
  renderStatusBadge,
  renderSurface,
  renderTag,
} from "../../shared/components/primitives.js";
import { el } from "../../shared/utils/dom.js";

const collectionPageSize = 10;
const filePageSize = 10;
const allCollectionsPageSize = 100;

function appendText(parent, tagName, className, text) {
  const node = el(tagName, className);
  node.textContent = text;
  parent.append(node);
  return node;
}

function formatDate(value) {
  return String(value ?? "").slice(0, 10) || "--";
}

function formatBytes(value) {
  const bytes = Number(value ?? 0);
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return "0 B";
  }
  const units = ["B", "KB", "MB", "GB", "TB"];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const amount = bytes / (1024 ** index);
  return `${amount >= 10 || index === 0 ? amount.toFixed(0) : amount.toFixed(1)} ${units[index]}`;
}

function splitTags(value) {
  const uniqueTags = new Set();
  String(value ?? "")
    .split(",")
    .map((tag) => tag.trim())
    .filter(Boolean)
    .forEach((tag) => uniqueTags.add(tag));
  return Array.from(uniqueTags);
}

async function getAllPages(path, key) {
  const values = [];
  let offset = 0;
  let hasMore = true;
  while (hasMore) {
    const separator = path.includes("?") ? "&" : "?";
    const result = await getJSON(`${path}${separator}limit=${allCollectionsPageSize}&offset=${offset}`);
    const pageValues = result[key] ?? [];
    values.push(...pageValues);
    hasMore = Boolean(result.pagination?.has_more);
    offset += result.pagination?.limit ?? allCollectionsPageSize;
    if (hasMore && pageValues.length === 0) {
      break;
    }
  }
  return values;
}

function renderControlLabel(text) {
  const label = el("span", "control-label");
  label.textContent = text;
  return label;
}

function renderField({ label, name, type = "text", placeholder = "", required = false }) {
  const field = el("label", "control-field");
  const input = el("input", "field");
  input.name = name;
  input.type = type;
  input.placeholder = placeholder;
  input.required = required;
  field.append(renderControlLabel(label), input);
  return field;
}

function renderTextarea({ label, name, placeholder = "" }) {
  const field = el("label", "control-field");
  const textarea = el("textarea", "textarea");
  textarea.name = name;
  textarea.placeholder = placeholder;
  field.append(renderControlLabel(label), textarea);
  return field;
}

function renderFileField({ label, name, required = false }) {
  const field = el("label", "control-field");
  const input = el("input", "field");
  input.name = name;
  input.type = "file";
  input.required = required;
  field.append(renderControlLabel(label), input);
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

function renderTableCell(row, text, className = "") {
  const cell = document.createElement("td");
  cell.className = className;
  cell.textContent = text;
  row.append(cell);
  return cell;
}

function renderEmptyRow(body, columnCount, text) {
  const row = document.createElement("tr");
  const cell = renderTableCell(row, text, "accounting-empty-cell");
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
  pager.setAttribute("aria-label", "Pagination");
  const previous = el("button", "page-button");
  previous.type = "button";
  previous.textContent = "<-";
  previous.disabled = !hasPrevious;
  previous.setAttribute("aria-label", "Previous page");
  previous.addEventListener("click", () => onPage(currentPage - 1));
  const current = el("button", "page-button");
  current.type = "button";
  current.textContent = String(currentPage);
  current.setAttribute("aria-current", "page");
  const next = el("button", "page-button");
  next.type = "button";
  next.textContent = "->";
  next.disabled = !hasNext;
  next.setAttribute("aria-label", "Next page");
  next.addEventListener("click", () => onPage(currentPage + 1));
  pager.append(previous, current, next);
  return pager;
}

function renderStat(label) {
  const card = renderSurface("article", { className: "accounting-stat" });
  appendText(card, "span", "", label);
  const value = appendText(card, "strong", "", "--");
  const visual = el("div", "accounting-stat__visual");
  const note = appendText(card, "p", "", "");
  card.append(visual, note);
  return { card, value, visual, note };
}

function renderStatusTag(status) {
  const tag = renderTag(status || "active");
  if (status === "deleted") {
    tag.classList.add("tag--voided");
  }
  return tag;
}

function renderTagCollection(tags) {
  const collection = el("div", "accounting-tags");
  (tags?.length ? tags : ["untagged"]).forEach((tag) => collection.append(renderTag(tag)));
  return collection;
}

export function renderFilesPage(target) {
  const state = {
    collections: [],
    allFiles: [],
    selectedCollection: null,
    files: [],
    currentFile: null,
    collectionPage: 1,
    filePage: 1,
    fileHasMore: false,
    overviewRequestNumber: 0,
    collectionRequestNumber: 0,
    fileRequestNumber: 0,
  };

  const module = el("div", "accounting-module");
  const notice = renderNotice({
    title: "Files API",
    message: "Loading your private file collections.",
    tone: "info",
  });
  notice.classList.add("accounting-notice");
  const noticeTitle = notice.querySelector("strong");
  const noticeMessage = notice.querySelector("p");

  const collectionsView = el("section", "accounting-view");
  collectionsView.setAttribute("aria-label", "File collection list");
  const summary = el("section", "accounting-summary-grid");
  summary.setAttribute("aria-label", "Files summary");
  const totalCollections = renderStat("Collections");
  const totalFiles = renderStat("Files");
  const totalStorage = renderStat("Storage Used");
  summary.append(totalCollections.card, totalFiles.card, totalStorage.card);

  const collectionsPanel = renderSurface("section", { className: "section accounting-panel", label: "Collections" });
  const collectionsTable = renderTable("Files collections", [
    { label: "Ref" },
    { label: "Collection" },
    { label: "Tags" },
    { label: "Status" },
    { label: "Description" },
    { label: "Updated" },
  ]);
  const collectionsFooter = el("footer", "accounting-footer");
  const collectionPagerSlot = el("div", "accounting-pager-slot");
  const newCollectionButton = renderButton("NEW", { variant: "primary", label: "New collection" });
  collectionsFooter.append(collectionPagerSlot, newCollectionButton);
  collectionsPanel.append(collectionsTable.element, collectionsFooter);
  collectionsView.append(summary, collectionsPanel);

  const newCollectionView = el("section", "accounting-view");
  newCollectionView.hidden = true;
  newCollectionView.setAttribute("aria-label", "New file collection");
  const collectionForm = renderSurface("form", { className: "accounting-form", label: "Create collection" });
  const collectionFormHeader = el("header", "accounting-form__head");
  appendText(collectionFormHeader, "span", "eyebrow", "Files / New");
  appendText(collectionFormHeader, "h2", "accounting-form__title", "Create Collection");
  appendText(collectionFormHeader, "p", "accounting-form__text", "Create a collection before uploading immutable file metadata and blobs.");
  const collectionFields = el("div", "control-stack accounting-form__fields");
  collectionFields.append(
    renderField({ label: "Collection Name", name: "name", placeholder: "Receipts", required: true }),
    renderField({ label: "Tags / Comma Separated", name: "tags", placeholder: "tax, receipt" }),
    renderTextarea({ label: "Description", name: "description", placeholder: "Receipts, manuals, exported records..." }),
  );
  const collectionActions = el("footer", "accounting-form__actions");
  const cancelCollectionButton = renderButton("RETURN");
  const saveCollectionButton = renderButton("SAVE", { type: "submit", variant: "primary" });
  collectionActions.append(cancelCollectionButton, saveCollectionButton);
  collectionForm.append(collectionFormHeader, collectionFields, collectionActions);
  newCollectionView.append(collectionForm);

  const collectionView = el("section", "accounting-view");
  collectionView.hidden = true;
  collectionView.setAttribute("aria-label", "Collection files");
  const collectionInfoGrid = el("section", "accounting-ledger-info-grid");
  collectionInfoGrid.setAttribute("aria-label", "Collection information");
  const collectionIdentity = renderSurface("article", { className: "accounting-stat accounting-ledger-identity" });
  appendText(collectionIdentity, "span", "", "COLLECTION");
  const collectionName = appendText(collectionIdentity, "strong", "accounting-ledger-title", "---");
  const collectionMeta = el("div", "accounting-ledger-meta");
  const collectionRef = appendText(collectionMeta, "span", "ref-code accounting-ledger-ref", "---");
  const collectionBadgeSlot = el("div", "accounting-stat__visual accounting-ledger-tags");
  collectionMeta.append(collectionBadgeSlot);
  collectionIdentity.append(collectionMeta);
  const collectionFileCount = renderStat("Files");
  const collectionBytes = renderStat("Storage Used");
  collectionInfoGrid.append(collectionIdentity, collectionFileCount.card, collectionBytes.card);
  const backToCollectionsButton = renderButton("RETURN", { label: "Return to collections" });
  const collectionActionsRack = el("div", "actions accounting-ledger-actions");
  const uploadButton = renderButton("UPLOAD", { variant: "primary", label: "Upload file" });
  const deleteCollectionButton = renderButton("DELETE", { variant: "danger", label: "Delete collection" });
  collectionActionsRack.append(backToCollectionsButton, uploadButton, deleteCollectionButton);

  const filePanel = renderSurface("section", { className: "section accounting-panel", label: "Files" });
  const fileTable = renderTable("Selected collection files", [
    { label: "Ref" },
    { label: "File Name" },
    { label: "MIME Type" },
    { label: "Tags" },
    { label: "Size", className: "accounting-align-right" },
    { label: "Status" },
    { label: "Uploaded" },
  ]);
  const fileFooter = el("footer", "accounting-footer");
  const filePagerSlot = el("div", "accounting-pager-slot");
  fileFooter.append(filePagerSlot, collectionActionsRack);
  filePanel.append(fileTable.element, fileFooter);
  collectionView.append(collectionInfoGrid, filePanel);

  const uploadView = el("section", "accounting-view");
  uploadView.hidden = true;
  uploadView.setAttribute("aria-label", "Upload file");
  const uploadForm = renderSurface("form", { className: "accounting-form", label: "Upload file" });
  const uploadFormHeader = el("header", "accounting-form__head");
  appendText(uploadFormHeader, "span", "eyebrow", "Files / Upload");
  const uploadFormTitle = appendText(uploadFormHeader, "h2", "accounting-form__title", "Upload File");
  appendText(uploadFormHeader, "p", "accounting-form__text", "Original name, MIME type, size and hashes are captured during upload and cannot be edited afterwards.");
  const uploadFields = el("div", "control-stack accounting-form__fields");
  uploadFields.append(
    renderFileField({ label: "File", name: "file", required: true }),
    renderField({ label: "Tags / Comma Separated", name: "tags", placeholder: "receipt, tax" }),
  );
  const uploadActions = el("footer", "accounting-form__actions");
  const cancelUploadButton = renderButton("RETURN");
  const saveUploadButton = renderButton("UPLOAD", { type: "submit", variant: "primary" });
  uploadActions.append(cancelUploadButton, saveUploadButton);
  uploadForm.append(uploadFormHeader, uploadFields, uploadActions);
  uploadView.append(uploadForm);

  const fileDetailView = el("section", "accounting-view");
  fileDetailView.hidden = true;
  fileDetailView.setAttribute("aria-label", "File detail");
  const fileDetail = renderSurface("section", { className: "accounting-transaction-detail", label: "File detail" });
  const detailHead = el("header", "accounting-detail__head accounting-transaction-detail__head");
  const detailCopy = el("div", "accounting-detail__copy");
  const detailRef = appendText(detailCopy, "span", "ref-code", "---");
  const detailTitle = appendText(detailCopy, "h2", "accounting-detail__title", "---");
  const detailBadgeSlot = el("div", "accounting-detail__badge");
  detailHead.append(detailCopy, detailBadgeSlot);
  const detailGrid = el("div", "accounting-readonly-grid");
  const detailValues = {
    collection: renderReadonlyDetail("Collection"),
    mimeType: renderReadonlyDetail("MIME Type"),
    size: renderReadonlyDetail("Size"),
    status: renderReadonlyDetail("Status"),
    tags: renderReadonlyDetail("Tags", "accounting-readonly--wide"),
    sha256: renderReadonlyDetail("SHA256", "accounting-readonly--wide"),
    blake3: renderReadonlyDetail("BLAKE3", "accounting-readonly--wide"),
    created: renderReadonlyDetail("Uploaded"),
  };
  Object.values(detailValues).forEach((item) => detailGrid.append(item.element));
  const detailFooter = el("footer", "accounting-footer");
  const backToFilesButton = renderButton("RETURN");
  const detailActions = el("div", "actions accounting-ledger-actions");
  const downloadButton = renderButton("DOWNLOAD", { variant: "primary" });
  const deleteFileButton = renderButton("DELETE", { variant: "danger", label: "Delete file" });
  detailActions.append(backToFilesButton, downloadButton, deleteFileButton);
  detailFooter.append(detailActions);
  fileDetail.append(detailHead, detailGrid, detailFooter);
  fileDetailView.append(fileDetail);

  module.append(collectionsView, newCollectionView, collectionView, uploadView, fileDetailView);
  target.append(module);

  function renderReadonlyDetail(label, className = "") {
    const element = el("div", `accounting-readonly ${className}`.trim());
    element.append(renderControlLabel(label));
    const value = appendText(element, "div", "accounting-readonly__value", "---");
    return { element, value };
  }

  function setNotice(title, message, tone = "info") {
    noticeTitle.textContent = title;
    noticeMessage.textContent = message;
    notice.dataset.tone = tone;
  }

  function setView(view) {
    collectionsView.hidden = view !== "collections";
    newCollectionView.hidden = view !== "new-collection";
    collectionView.hidden = view !== "collection";
    uploadView.hidden = view !== "upload";
    fileDetailView.hidden = view !== "file-detail";
  }

  function setFormBusy(form, busy) {
    Array.from(form.elements).forEach((element) => {
      element.disabled = busy;
    });
  }

  function renderOverview() {
    const bytes = state.allFiles.reduce((total, file) => total + Number(file.size_bytes ?? 0), 0);
    totalCollections.value.textContent = String(state.collections.length);
    totalCollections.visual.replaceChildren();
    totalCollections.note.textContent = "Owner-only file collections.";
    totalFiles.value.textContent = String(state.allFiles.length);
    totalFiles.visual.replaceChildren();
    totalFiles.note.textContent = "Immutable file records across all collections.";
    totalStorage.value.textContent = formatBytes(bytes);
    totalStorage.visual.replaceChildren();
    totalStorage.note.textContent = "Blob size reported by stored file metadata.";
  }

  function renderCollections() {
    const firstIndex = (state.collectionPage - 1) * collectionPageSize;
    const visibleCollections = state.collections.slice(firstIndex, firstIndex + collectionPageSize);
    if (visibleCollections.length === 0) {
      renderEmptyRow(collectionsTable.body, 6, "No collections found. Create a collection before uploading files.");
    } else {
      collectionsTable.body.replaceChildren(...visibleCollections.map((collection) => {
        const row = el("tr", "accounting-row");
        row.setAttribute("aria-label", `Open ${collection.name} collection`);
        renderTableCell(row, collection.ref_code, "ref-code");
        renderTableCell(row, collection.name, "accounting-name");
        const tagsCell = document.createElement("td");
        tagsCell.append(renderTagCollection(collection.tags));
        row.append(tagsCell);
        const statusCell = document.createElement("td");
        statusCell.append(renderStatusTag(collection.status));
        row.append(statusCell);
        renderTableCell(row, collection.description || "--");
        renderTableCell(row, formatDate(collection.updated_at));
        enableRow(row, () => void openCollection(collection.ref_code, 1));
        return row;
      }));
    }
    const pageCount = Math.max(1, Math.ceil(state.collections.length / collectionPageSize));
    collectionPagerSlot.replaceChildren(renderPager({
      currentPage: state.collectionPage,
      hasPrevious: state.collectionPage > 1,
      hasNext: state.collectionPage < pageCount,
      onPage(page) {
        state.collectionPage = page;
        renderCollections();
      },
    }));
  }

  function renderCollection() {
    const collection = state.selectedCollection;
    if (!collection) {
      return;
    }
    const collectionFiles = state.allFiles.filter((file) => file.collection_ref_code === collection.ref_code);
    const bytes = collectionFiles.reduce((total, file) => total + Number(file.size_bytes ?? 0), 0);
    collectionRef.textContent = collection.ref_code;
    collectionName.textContent = collection.name;
    collectionBadgeSlot.replaceChildren(renderTagCollection(collection.tags));
    collectionFileCount.value.textContent = String(collectionFiles.length);
    collectionFileCount.visual.replaceChildren();
    collectionFileCount.note.textContent = "Files uploaded into this collection.";
    collectionBytes.value.textContent = formatBytes(bytes);
    collectionBytes.visual.replaceChildren();
    collectionBytes.note.textContent = "Total immutable blob metadata size.";

    if (state.files.length === 0) {
      renderEmptyRow(fileTable.body, 7, "No files found for this collection.");
    } else {
      fileTable.body.replaceChildren(...state.files.map((file) => {
        const row = el("tr", "accounting-row");
        row.setAttribute("aria-label", `Open ${file.original_name} file`);
        renderTableCell(row, file.ref_code, "ref-code");
        renderTableCell(row, file.original_name, "accounting-name");
        renderTableCell(row, file.mime_type || "application/octet-stream");
        const tagsCell = document.createElement("td");
        tagsCell.append(renderTagCollection(file.tags));
        row.append(tagsCell);
        renderTableCell(row, formatBytes(file.size_bytes), "accounting-align-right accounting-amount");
        const statusCell = document.createElement("td");
        statusCell.append(renderStatusTag(file.status));
        row.append(statusCell);
        renderTableCell(row, formatDate(file.created_at));
        enableRow(row, () => void openFile(file.ref_code));
        return row;
      }));
    }
    filePagerSlot.replaceChildren(renderPager({
      currentPage: state.filePage,
      hasPrevious: state.filePage > 1,
      hasNext: state.fileHasMore,
      onPage(page) {
        void openCollection(collection.ref_code, page);
      },
    }));
  }

  function renderFileDetail() {
    const file = state.currentFile;
    const collection = state.selectedCollection;
    if (!file) {
      return;
    }
    detailRef.textContent = file.ref_code;
    detailTitle.textContent = file.original_name;
    detailBadgeSlot.replaceChildren(renderStatusBadge("Uploaded / Immutable", { state: "warning" }));
    detailValues.collection.value.textContent = collection
      ? `${collection.name} / ${collection.ref_code}`
      : file.collection_ref_code;
    detailValues.mimeType.value.textContent = file.mime_type || "application/octet-stream";
    detailValues.size.value.textContent = formatBytes(file.size_bytes);
    detailValues.status.value.replaceChildren(renderStatusTag(file.status));
    detailValues.tags.value.replaceChildren(renderTagCollection(file.tags));
    detailValues.sha256.value.textContent = file.sha256 || file.metadata?.sha256 || "--";
    detailValues.blake3.value.textContent = file.blake3 || file.metadata?.blake3 || "--";
    detailValues.created.value.textContent = formatDate(file.created_at);
  }

  async function loadOverview({ announce = true } = {}) {
    const requestNumber = ++state.overviewRequestNumber;
    if (announce) {
      setNotice("Files API", "Loading collections and immutable file metadata.", "info");
    }
    try {
      const [collections, allFiles] = await Promise.all([
        getAllPages("/api/files/collections", "collections"),
        getAllPages("/api/files", "files"),
      ]);
      if (requestNumber !== state.overviewRequestNumber) {
        return false;
      }
      state.collections = collections;
      state.allFiles = allFiles;
      const pageCount = Math.max(1, Math.ceil(collections.length / collectionPageSize));
      state.collectionPage = Math.min(state.collectionPage, pageCount);
      renderOverview();
      renderCollections();
      if (announce) {
        setNotice("Files API", `Loaded ${collections.length} private collection${collections.length === 1 ? "" : "s"}.`, "info");
      }
      return true;
    } catch (error) {
      if (requestNumber !== state.overviewRequestNumber) {
        return false;
      }
      state.collections = [];
      state.allFiles = [];
      renderOverview();
      renderCollections();
      setNotice("Unable to Load Files", error.message, "warning");
      return false;
    }
  }

  async function openCollection(refCode, page = 1, { announce = true } = {}) {
    const requestNumber = ++state.collectionRequestNumber;
    state.filePage = page;
    setView("collection");
    if (announce) {
      setNotice("Files API", `Loading collection ${refCode}.`, "info");
    }
    try {
      const offset = (page - 1) * filePageSize;
      const fileParams = new URLSearchParams({
        collection_ref_code: refCode,
        limit: String(filePageSize),
        offset: String(offset),
      });
      const [collectionResult, fileResult] = await Promise.all([
        getJSON(`/api/files/collections/${encodeURIComponent(refCode)}`),
        getJSON(`/api/files?${fileParams.toString()}`),
      ]);
      if (requestNumber !== state.collectionRequestNumber) {
        return false;
      }
      if ((fileResult.files ?? []).length === 0 && page > 1) {
        return openCollection(refCode, page - 1, { announce });
      }
      state.selectedCollection = collectionResult.collection;
      state.files = fileResult.files ?? [];
      state.filePage = page;
      state.fileHasMore = Boolean(fileResult.pagination?.has_more);
      renderCollection();
      if (announce) {
        setNotice("Files API", `Showing immutable uploads for ${collectionResult.collection.name}.`, "info");
      }
      return true;
    } catch (error) {
      if (requestNumber !== state.collectionRequestNumber) {
        return false;
      }
      setNotice("Unable to Load Collection", error.message, "warning");
      return false;
    }
  }

  async function openFile(refCode) {
    const requestNumber = ++state.fileRequestNumber;
    setView("file-detail");
    setNotice("Files API", `Loading immutable file metadata ${refCode}.`, "info");
    try {
      const result = await getJSON(`/api/files/${encodeURIComponent(refCode)}`);
      if (requestNumber !== state.fileRequestNumber) {
        return;
      }
      state.currentFile = result.file;
      renderFileDetail();
      setNotice("Immutable File Metadata", "Uploaded file metadata cannot be edited. Delete and upload a new file if metadata is wrong.", "info");
    } catch (error) {
      if (requestNumber !== state.fileRequestNumber) {
        return;
      }
      setNotice("Unable to Load File", error.message, "warning");
    }
  }

  function showCollectionForm() {
    collectionForm.reset();
    setView("new-collection");
    setNotice("New Collection", "A collection groups immutable file uploads under one FIL reference.", "info");
  }

  function showUploadForm() {
    if (!state.selectedCollection) {
      return;
    }
    uploadForm.reset();
    uploadFormTitle.textContent = `Upload File / ${state.selectedCollection.name}`;
    setView("upload");
    setNotice("Upload File", "The selected file is uploaded as multipart form data; metadata is derived by the server.", "info");
  }

  async function createCollection(event) {
    event.preventDefault();
    const name = collectionForm.elements.name.value.trim();
    if (!name) {
      setNotice("Unable to Create Collection", "Collection name is required.", "warning");
      return;
    }
    setFormBusy(collectionForm, true);
    try {
      const result = await postJSON("/api/files/collections", {
        name,
        description: collectionForm.elements.description.value.trim(),
        tags: splitTags(collectionForm.elements.tags.value),
      });
      await loadOverview({ announce: false });
      await openCollection(result.collection.ref_code, 1, { announce: false });
      setNotice("Collection Created", `${result.collection.name} is ready for uploads.`, "info");
    } catch (error) {
      setNotice("Unable to Create Collection", error.message, "warning");
    } finally {
      setFormBusy(collectionForm, false);
    }
  }

  async function uploadFile(event) {
    event.preventDefault();
    const collection = state.selectedCollection;
    const file = uploadForm.elements.file.files[0];
    if (!collection || !file) {
      setNotice("Unable to Upload File", "Choose a file to upload.", "warning");
      return;
    }
    const body = new FormData();
    body.append("file", file);
    body.append("tags", splitTags(uploadForm.elements.tags.value).join(","));
    setFormBusy(uploadForm, true);
    try {
      await postFormData(`/api/files/collections/${encodeURIComponent(collection.ref_code)}/files`, body);
      await loadOverview({ announce: false });
      await openCollection(collection.ref_code, 1, { announce: false });
      setNotice("File Uploaded", "The immutable file metadata and blob have been stored.", "info");
    } catch (error) {
      setNotice("Unable to Upload File", error.message, "warning");
    } finally {
      setFormBusy(uploadForm, false);
    }
  }

  async function deleteCollection() {
    const collection = state.selectedCollection;
    if (!collection || !window.confirm(`Delete ${collection.name} and all files in it?`)) {
      return;
    }
    deleteCollectionButton.disabled = true;
    try {
      await deleteJSON(`/api/files/collections/${encodeURIComponent(collection.ref_code)}`);
      state.selectedCollection = null;
      state.currentFile = null;
      await loadOverview({ announce: false });
      setView("collections");
      setNotice("Collection Deleted", `${collection.name} and its files were deleted.`, "info");
    } catch (error) {
      setNotice("Unable to Delete Collection", error.message, "warning");
    } finally {
      deleteCollectionButton.disabled = false;
    }
  }

  async function deleteFile() {
    const file = state.currentFile;
    const collection = state.selectedCollection;
    if (!file || !collection || !window.confirm(`Delete ${file.original_name}?`)) {
      return;
    }
    deleteFileButton.disabled = true;
    try {
      await deleteJSON(`/api/files/${encodeURIComponent(file.ref_code)}`);
      state.currentFile = null;
      await loadOverview({ announce: false });
      await openCollection(collection.ref_code, state.filePage, { announce: false });
      setView("collection");
      setNotice("File Deleted", `${file.original_name} was deleted.`, "info");
    } catch (error) {
      setNotice("Unable to Delete File", error.message, "warning");
    } finally {
      deleteFileButton.disabled = false;
    }
  }

  async function downloadFile() {
    const file = state.currentFile;
    if (!file) {
      return;
    }
    downloadButton.disabled = true;
    try {
      const headers = new Headers({ Accept: file.mime_type || "application/octet-stream" });
      const token = getAccessToken();
      if (token) {
        headers.set("Authorization", `Bearer ${token}`);
      }
      const response = await fetch(`/api/files/objects/${encodeURIComponent(file.ref_code)}/download`, { headers });
      if (!response.ok) {
        const result = await response.json().catch(() => null);
        throw new Error(result?.error?.message ?? `Request failed with ${response.status}`);
      }
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = file.original_name;
      document.body.append(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      setNotice("Download Ready", `${file.original_name} was downloaded with verified metadata.`, "info");
    } catch (error) {
      setNotice("Unable to Download File", error.message, "warning");
      window.alert(`Unable to Download File: ${error.message}`);
    } finally {
      downloadButton.disabled = false;
    }
  }

  newCollectionButton.addEventListener("click", showCollectionForm);
  cancelCollectionButton.addEventListener("click", () => setView("collections"));
  collectionForm.addEventListener("submit", (event) => void createCollection(event));
  backToCollectionsButton.addEventListener("click", () => {
    setView("collections");
    setNotice("Files API", "Showing your private file collections.", "info");
  });
  uploadButton.addEventListener("click", showUploadForm);
  deleteCollectionButton.addEventListener("click", () => void deleteCollection());
  cancelUploadButton.addEventListener("click", () => {
    setView("collection");
    setNotice("Files API", `Showing immutable uploads for ${state.selectedCollection?.name ?? "this collection"}.`, "info");
  });
  uploadForm.addEventListener("submit", (event) => void uploadFile(event));
  backToFilesButton.addEventListener("click", () => {
    setView("collection");
    renderCollection();
  });
  downloadButton.addEventListener("click", () => void downloadFile());
  deleteFileButton.addEventListener("click", () => void deleteFile());

  setView("collections");
  void loadOverview();
}
