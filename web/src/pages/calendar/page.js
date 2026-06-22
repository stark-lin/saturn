// This file renders the authenticated Calendar event aggregate workflow.
import { deleteJSON, getJSON, postJSON } from "../../shared/api/client.js";
import {
  renderButton,
  renderMeter,
  renderNotice,
  renderStatusBadge,
  renderSurface,
  renderTag,
} from "../../shared/components/primitives.js";
import { el } from "../../shared/utils/dom.js";

const aggregatePageSize = 10;
const eventPageSize = 10;
const collectionPageSize = 100;
const weekdayOptions = [
  ["mon", "Mon"],
  ["tue", "Tue"],
  ["wed", "Wed"],
  ["thu", "Thu"],
  ["fri", "Fri"],
  ["sat", "Sat"],
  ["sun", "Sun"],
];
const defaultTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";

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
    const result = await getJSON(`${path}${separator}limit=${collectionPageSize}&offset=${offset}`);
    const pageValues = result[key] ?? [];
    values.push(...pageValues);
    hasMore = Boolean(result.pagination?.has_more);
    offset += result.pagination?.limit ?? collectionPageSize;
    if (hasMore && pageValues.length === 0) {
      break;
    }
  }
  return values;
}

function localDateValue(date = new Date()) {
  const offset = date.getTimezoneOffset() * 60 * 1000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 10);
}

function parseDateValue(value) {
  const [year, month, day] = String(value ?? "").split("-").map(Number);
  return new Date(year, month - 1, day);
}

function localMonthStart(date = new Date()) {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function monthRange(cursor) {
  const from = new Date(cursor.getFullYear(), cursor.getMonth(), 1, 0, 0, 0, 0);
  const to = new Date(cursor.getFullYear(), cursor.getMonth() + 1, 0, 23, 59, 59, 999);
  return {
    from: from.toISOString(),
    to: to.toISOString(),
  };
}

function monthDates(cursor) {
  const first = new Date(cursor.getFullYear(), cursor.getMonth(), 1);
  const start = new Date(first);
  const mondayOffset = (first.getDay() + 6) % 7;
  start.setDate(first.getDate() - mondayOffset);
  return Array.from({ length: 42 }, (_item, index) => {
    const date = new Date(start);
    date.setDate(start.getDate() + index);
    return date;
  });
}

function formatMonthLabel(date) {
  return date.toLocaleDateString("en-US", { month: "long", year: "numeric" });
}

function formatLocalDateTime(value) {
  if (!value) {
    return "--";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "--";
  }
  return `${localDateValue(date)} ${formatLocalTime(date)}`;
}

function formatLocalTime(date) {
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false });
}

function formatEventDate(event) {
  return localDateValue(new Date(event.starts_at));
}

function eventEnd(event) {
  return new Date(new Date(event.starts_at).getTime() + Number(event.duration_minutes ?? 0) * 60 * 1000);
}

function eventTimeRange(event) {
  const start = new Date(event.starts_at);
  if (Number.isNaN(start.getTime())) {
    return "--";
  }
  return `${formatLocalTime(start)}-${formatLocalTime(eventEnd(event))}`;
}

function combineLocalDateTime(dateValue, timeValue) {
  const [year, month, day] = String(dateValue ?? "").split("-").map(Number);
  const [hour, minute] = String(timeValue ?? "").split(":").map(Number);
  const date = new Date(year, month - 1, day, hour, minute, 0, 0);
  return Number.isNaN(date.getTime()) ? "" : date.toISOString();
}

function weekdayForDate(dateValue) {
  const index = (parseDateValue(dateValue).getDay() + 6) % 7;
  return weekdayOptions[index]?.[0] ?? "mon";
}

function renderControlLabel(text) {
  const label = el("span", "control-label");
  label.textContent = text;
  return label;
}

function renderField({ label, name, type = "text", placeholder = "", value = "", required = false, min = "" }) {
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

function renderSelect({ label, name, options, value }) {
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

function renderTextarea({ label, name, placeholder = "" }) {
  const field = el("label", "control-field");
  const textarea = el("textarea", "textarea");
  textarea.name = name;
  textarea.placeholder = placeholder;
  field.append(renderControlLabel(label), textarea);
  return field;
}

function renderWeekdayField() {
  const field = el("fieldset", "control-field calendar-weekday-field");
  const legend = document.createElement("legend");
  legend.append(renderControlLabel("Weekdays"));
  const checks = el("div", "calendar-check-grid");
  weekdayOptions.forEach(([value, label]) => {
    const item = el("label", "calendar-check");
    const input = document.createElement("input");
    input.type = "checkbox";
    input.name = "weekday";
    input.value = value;
    item.append(input, document.createTextNode(label));
    checks.append(item);
  });
  field.append(legend, checks);
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
  const card = renderSurface("article", { className: "accounting-stat calendar-stat" });
  appendText(card, "span", "", label);
  const value = appendText(card, "strong", "", "--");
  const visual = el("div", "accounting-stat__visual");
  const note = appendText(card, "p", "", "");
  card.append(visual, note);
  return { card, value, visual, note };
}

function renderTagCollection(tags) {
  const collection = el("div", "accounting-tags");
  (tags?.length ? tags : ["untagged"]).forEach((tag) => collection.append(renderTag(tag)));
  return collection;
}

function renderStatusTag(status) {
  const tag = renderTag(status || "scheduled");
  if (status === "finished" || status === "voided") {
    tag.classList.add("tag--voided");
  }
  return tag;
}

function isInactiveEventStatus(status) {
  return status === "finished" || status === "voided";
}

export function renderCalendarPage(target) {
  const state = {
    aggregates: [],
    viewEvents: [],
    selectedAggregate: null,
    aggregateEvents: [],
    currentEvent: null,
    selectedDate: localDateValue(),
    monthCursor: localMonthStart(new Date()),
    aggregatePage: 1,
    eventPage: 1,
    returnView: "overview",
    formReturnView: "overview",
    eventFormReturnView: "aggregate",
    overviewRequestNumber: 0,
    aggregateRequestNumber: 0,
    eventRequestNumber: 0,
  };

  const module = el("div", "accounting-module calendar-module");
  const notice = renderNotice({
    title: "Calendar API",
    message: "Loading scheduled events and event aggregates.",
    tone: "info",
  });
  notice.classList.add("accounting-notice");
  const noticeTitle = notice.querySelector("strong");
  const noticeMessage = notice.querySelector("p");

  const overviewView = el("section", "accounting-view calendar-view");
  overviewView.setAttribute("aria-label", "Calendar overview");

  const workbench = el("section", "calendar-widget");
  const monthPanel = el("section", "calendar-mini surface");
  monthPanel.setAttribute("aria-labelledby", "calendar-page-title");
  const monthHead = el("div", "calendar-head");
  const monthLabel = appendText(monthHead, "h4", "calendar-title", "---");
  monthLabel.id = "calendar-page-title";
  const calendarNav = el("div", "calendar-nav");
  calendarNav.setAttribute("aria-label", "Calendar navigation");
  const todayButton = renderButton("TODAY", {
    flat: true,
    className: "calendar-today-button",
    label: "Go to today",
  });
  const previousMonthButton = document.createElement("button");
  previousMonthButton.type = "button";
  previousMonthButton.className = "calendar-month-button";
  previousMonthButton.textContent = "<";
  previousMonthButton.setAttribute("aria-label", "Previous month");
  const nextMonthButton = document.createElement("button");
  nextMonthButton.type = "button";
  nextMonthButton.className = "calendar-month-button";
  nextMonthButton.textContent = ">";
  nextMonthButton.setAttribute("aria-label", "Next month");
  calendarNav.append(todayButton, previousMonthButton, nextMonthButton);
  monthHead.append(monthLabel, calendarNav);
  const weekdayGrid = el("div", "calendar-weekdays");
  weekdayGrid.setAttribute("aria-hidden", "true");
  weekdayOptions.forEach(([_value, label]) => appendText(weekdayGrid, "span", "calendar-weekday", label));
  const monthGrid = el("div", "calendar-grid");
  monthPanel.append(monthHead, weekdayGrid, monthGrid);

  const dayPanel = el("section", "calendar-agenda surface");
  dayPanel.setAttribute("aria-labelledby", "calendar-agenda-title");
  const dayTitle = appendText(dayPanel, "h4", "calendar-agenda-title", "Selected Day");
  dayTitle.id = "calendar-agenda-title";
  const dayEvents = el("ul", "agenda-list");
  dayPanel.append(dayEvents);
  workbench.append(monthPanel, dayPanel);

  const aggregatePanel = renderSurface("section", { className: "section accounting-panel calendar-panel", label: "Event aggregates" });
  const aggregateTable = renderTable("Calendar event aggregates", [
    { label: "Ref" },
    { label: "Title" },
    { label: "Tags" },
    { label: "Timezone" },
    { label: "Created" },
  ]);
  const aggregateFooter = el("footer", "accounting-footer");
  const aggregatePagerSlot = el("div", "accounting-pager-slot");
  const newAggregateButton = renderButton("NEW", { variant: "primary", label: "New event aggregate" });
  aggregateFooter.append(aggregatePagerSlot, newAggregateButton);
  aggregatePanel.append(aggregateTable.element, aggregateFooter);
  overviewView.append(workbench, aggregatePanel);

  const aggregateDetailView = el("section", "accounting-view calendar-view");
  aggregateDetailView.hidden = true;
  aggregateDetailView.setAttribute("aria-label", "Event aggregate detail");
  const aggregateInfoGrid = el("section", "accounting-ledger-info-grid calendar-detail-grid");
  aggregateInfoGrid.setAttribute("aria-label", "Aggregate information");
  const aggregateIdentity = renderSurface("article", { className: "accounting-stat accounting-ledger-identity" });
  appendText(aggregateIdentity, "span", "", "AGGREGATE");
  const aggregateTitle = appendText(aggregateIdentity, "strong", "accounting-ledger-title", "---");
  const aggregateLedgerMeta = el("div", "accounting-ledger-meta");
  const aggregateRef = appendText(aggregateLedgerMeta, "span", "ref-code accounting-ledger-ref", "---");
  const aggregateBadgeSlot = el("div", "accounting-stat__visual accounting-ledger-tags");
  aggregateLedgerMeta.append(aggregateBadgeSlot);
  aggregateIdentity.append(aggregateLedgerMeta);
  const aggregateEventsStat = renderStat("Events");
  const aggregateCreated = renderStat("Created");
  aggregateInfoGrid.append(aggregateIdentity, aggregateEventsStat.card, aggregateCreated.card);

  const aggregateMetadata = renderSurface("section", { className: "accounting-transaction-detail calendar-metadata", label: "Aggregate metadata" });
  const metadataGrid = el("div", "accounting-readonly-grid");
  const metadataValues = {
    timezone: renderReadonlyDetail("Timezone"),
    location: renderReadonlyDetail("Location"),
    description: renderReadonlyDetail("Description", "accounting-readonly--wide accounting-readonly--note"),
  };
  Object.values(metadataValues).forEach((item) => metadataGrid.append(item.element));
  aggregateMetadata.append(metadataGrid);

  const aggregateActions = el("div", "actions accounting-ledger-actions");
  const backToOverviewButton = renderButton("RETURN", { label: "Return to calendar overview" });
  const newEventButton = renderButton("NEW", { variant: "primary", label: "New event" });
  const deleteAggregateButton = renderButton("DELETE", { variant: "danger", label: "Delete event aggregate" });
  aggregateActions.append(backToOverviewButton, newEventButton, deleteAggregateButton);

  const aggregateEventsPanel = renderSurface("section", { className: "section accounting-panel calendar-panel", label: "Aggregate events" });
  const eventTable = renderTable("Selected aggregate events", [
    { label: "Ref" },
    { label: "Date" },
    { label: "Time" },
    { label: "Title" },
    { label: "Status" },
    { label: "Tags" },
    { label: "Note" },
  ]);
  const eventFooter = el("footer", "accounting-footer");
  const eventPagerSlot = el("div", "accounting-pager-slot");
  eventFooter.append(eventPagerSlot, aggregateActions);
  aggregateEventsPanel.append(eventTable.element, eventFooter);
  aggregateDetailView.append(aggregateInfoGrid, aggregateMetadata, aggregateEventsPanel);

  const newAggregateView = el("section", "accounting-view calendar-view");
  newAggregateView.hidden = true;
  newAggregateView.setAttribute("aria-label", "New event aggregate");
  const aggregateForm = renderSurface("form", { className: "accounting-form calendar-form", label: "Create event aggregate" });
  const aggregateFormHeader = el("header", "accounting-form__head");
  appendText(aggregateFormHeader, "span", "eyebrow", "Calendar / New");
  appendText(aggregateFormHeader, "h2", "accounting-form__title", "Create Event Aggregate");
  appendText(aggregateFormHeader, "p", "accounting-form__text", "Create immutable aggregate metadata. Events are added from the aggregate detail view.");
  const aggregateFields = el("div", "control-stack accounting-form__fields");
  const aggregateBasics = el("div", "control-split");
  aggregateBasics.append(
    renderField({ label: "Aggregate Title", name: "aggregate_title", placeholder: "Training", required: true }),
    renderField({ label: "Aggregate Tags", name: "aggregate_tags", placeholder: "health, weekly" }),
  );
  const aggregateDetails = el("div", "control-split");
  aggregateDetails.append(
    renderField({ label: "Timezone", name: "timezone", value: defaultTimezone }),
    renderField({ label: "Aggregate Location", name: "aggregate_location", placeholder: "Gym" }),
  );
  const aggregateDescription = renderTextarea({ label: "Aggregate Description", name: "aggregate_description", placeholder: "Weekly training block." });
  aggregateFields.append(
    aggregateBasics,
    aggregateDetails,
    aggregateDescription,
  );
  const aggregateFormActions = el("footer", "accounting-form__actions");
  const cancelAggregateButton = renderButton("RETURN");
  const saveAggregateButton = renderButton("SAVE", { type: "submit", variant: "primary" });
  aggregateFormActions.append(cancelAggregateButton, saveAggregateButton);
  aggregateForm.append(aggregateFormHeader, aggregateFields, aggregateFormActions);
  newAggregateView.append(aggregateForm);

  const newEventView = el("section", "accounting-view calendar-view");
  newEventView.hidden = true;
  newEventView.setAttribute("aria-label", "New aggregate event");
  const eventForm = renderSurface("form", { className: "accounting-form calendar-form", label: "Create aggregate event" });
  const eventFormHeader = el("header", "accounting-form__head");
  appendText(eventFormHeader, "span", "eyebrow", "Calendar / Event");
  appendText(eventFormHeader, "h2", "accounting-form__title", "Add Event");
  appendText(eventFormHeader, "p", "accounting-form__text", "Create scheduled event instances under the selected aggregate.");
  const eventFields = el("div", "control-stack accounting-form__fields");
  const eventBasics = el("div", "control-split");
  eventBasics.append(
    renderField({ label: "Event Title", name: "event_title", placeholder: "Training session", required: true }),
    renderField({ label: "Event Location", name: "event_location", placeholder: "Gym" }),
  );
  const scheduleFields = el("div", "control-split");
  scheduleFields.append(
    renderField({ label: "Starts Date", name: "starts_date", type: "date", value: state.selectedDate, required: true }),
    renderField({ label: "Starts Time", name: "starts_time", type: "time", value: "09:00", required: true }),
  );
  const durationRecurrenceFields = el("div", "control-split");
  durationRecurrenceFields.append(
    renderField({ label: "Duration Minutes", name: "duration_minutes", type: "number", value: "60", required: true, min: 1 }),
    renderSelect({
      label: "Recurrence",
      name: "recurrence_kind",
      options: [["single", "Single"], ["weekly", "Weekly"]],
      value: "single",
    }),
  );
  const recurrenceFields = el("div", "control-split calendar-recurrence-fields");
  recurrenceFields.append(
    renderWeekdayField(),
    renderField({ label: "Week Count", name: "week_count", type: "number", value: "1", min: 1 }),
  );
  const eventTags = renderField({ label: "Event Tags", name: "event_tags", placeholder: "workout, calendar" });
  const eventDescription = renderTextarea({ label: "Event Description", name: "event_description", placeholder: "Strength block." });
  eventFields.append(
    eventBasics,
    scheduleFields,
    durationRecurrenceFields,
    recurrenceFields,
    eventTags,
    eventDescription,
  );
  const eventFormActions = el("footer", "accounting-form__actions");
  const cancelEventButton = renderButton("RETURN");
  const saveEventButton = renderButton("SAVE", { type: "submit", variant: "primary" });
  eventFormActions.append(cancelEventButton, saveEventButton);
  eventForm.append(eventFormHeader, eventFields, eventFormActions);
  newEventView.append(eventForm);

  const eventDetailView = el("section", "accounting-view calendar-view");
  eventDetailView.hidden = true;
  eventDetailView.setAttribute("aria-label", "Event detail");
  const eventDetail = renderSurface("section", { className: "accounting-transaction-detail calendar-event-detail", label: "Event detail" });
  const eventDetailHead = el("header", "accounting-detail__head accounting-transaction-detail__head");
  const eventDetailCopy = el("div", "accounting-detail__copy");
  const eventDetailRef = appendText(eventDetailCopy, "span", "ref-code", "---");
  const eventDetailTitle = appendText(eventDetailCopy, "h2", "accounting-detail__title", "---");
  const eventDetailBadgeSlot = el("div", "accounting-detail__badge");
  eventDetailHead.append(eventDetailCopy, eventDetailBadgeSlot);
  const eventDetailGrid = el("div", "accounting-readonly-grid");
  const eventDetailValues = {
    aggregate: renderReadonlyDetail("Aggregate"),
    startsAt: renderReadonlyDetail("Starts At"),
    duration: renderReadonlyDetail("Duration"),
    status: renderReadonlyDetail("Status"),
    location: renderReadonlyDetail("Location", "accounting-readonly--wide"),
    tags: renderReadonlyDetail("Tags", "accounting-readonly--wide"),
    description: renderReadonlyDetail("Description", "accounting-readonly--wide accounting-readonly--note"),
  };
  Object.values(eventDetailValues).forEach((item) => eventDetailGrid.append(item.element));
  const eventDetailFooter = el("footer", "accounting-footer calendar-event-detail__footer");
  const backToAggregateButton = renderButton("RETURN");
  const eventDetailActions = el("div", "actions calendar-event-detail__actions");
  const finishEventButton = renderButton("FINISH", { variant: "primary", label: "Finish event" });
  const voidEventButton = renderButton("VOID", { variant: "danger", label: "Void event" });
  eventDetailActions.append(backToAggregateButton, finishEventButton, voidEventButton);
  eventDetailFooter.append(eventDetailActions);
  eventDetail.append(eventDetailHead, eventDetailGrid, eventDetailFooter);
  eventDetailView.append(eventDetail);

  module.append(overviewView, aggregateDetailView, newAggregateView, newEventView, eventDetailView);
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
    overviewView.hidden = view !== "overview";
    aggregateDetailView.hidden = view !== "aggregate";
    newAggregateView.hidden = view !== "new-aggregate";
    newEventView.hidden = view !== "new-event";
    eventDetailView.hidden = view !== "event-detail";
  }

  function setFormBusy(form, busy) {
    Array.from(form.elements).forEach((element) => {
      element.disabled = busy;
    });
  }

  function aggregateByRef(refCode) {
    if (state.selectedAggregate?.ref_code === refCode) {
      return state.selectedAggregate;
    }
    return state.aggregates.find((aggregate) => aggregate.ref_code === refCode) ?? null;
  }

  function eventsForSelectedDate() {
    return state.viewEvents
      .filter((event) => formatEventDate(event) === state.selectedDate)
      .sort((left, right) => `${left.starts_at}${left.ref_code}`.localeCompare(`${right.starts_at}${right.ref_code}`));
  }

  function renderMonthGrid() {
    monthLabel.textContent = formatMonthLabel(state.monthCursor);
    monthGrid.replaceChildren(...monthDates(state.monthCursor).map((date) => {
      const dateValue = localDateValue(date);
      const button = el("button", "calendar-day");
      button.type = "button";
      button.textContent = String(date.getDate()).padStart(2, "0");
      button.dataset.date = dateValue;
      if (date.getMonth() !== state.monthCursor.getMonth()) {
        button.dataset.muted = "true";
      }
      if (dateValue === state.selectedDate) {
        button.setAttribute("aria-selected", "true");
      }
      if (state.viewEvents.some((event) => formatEventDate(event) === dateValue)) {
        button.dataset.hasEvent = "true";
      }
      button.addEventListener("click", () => {
        state.selectedDate = dateValue;
        if (date.getMonth() !== state.monthCursor.getMonth() || date.getFullYear() !== state.monthCursor.getFullYear()) {
          state.monthCursor = localMonthStart(date);
          void loadOverview({ announce: false });
          return;
        }
        renderOverview();
      });
      return button;
    }));
  }

  function renderDayEvents() {
    dayTitle.textContent = `Selected Day · ${state.selectedDate}`;
    const dayEventsForDate = eventsForSelectedDate();
    if (dayEventsForDate.length === 0) {
      const empty = el("li", "agenda-item calendar-empty");
      const time = appendText(empty, "span", "agenda-time", "--:--");
      time.setAttribute("aria-hidden", "true");
      const copy = el("span", "agenda-copy");
      appendText(copy, "strong", "", "No scheduled events");
      appendText(copy, "span", "", state.selectedDate);
      empty.append(copy);
      dayEvents.replaceChildren(empty);
      return;
    }
    dayEvents.replaceChildren(...dayEventsForDate.map((event) => {
      const aggregate = aggregateByRef(event.aggregate_ref_code);
      const item = el("li", "agenda-item");
      item.dataset.status = event.status;
      item.setAttribute("role", "button");
      item.setAttribute("aria-label", `Open ${event.metadata?.title ?? event.ref_code}`);
      const time = appendText(item, "span", "agenda-time", eventTimeRange(event));
      const copy = el("span", "agenda-copy");
      appendText(copy, "strong", "calendar-event-title", event.metadata?.title ?? "Untitled event");
      appendText(copy, "span", "calendar-event-meta", `${aggregate?.metadata?.title ?? "Event"} · ${event.ref_code}`);
      item.append(time, copy);
      enableRow(item, () => void openEvent(event.ref_code, "overview"));
      return item;
    }));
  }

  function renderAggregates() {
    const firstIndex = (state.aggregatePage - 1) * aggregatePageSize;
    const visibleAggregates = state.aggregates.slice(firstIndex, firstIndex + aggregatePageSize);
    if (visibleAggregates.length === 0) {
      renderEmptyRow(aggregateTable.body, 5, "No event aggregates found. Create an aggregate to schedule calendar events.");
    } else {
      aggregateTable.body.replaceChildren(...visibleAggregates.map((aggregate) => {
        const row = el("tr", "accounting-row");
        row.setAttribute("aria-label", `Open ${aggregate.metadata?.title ?? aggregate.ref_code} aggregate`);
        renderTableCell(row, aggregate.ref_code, "ref-code");
        renderTableCell(row, aggregate.metadata?.title ?? "Untitled aggregate", "accounting-name");
        const tagsCell = document.createElement("td");
        tagsCell.append(renderTagCollection(aggregate.tags));
        row.append(tagsCell);
        renderTableCell(row, aggregate.metadata?.timezone || "--");
        renderTableCell(row, localDateValue(new Date(aggregate.created_at)));
        enableRow(row, () => void openAggregate(aggregate.ref_code, 1));
        return row;
      }));
    }
    const pageCount = Math.max(1, Math.ceil(state.aggregates.length / aggregatePageSize));
    aggregatePagerSlot.replaceChildren(renderPager({
      currentPage: state.aggregatePage,
      hasPrevious: state.aggregatePage > 1,
      hasNext: state.aggregatePage < pageCount,
      onPage(page) {
        state.aggregatePage = page;
        renderAggregates();
      },
    }));
  }

  function renderOverview() {
    renderMonthGrid();
    renderDayEvents();
    renderAggregates();
  }

  function renderAggregateDetail() {
    const aggregate = state.selectedAggregate;
    if (!aggregate) {
      return;
    }
    aggregateRef.textContent = aggregate.ref_code;
    aggregateTitle.textContent = aggregate.metadata?.title ?? "Untitled aggregate";
    aggregateBadgeSlot.replaceChildren(renderTagCollection(aggregate.tags));
    aggregateEventsStat.value.textContent = String(state.aggregateEvents.length);
    aggregateEventsStat.visual.replaceChildren(renderMeter({ label: "Loaded", value: state.aggregateEvents.length > 0 ? 100 : 0, tone: "info" }));
    aggregateEventsStat.note.textContent = "Includes scheduled, finished and voided child events.";
    aggregateCreated.value.textContent = localDateValue(new Date(aggregate.created_at));
    aggregateCreated.visual.replaceChildren();
    aggregateCreated.note.textContent = "Aggregate creation date.";
    metadataValues.timezone.value.textContent = aggregate.metadata?.timezone || "--";
    metadataValues.location.value.textContent = aggregate.metadata?.location || "--";
    metadataValues.description.value.textContent = aggregate.metadata?.description || "--";

    const firstIndex = (state.eventPage - 1) * eventPageSize;
    const visibleEvents = state.aggregateEvents.slice(firstIndex, firstIndex + eventPageSize);
    if (visibleEvents.length === 0) {
      renderEmptyRow(eventTable.body, 7, "No events found for this aggregate.");
    } else {
      eventTable.body.replaceChildren(...visibleEvents.map((event) => {
        const row = el("tr", `accounting-row ${isInactiveEventStatus(event.status) ? "calendar-status-voided" : ""}`.trim());
        row.setAttribute("aria-label", `Open ${event.metadata?.title ?? event.ref_code} event`);
        renderTableCell(row, event.ref_code, "ref-code");
        renderTableCell(row, formatEventDate(event));
        renderTableCell(row, eventTimeRange(event));
        renderTableCell(row, event.metadata?.title ?? "Untitled event", "accounting-name");
        const statusCell = document.createElement("td");
        statusCell.append(renderStatusTag(event.status));
        row.append(statusCell);
        const tagsCell = document.createElement("td");
        tagsCell.append(renderTagCollection(event.tags));
        row.append(tagsCell);
        renderTableCell(row, event.metadata?.description || "--");
        enableRow(row, () => void openEvent(event.ref_code, "aggregate"));
        return row;
      }));
    }
    const pageCount = Math.max(1, Math.ceil(state.aggregateEvents.length / eventPageSize));
    eventPagerSlot.replaceChildren(renderPager({
      currentPage: state.eventPage,
      hasPrevious: state.eventPage > 1,
      hasNext: state.eventPage < pageCount,
      onPage(page) {
        state.eventPage = page;
        renderAggregateDetail();
      },
    }));
  }

  function renderEventDetail() {
    const event = state.currentEvent;
    if (!event) {
      return;
    }
    const aggregate = aggregateByRef(event.aggregate_ref_code);
    eventDetailRef.textContent = event.ref_code;
    eventDetailTitle.textContent = event.metadata?.title ?? "Untitled event";
    const isVoided = event.status === "voided";
    const isFinished = event.status === "finished";
    eventDetailBadgeSlot.replaceChildren(renderStatusBadge(
      isVoided ? "Voided Event" : isFinished ? "Finished Event" : "Scheduled / Immutable",
      { state: isInactiveEventStatus(event.status) ? "off" : "warning" },
    ));
    eventDetailValues.aggregate.value.textContent = `${aggregate?.metadata?.title ?? "Aggregate"} / ${event.aggregate_ref_code}`;
    eventDetailValues.startsAt.value.textContent = formatLocalDateTime(event.starts_at);
    eventDetailValues.duration.value.textContent = `${event.duration_minutes} minutes`;
    eventDetailValues.status.value.textContent = titleCase(event.status);
    eventDetailValues.location.value.textContent = event.metadata?.location || aggregate?.metadata?.location || "--";
    eventDetailValues.tags.value.replaceChildren(renderTagCollection(event.tags));
    eventDetailValues.description.value.textContent = event.metadata?.description || "--";
    finishEventButton.hidden = event.status !== "scheduled";
    voidEventButton.hidden = isVoided;
  }

  async function loadOverview({ announce = true } = {}) {
    const requestNumber = ++state.overviewRequestNumber;
    const range = monthRange(state.monthCursor);
    if (announce) {
      setNotice("Calendar API", "Loading event aggregates and scheduled month events.", "info");
    }
    try {
      const [aggregates, viewEvents] = await Promise.all([
        getAllPages("/api/calendar/aggregates", "aggregates"),
        getAllPages(`/api/calendar/view?from=${encodeURIComponent(range.from)}&to=${encodeURIComponent(range.to)}`, "events"),
      ]);
      if (requestNumber !== state.overviewRequestNumber) {
        return false;
      }
      state.aggregates = aggregates;
      state.viewEvents = viewEvents;
      const pageCount = Math.max(1, Math.ceil(aggregates.length / aggregatePageSize));
      state.aggregatePage = Math.min(state.aggregatePage, pageCount);
      renderOverview();
      if (announce) {
        setNotice("Calendar API", `Loaded ${viewEvents.length} scheduled event${viewEvents.length === 1 ? "" : "s"} for ${formatMonthLabel(state.monthCursor)}.`, "info");
      }
      return true;
    } catch (error) {
      if (requestNumber !== state.overviewRequestNumber) {
        return false;
      }
      state.aggregates = [];
      state.viewEvents = [];
      renderOverview();
      setNotice("Unable to Load Calendar", error.message, "warning");
      return false;
    }
  }

  async function openAggregate(refCode, page = 1, { announce = true } = {}) {
    const requestNumber = ++state.aggregateRequestNumber;
    state.eventPage = page;
    setView("aggregate");
    if (announce) {
      setNotice("Calendar API", `Loading aggregate ${refCode}.`, "info");
    }
    try {
      const result = await getJSON(`/api/calendar/aggregates/${encodeURIComponent(refCode)}`);
      if (requestNumber !== state.aggregateRequestNumber) {
        return false;
      }
      state.selectedAggregate = result.aggregate;
      state.aggregateEvents = result.events ?? [];
      renderAggregateDetail();
      if (announce) {
        setNotice("Calendar API", `Showing ${state.aggregateEvents.length} event${state.aggregateEvents.length === 1 ? "" : "s"} for ${result.aggregate.metadata?.title ?? refCode}.`, "info");
      }
      return true;
    } catch (error) {
      if (requestNumber !== state.aggregateRequestNumber) {
        return false;
      }
      setNotice("Unable to Load Aggregate", error.message, "warning");
      return false;
    }
  }

  async function openEvent(refCode, returnView = "overview") {
    const requestNumber = ++state.eventRequestNumber;
    state.returnView = returnView;
    setView("event-detail");
    setNotice("Calendar API", `Loading event ${refCode}.`, "info");
    try {
      const result = await getJSON(`/api/calendar/events/${encodeURIComponent(refCode)}`);
      if (requestNumber !== state.eventRequestNumber) {
        return;
      }
      state.currentEvent = result.event;
      const aggregate = aggregateByRef(result.event.aggregate_ref_code);
      if (aggregate && state.selectedAggregate?.ref_code !== aggregate.ref_code) {
        state.selectedAggregate = aggregate;
      }
      renderEventDetail();
      if (result.event.status === "finished") {
        setNotice("Finished Event", "Finished events remain visible in aggregate detail and can still be voided.", "info");
      } else if (result.event.status === "voided") {
        setNotice("Voided Event", "Voided events remain visible in aggregate detail.", "info");
      } else {
        setNotice("Immutable Event", "Scheduled events cannot be edited. They can be finished or voided.", "info");
      }
    } catch (error) {
      if (requestNumber !== state.eventRequestNumber) {
        return;
      }
      setNotice("Unable to Load Event", error.message, "warning");
    }
  }

  function showAggregateForm(returnView = "overview") {
    state.formReturnView = returnView;
    aggregateForm.reset();
    aggregateForm.elements.timezone.value = defaultTimezone;
    setView("new-aggregate");
    setNotice("New Aggregate", "Create the aggregate first, then add events from its detail page.", "info");
  }

  function showEventForm(returnView = "aggregate") {
    if (!state.selectedAggregate) {
      setNotice("Select Aggregate", "Open an aggregate before adding events.", "warning");
      return;
    }
    state.eventFormReturnView = returnView;
    eventForm.reset();
    eventForm.elements.starts_date.value = state.selectedDate;
    eventForm.elements.starts_time.value = "09:00";
    eventForm.elements.duration_minutes.value = "60";
    eventForm.elements.recurrence_kind.value = "single";
    eventForm.elements.week_count.value = "1";
    const defaultWeekday = weekdayForDate(state.selectedDate);
    Array.from(eventForm.elements.weekday).forEach((checkbox) => {
      checkbox.checked = checkbox.value === defaultWeekday;
    });
    updateEventRecurrenceFields();
    setView("new-event");
    setNotice("Add Event", `Adding events under ${state.selectedAggregate.metadata?.title ?? state.selectedAggregate.ref_code}.`, "info");
  }

  function updateEventRecurrenceFields() {
    const isWeekly = eventForm.elements.recurrence_kind.value === "weekly";
    recurrenceFields.hidden = !isWeekly;
    Array.from(recurrenceFields.querySelectorAll("input")).forEach((input) => {
      input.disabled = !isWeekly;
    });
  }

  async function createAggregate(event) {
    event.preventDefault();
    setFormBusy(aggregateForm, true);
    try {
      const result = await postJSON("/api/calendar/aggregates", {
        metadata: {
          title: aggregateForm.elements.aggregate_title.value.trim(),
          description: aggregateForm.elements.aggregate_description.value,
          location: aggregateForm.elements.aggregate_location.value.trim(),
          timezone: aggregateForm.elements.timezone.value.trim(),
        },
        tags: splitTags(aggregateForm.elements.aggregate_tags.value),
      });
      await loadOverview({ announce: false });
      state.selectedAggregate = result.aggregate;
      state.aggregateEvents = result.events ?? [];
      state.eventPage = 1;
      renderAggregateDetail();
      setView("aggregate");
      setNotice("Aggregate Created", `${result.aggregate.metadata?.title ?? result.aggregate.ref_code} is ready for events.`, "info");
    } catch (error) {
      setNotice("Unable to Create Aggregate", error.message, "warning");
    } finally {
      setFormBusy(aggregateForm, false);
    }
  }

  async function createEvent(event) {
    event.preventDefault();
    const aggregate = state.selectedAggregate;
    if (!aggregate) {
      setNotice("Unable to Create Event", "Open an aggregate before adding events.", "warning");
      return;
    }
    const durationMinutes = Number(eventForm.elements.duration_minutes.value);
    const startsAt = combineLocalDateTime(eventForm.elements.starts_date.value, eventForm.elements.starts_time.value);
    if (!startsAt || !Number.isInteger(durationMinutes) || durationMinutes < 1) {
      setNotice("Unable to Create Event", "Provide a valid start date, time and positive duration.", "warning");
      return;
    }

    const recurrenceKind = eventForm.elements.recurrence_kind.value;
    const recurrence = { kind: recurrenceKind };
    if (recurrenceKind === "weekly") {
      const weekdays = Array.from(eventForm.elements.weekday)
        .filter((checkbox) => checkbox.checked)
        .map((checkbox) => checkbox.value);
      const weekCount = Number(eventForm.elements.week_count.value);
      if (weekdays.length === 0 || !Number.isInteger(weekCount) || weekCount < 1) {
        setNotice("Unable to Create Event", "Weekly recurrence needs at least one weekday and a positive week count.", "warning");
        return;
      }
      recurrence.weekdays = weekdays;
      recurrence.week_count = weekCount;
    }

    setFormBusy(eventForm, true);
    try {
      const result = await postJSON(`/api/calendar/aggregates/${encodeURIComponent(aggregate.ref_code)}/events`, {
        metadata: {
          title: eventForm.elements.event_title.value.trim(),
          description: eventForm.elements.event_description.value,
          location: eventForm.elements.event_location.value.trim(),
        },
        tags: splitTags(eventForm.elements.event_tags.value),
        starts_at: startsAt,
        duration_minutes: durationMinutes,
        recurrence,
      });
      const firstEvent = result.events?.[0];
      if (firstEvent) {
        state.selectedDate = formatEventDate(firstEvent);
        state.monthCursor = localMonthStart(parseDateValue(state.selectedDate));
      }
      await loadOverview({ announce: false });
      const detail = await getJSON(`/api/calendar/aggregates/${encodeURIComponent(result.aggregate.ref_code)}`);
      state.selectedAggregate = detail.aggregate;
      state.aggregateEvents = detail.events ?? [];
      state.eventPage = 1;
      renderAggregateDetail();
      setView("aggregate");
      const createdCount = result.events?.length ?? 0;
      setNotice("Event Created", `${createdCount} event${createdCount === 1 ? "" : "s"} added under ${result.aggregate.metadata?.title ?? result.aggregate.ref_code}.`, "info");
    } catch (error) {
      setNotice("Unable to Create Event", error.message, "warning");
    } finally {
      setFormBusy(eventForm, false);
      updateEventRecurrenceFields();
    }
  }

  async function deleteAggregate() {
    const aggregate = state.selectedAggregate;
    if (!aggregate || !window.confirm(`Delete ${aggregate.metadata?.title ?? aggregate.ref_code} and all child events?`)) {
      return;
    }
    deleteAggregateButton.disabled = true;
    try {
      await deleteJSON(`/api/calendar/aggregates/${encodeURIComponent(aggregate.ref_code)}`);
      state.selectedAggregate = null;
      state.aggregateEvents = [];
      state.currentEvent = null;
      await loadOverview({ announce: false });
      setView("overview");
      setNotice("Aggregate Deleted", `${aggregate.metadata?.title ?? aggregate.ref_code} and its child events were deleted.`, "info");
    } catch (error) {
      setNotice("Unable to Delete Aggregate", error.message, "warning");
    } finally {
      deleteAggregateButton.disabled = false;
    }
  }

  async function voidEvent() {
    const event = state.currentEvent;
    if (!event || !window.confirm(`Void ${event.ref_code}? This cannot be reversed.`)) {
      return;
    }
    voidEventButton.disabled = true;
    try {
      const result = await postJSON(`/api/calendar/events/${encodeURIComponent(event.ref_code)}/void`, {});
      state.currentEvent = result.event;
      if (state.selectedAggregate?.ref_code === result.event.aggregate_ref_code) {
        const detail = await getJSON(`/api/calendar/aggregates/${encodeURIComponent(result.event.aggregate_ref_code)}`);
        state.selectedAggregate = detail.aggregate;
        state.aggregateEvents = detail.events ?? [];
      }
      await loadOverview({ announce: false });
      renderEventDetail();
      setNotice("Event Voided", `${event.ref_code} no longer appears in the main CalendarView.`, "info");
    } catch (error) {
      setNotice("Unable to Void Event", error.message, "warning");
    } finally {
      voidEventButton.disabled = false;
    }
  }

  async function finishEvent() {
    const event = state.currentEvent;
    if (!event || !window.confirm(`Finish ${event.ref_code}?`)) {
      return;
    }
    finishEventButton.disabled = true;
    try {
      const result = await postJSON(`/api/calendar/events/${encodeURIComponent(event.ref_code)}/finish`, {});
      state.currentEvent = result.event;
      if (state.selectedAggregate?.ref_code === result.event.aggregate_ref_code) {
        const detail = await getJSON(`/api/calendar/aggregates/${encodeURIComponent(result.event.aggregate_ref_code)}`);
        state.selectedAggregate = detail.aggregate;
        state.aggregateEvents = detail.events ?? [];
      }
      await loadOverview({ announce: false });
      renderEventDetail();
      setNotice("Event Finished", `${event.ref_code} no longer appears in the main CalendarView.`, "info");
    } catch (error) {
      setNotice("Unable to Finish Event", error.message, "warning");
    } finally {
      finishEventButton.disabled = false;
    }
  }

  previousMonthButton.addEventListener("click", () => {
    state.monthCursor = new Date(state.monthCursor.getFullYear(), state.monthCursor.getMonth() - 1, 1);
    state.selectedDate = localDateValue(state.monthCursor);
    void loadOverview({ announce: false });
  });
  todayButton.addEventListener("click", () => {
    const today = new Date();
    state.monthCursor = localMonthStart(today);
    state.selectedDate = localDateValue(today);
    void loadOverview({ announce: false });
  });
  nextMonthButton.addEventListener("click", () => {
    state.monthCursor = new Date(state.monthCursor.getFullYear(), state.monthCursor.getMonth() + 1, 1);
    state.selectedDate = localDateValue(state.monthCursor);
    void loadOverview({ announce: false });
  });
  newAggregateButton.addEventListener("click", () => showAggregateForm("overview"));
  newEventButton.addEventListener("click", () => showEventForm("aggregate"));
  backToOverviewButton.addEventListener("click", () => {
    setView("overview");
    renderOverview();
    setNotice("Calendar API", "Showing scheduled month events and event aggregates.", "info");
  });
  deleteAggregateButton.addEventListener("click", () => void deleteAggregate());
  cancelAggregateButton.addEventListener("click", () => {
    const nextView = state.formReturnView === "aggregate" && state.selectedAggregate ? "aggregate" : "overview";
    setView(nextView);
    if (nextView === "aggregate") {
      renderAggregateDetail();
    } else {
      renderOverview();
    }
  });
  cancelEventButton.addEventListener("click", () => {
    const nextView = state.eventFormReturnView === "aggregate" && state.selectedAggregate ? "aggregate" : "overview";
    setView(nextView);
    if (nextView === "aggregate") {
      renderAggregateDetail();
    } else {
      renderOverview();
    }
  });
  eventForm.elements.recurrence_kind.addEventListener("change", updateEventRecurrenceFields);
  aggregateForm.addEventListener("submit", (event) => void createAggregate(event));
  eventForm.addEventListener("submit", (event) => void createEvent(event));
  backToAggregateButton.addEventListener("click", () => {
    if (state.returnView === "aggregate" && state.selectedAggregate) {
      setView("aggregate");
      renderAggregateDetail();
      return;
    }
    setView("overview");
    renderOverview();
  });
  finishEventButton.addEventListener("click", () => void finishEvent());
  voidEventButton.addEventListener("click", () => void voidEvent());

  setView("overview");
  void loadOverview();
}
