// This file renders the authenticated Accounting ledger workflow.
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

const accountPageSize = 10;
const transactionPageSize = 10;
const collectionPageSize = 100;
const transactionKinds = [
  ["expense", "Expense"],
  ["income", "Income"],
  ["adjustment", "Adjustment"],
];

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

function formatMoney(cents, currency) {
  try {
    return new Intl.NumberFormat("en-AU", {
      style: "currency",
      currency: currency || "AUD",
      currencyDisplay: "code",
    }).format(Number(cents ?? 0) / 100);
  } catch (_error) {
    return `${currency || "AUD"} ${(Number(cents ?? 0) / 100).toFixed(2)}`;
  }
}

function formatDate(value) {
  return String(value ?? "").slice(0, 10) || "--";
}

function localDateValue(date = new Date()) {
  const offset = date.getTimezoneOffset() * 60 * 1000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 10);
}

function monthRange() {
  const today = localDateValue();
  return {
    from: `${today.slice(0, 7)}-01`,
    to: today,
  };
}

function parseDecimalCents(value) {
  const match = String(value ?? "").trim().match(/^([+-]?)(\d+)(?:\.(\d{1,2}))?$/);
  if (!match) {
    return null;
  }
  const cents = Number(match[2]) * 100 + Number((match[3] ?? "").padEnd(2, "0"));
  if (!Number.isSafeInteger(cents)) {
    return null;
  }
  return match[1] === "-" ? -cents : cents;
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

function renderControlLabel(text) {
  const label = el("span", "control-label");
  label.textContent = text;
  return label;
}

function renderField({ label, name, type = "text", placeholder = "", value = "", required = false }) {
  const field = el("label", "control-field");
  const input = el("input", "field");
  input.name = name;
  input.type = type;
  input.placeholder = placeholder;
  input.value = value;
  input.required = required;
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

function renderMoneyField({ label, name, currency, value = "", required = false }) {
  const field = el("label", "control-field");
  const group = el("span", "input-group input-group--prefix");
  const prefix = document.createElement("span");
  prefix.setAttribute("aria-hidden", "true");
  prefix.textContent = currency;
  const input = document.createElement("input");
  input.name = name;
  input.type = "text";
  input.inputMode = "decimal";
  input.placeholder = "0.00";
  input.value = value;
  input.required = required;
  input.setAttribute("aria-label", label);
  group.append(prefix, input);
  field.append(renderControlLabel(label), group);
  return { field, prefix };
}

function renderTextarea({ label, name, placeholder = "" }) {
  const field = el("label", "control-field");
  const textarea = el("textarea", "textarea");
  textarea.name = name;
  textarea.placeholder = placeholder;
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

function renderTagCollection(tags) {
  const collection = el("div", "accounting-tags");
  (tags?.length ? tags : ["untagged"]).forEach((tag) => collection.append(renderTag(tag)));
  return collection;
}

export function renderAccountingPage(target) {
  const state = {
    accounts: [],
    monthlyTransactions: [],
    selectedAccount: null,
    transactions: [],
    currentTransaction: null,
    accountPage: 1,
    transactionPage: 1,
    transactionHasMore: false,
    overviewRequestNumber: 0,
    ledgerRequestNumber: 0,
    transactionRequestNumber: 0,
  };

  const module = el("div", "accounting-module");
  const notice = renderNotice({
    title: "Accounting API",
    message: "Loading your private ledgers.",
    tone: "info",
  });
  notice.classList.add("accounting-notice");
  const noticeTitle = notice.querySelector("strong");
  const noticeMessage = notice.querySelector("p");

  const accountsView = el("section", "accounting-view");
  accountsView.setAttribute("aria-label", "Account list");
  const summary = el("section", "accounting-summary-grid");
  summary.setAttribute("aria-label", "Accounting summary");
  const totalBalance = renderStat("Total Balance");
  const monthlyIn = renderStat("Monthly Cash Flow / In");
  const monthlyOut = renderStat("Monthly Cash Flow / Out");
  summary.append(totalBalance.card, monthlyIn.card, monthlyOut.card);

  const accountsPanel = renderSurface("section", { className: "section accounting-panel", label: "Accounts" });
  const accountsTable = renderTable("Accounting accounts", [
    { label: "Ref" },
    { label: "Account" },
    { label: "Tags" },
    { label: "Balance", className: "accounting-align-right" },
    { label: "Updated" },
  ]);
  const accountsFooter = el("footer", "accounting-footer");
  const accountPagerSlot = el("div", "accounting-pager-slot");
  const newAccountButton = renderButton("NEW", { variant: "primary", label: "New account" });
  accountsFooter.append(accountPagerSlot, newAccountButton);
  accountsPanel.append(accountsTable.element, accountsFooter);
  accountsView.append(summary, accountsPanel);

  const newAccountView = el("section", "accounting-view");
  newAccountView.hidden = true;
  newAccountView.setAttribute("aria-label", "New account");
  const accountForm = renderSurface("form", { className: "accounting-form", label: "Create account" });
  const accountFormHeader = el("header", "accounting-form__head");
  appendText(accountFormHeader, "span", "eyebrow", "Accounting / New");
  appendText(accountFormHeader, "h2", "accounting-form__title", "Create Ledger");
  appendText(accountFormHeader, "p", "accounting-form__text", "Create an account with tags, currency and an opening balance. Balances then change only through posted transactions.");
  const accountFields = el("div", "control-stack accounting-form__fields");
  const accountBasics = el("div", "control-split");
  accountBasics.append(
    renderField({ label: "Account Name", name: "name", placeholder: "Everyday Account", required: true }),
    renderField({ label: "Tags / Comma Separated", name: "tags", placeholder: "bank, daily" }),
  );
  const accountFinancials = el("div", "control-split");
  const currencyField = renderField({ label: "Currency", name: "currency", value: "AUD", required: true });
  const openingBalance = renderMoneyField({
    label: "Opening Balance",
    name: "opening_balance",
    currency: "AUD",
    value: "0.00",
    required: true,
  });
  accountFinancials.append(currencyField, openingBalance.field);
  accountFields.append(accountBasics, accountFinancials);
  const accountActions = el("footer", "accounting-form__actions");
  const cancelAccountButton = renderButton("RETURN");
  const saveAccountButton = renderButton("SAVE", { type: "submit", variant: "primary" });
  accountActions.append(cancelAccountButton, saveAccountButton);
  accountForm.append(accountFormHeader, accountFields, accountActions);
  newAccountView.append(accountForm);

  const ledgerView = el("section", "accounting-view");
  ledgerView.hidden = true;
  ledgerView.setAttribute("aria-label", "Account transactions");
  const accountInfoGrid = el("section", "accounting-ledger-info-grid");
  accountInfoGrid.setAttribute("aria-label", "Account information");
  const accountIdentity = renderSurface("article", { className: "accounting-stat accounting-ledger-identity" });
  appendText(accountIdentity, "span", "", "LEDGER");
  const accountName = appendText(accountIdentity, "strong", "accounting-ledger-title", "---");
  const accountLedgerMeta = el("div", "accounting-ledger-meta");
  const accountRef = appendText(accountLedgerMeta, "span", "ref-code accounting-ledger-ref", "---");
  const accountBadgeSlot = el("div", "accounting-stat__visual accounting-ledger-tags");
  accountLedgerMeta.append(accountBadgeSlot);
  accountIdentity.append(accountLedgerMeta);
  const accountOpening = renderStat("Opening Balance");
  const accountBalance = renderStat("Current Balance");
  accountInfoGrid.append(accountIdentity, accountOpening.card, accountBalance.card);
  const backToAccountsButton = renderButton("RETURN", { label: "Return to accounts" });
  const ledgerActions = el("div", "actions accounting-ledger-actions");
  const newTransactionButton = renderButton("NEW", { variant: "primary", label: "New transaction" });
  const deleteAccountButton = renderButton("DELETE", { variant: "danger", label: "Delete account" });
  ledgerActions.append(backToAccountsButton, newTransactionButton, deleteAccountButton);

  const transactionPanel = renderSurface("section", { className: "section accounting-panel", label: "Transactions" });
  const transactionTable = renderTable("Selected account transactions", [
    { label: "Ref" },
    { label: "Date" },
    { label: "Title" },
    { label: "Status" },
    { label: "Tags" },
    { label: "Amount", className: "accounting-align-right" },
  ]);
  const transactionFooter = el("footer", "accounting-footer");
  const transactionPagerSlot = el("div", "accounting-pager-slot");
  transactionFooter.append(transactionPagerSlot, ledgerActions);
  transactionPanel.append(transactionTable.element, transactionFooter);
  ledgerView.append(accountInfoGrid, transactionPanel);

  const newTransactionView = el("section", "accounting-view");
  newTransactionView.hidden = true;
  newTransactionView.setAttribute("aria-label", "New transaction");
  const transactionForm = renderSurface("form", { className: "accounting-form", label: "Create transaction" });
  const transactionFormHeader = el("header", "accounting-form__head");
  appendText(transactionFormHeader, "span", "eyebrow", "Accounting / Transaction");
  const transactionFormTitle = appendText(transactionFormHeader, "h2", "accounting-form__title", "New Transaction");
  appendText(transactionFormHeader, "p", "accounting-form__text", "New transactions are posted immediately. Account, amount, date, title, note and tags cannot be edited afterwards.");
  const transactionFields = el("div", "control-stack accounting-form__fields");
  const transactionBasics = el("div", "control-split");
  transactionBasics.append(
    renderField({ label: "Title", name: "title", placeholder: "Lunch" }),
    renderField({ label: "Occurred On", name: "occurred_on", type: "date", value: localDateValue(), required: true }),
  );
  const transactionFinancials = el("div", "control-split");
  const amount = renderMoneyField({
    label: "Amount",
    name: "amount",
    currency: "AUD",
    required: true,
  });
  transactionFinancials.append(
    amount.field,
    renderSelect({ label: "Kind", name: "kind", options: transactionKinds, value: "expense" }),
  );
  const tagsField = renderField({ label: "Tags / Comma Separated", name: "tags", placeholder: "food, work" });
  const noteField = renderTextarea({ label: "Note", name: "note", placeholder: "Receipt FIL-00000008" });
  transactionFields.append(transactionBasics, transactionFinancials, tagsField, noteField);
  const transactionActions = el("footer", "accounting-form__actions");
  const cancelTransactionButton = renderButton("RETURN");
  const saveTransactionButton = renderButton("SAVE", { type: "submit", variant: "primary" });
  transactionActions.append(cancelTransactionButton, saveTransactionButton);
  transactionForm.append(transactionFormHeader, transactionFields, transactionActions);
  newTransactionView.append(transactionForm);

  const transactionDetailView = el("section", "accounting-view");
  transactionDetailView.hidden = true;
  transactionDetailView.setAttribute("aria-label", "Transaction detail");
  const transactionDetail = renderSurface("section", { className: "accounting-transaction-detail", label: "Transaction detail" });
  const detailHead = el("header", "accounting-detail__head accounting-transaction-detail__head");
  const detailCopy = el("div", "accounting-detail__copy");
  const detailRef = appendText(detailCopy, "span", "ref-code", "---");
  const detailTitle = appendText(detailCopy, "h2", "accounting-detail__title", "---");
  const detailBadgeSlot = el("div", "accounting-detail__badge");
  detailHead.append(detailCopy, detailBadgeSlot);
  const detailGrid = el("div", "accounting-readonly-grid");
  const detailValues = {
    account: renderReadonlyDetail("Account"),
    date: renderReadonlyDetail("Date"),
    kind: renderReadonlyDetail("Kind"),
    amount: renderReadonlyDetail("Amount"),
    tags: renderReadonlyDetail("Tags", "accounting-readonly--wide"),
    note: renderReadonlyDetail("Note", "accounting-readonly--wide accounting-readonly--note"),
  };
  Object.values(detailValues).forEach((item) => detailGrid.append(item.element));
  const detailFooter = el("footer", "accounting-footer");
  const backToLedgerButton = renderButton("RETURN");
  const detailActions = el("div", "actions accounting-ledger-actions");
  const voidButton = renderButton("VOID", { variant: "danger", label: "Void transaction" });
  detailActions.append(backToLedgerButton, voidButton);
  detailFooter.append(detailActions);
  transactionDetail.append(detailHead, detailGrid, detailFooter);
  transactionDetailView.append(transactionDetail);

  module.append(accountsView, newAccountView, ledgerView, newTransactionView, transactionDetailView);
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
    accountsView.hidden = view !== "accounts";
    newAccountView.hidden = view !== "new-account";
    ledgerView.hidden = view !== "ledger";
    newTransactionView.hidden = view !== "new-transaction";
    transactionDetailView.hidden = view !== "transaction-detail";
  }

  function setFormBusy(form, busy) {
    Array.from(form.elements).forEach((element) => {
      element.disabled = busy;
    });
  }

  function summaryCurrency() {
    if (state.accounts.some((account) => account.currency === "AUD")) {
      return "AUD";
    }
    return state.accounts[0]?.currency ?? "AUD";
  }

  function renderOverview() {
    const currencies = new Map();
    state.accounts.forEach((account) => {
      currencies.set(account.currency, (currencies.get(account.currency) ?? 0) + account.balance_cents);
    });
    const currency = summaryCurrency();
    const currencyBalance = currencies.get(currency) ?? 0;
    totalBalance.value.textContent = formatMoney(currencyBalance, currency);
    totalBalance.visual.replaceChildren();
    totalBalance.note.textContent = currencies.size > 1
      ? `Showing ${currency}; additional balances: ${Array.from(currencies.entries())
        .filter(([code]) => code !== currency)
        .map(([code, cents]) => formatMoney(cents, code))
        .join(", ")}.`
      : "Only records and aggregates balances; no financial advice.";

    const accountCurrencies = new Map(state.accounts.map((account) => [account.ref_code, account.currency]));
    const posted = state.monthlyTransactions.filter((transaction) => (
      transaction.status === "posted" && accountCurrencies.get(transaction.account_ref_code) === currency
    ));
    const incomeCents = posted
      .filter((transaction) => transaction.amount_cents > 0)
      .reduce((total, transaction) => total + transaction.amount_cents, 0);
    const outgoingCents = Math.abs(posted
      .filter((transaction) => transaction.amount_cents < 0)
      .reduce((total, transaction) => total + transaction.amount_cents, 0));
    const cashFlowTotal = incomeCents + outgoingCents;
    const incomePercent = cashFlowTotal === 0 ? 0 : Math.round((incomeCents / cashFlowTotal) * 1000) / 10;
    const outgoingPercent = cashFlowTotal === 0 ? 0 : Math.round((outgoingCents / cashFlowTotal) * 1000) / 10;
    monthlyIn.value.textContent = formatMoney(incomeCents, currency);
    monthlyIn.visual.replaceChildren(renderMeter({ label: "In", value: incomePercent, tone: "success" }));
    monthlyIn.note.textContent = `Posted positive amounts in ${currency} for the current month.`;
    monthlyOut.value.textContent = formatMoney(outgoingCents, currency);
    monthlyOut.visual.replaceChildren(renderMeter({ label: "Out", value: outgoingPercent, tone: "danger" }));
    monthlyOut.note.textContent = `Posted negative amounts in ${currency}; voided entries excluded.`;
  }

  function renderAccounts() {
    const firstIndex = (state.accountPage - 1) * accountPageSize;
    const visibleAccounts = state.accounts.slice(firstIndex, firstIndex + accountPageSize);
    if (visibleAccounts.length === 0) {
      renderEmptyRow(accountsTable.body, 5, "No accounts found. Create a ledger to start recording transactions.");
    } else {
      accountsTable.body.replaceChildren(...visibleAccounts.map((account) => {
        const row = el("tr", "accounting-row");
        row.setAttribute("aria-label", `Open ${account.name} ledger`);
        renderTableCell(row, account.ref_code, "ref-code");
        renderTableCell(row, account.name, "accounting-name");
        const tagsCell = document.createElement("td");
        tagsCell.append(renderTagCollection(account.tags));
        row.append(tagsCell);
        renderTableCell(
          row,
          formatMoney(account.balance_cents, account.currency),
          `accounting-align-right accounting-amount ${account.balance_cents < 0 ? "accounting-amount--negative" : ""}`,
        );
        renderTableCell(row, formatDate(account.updated_at));
        enableRow(row, () => void openLedger(account.ref_code, 1));
        return row;
      }));
    }
    const pageCount = Math.max(1, Math.ceil(state.accounts.length / accountPageSize));
    accountPagerSlot.replaceChildren(renderPager({
      currentPage: state.accountPage,
      hasPrevious: state.accountPage > 1,
      hasNext: state.accountPage < pageCount,
      onPage(page) {
        state.accountPage = page;
        renderAccounts();
      },
    }));
  }

  function renderLedger() {
    const account = state.selectedAccount;
    if (!account) {
      return;
    }
    accountRef.textContent = account.ref_code;
    accountName.textContent = account.name;
    accountBadgeSlot.replaceChildren(renderTagCollection(account.tags));
    accountOpening.value.textContent = formatMoney(account.opening_balance_cents, account.currency);
    accountOpening.visual.replaceChildren();
    accountOpening.note.textContent = "Initial ledger balance";
    accountBalance.value.textContent = formatMoney(account.balance_cents, account.currency);
    accountBalance.value.classList.toggle("accounting-negative", account.balance_cents < 0);
    accountBalance.visual.replaceChildren();
    accountBalance.note.textContent = "Opening balance plus posted transactions";

    if (state.transactions.length === 0) {
      renderEmptyRow(transactionTable.body, 6, "No transactions found for this ledger.");
    } else {
      transactionTable.body.replaceChildren(...state.transactions.map((transaction) => {
        const row = el("tr", "accounting-row");
        const title = transaction.title || `${titleCase(transaction.kind)} ${transaction.occurred_on}`;
        row.setAttribute("aria-label", `Open ${title} transaction`);
        renderTableCell(row, transaction.ref_code, "ref-code");
        renderTableCell(row, transaction.occurred_on);
        renderTableCell(row, title, "accounting-name");
        const statusCell = document.createElement("td");
        const statusTag = renderTag(transaction.status);
        if (transaction.status === "voided") {
          statusTag.classList.add("tag--voided");
        }
        statusCell.append(statusTag);
        row.append(statusCell);
        const tagCell = document.createElement("td");
        const tagList = renderTagCollection(transaction.tags);
        tagCell.append(tagList);
        row.append(tagCell);
        renderTableCell(
          row,
          formatMoney(transaction.amount_cents, account.currency),
          `accounting-align-right accounting-amount ${transaction.status === "voided"
            ? "accounting-amount--voided"
            : transaction.amount_cents < 0
              ? "accounting-amount--negative"
              : "accounting-amount--positive"}`,
        );
        enableRow(row, () => void openTransaction(transaction.ref_code));
        return row;
      }));
    }
    transactionPagerSlot.replaceChildren(renderPager({
      currentPage: state.transactionPage,
      hasPrevious: state.transactionPage > 1,
      hasNext: state.transactionHasMore,
      onPage(page) {
        void openLedger(account.ref_code, page);
      },
    }));
  }

  function renderTransactionDetail() {
    const transaction = state.currentTransaction;
    const account = state.selectedAccount;
    if (!transaction || !account) {
      return;
    }
    const title = transaction.title || `${titleCase(transaction.kind)} ${transaction.occurred_on}`;
    detailRef.textContent = transaction.ref_code;
    detailTitle.textContent = title;
    detailBadgeSlot.replaceChildren(renderStatusBadge(
      transaction.status === "voided" ? "Voided Record" : "Posted / Immutable",
      { state: transaction.status === "voided" ? "off" : "warning" },
    ));
    detailValues.account.value.textContent = `${account.name} / ${account.ref_code}`;
    detailValues.date.value.textContent = transaction.occurred_on;
    detailValues.kind.value.textContent = titleCase(transaction.kind);
    detailValues.amount.value.textContent = formatMoney(transaction.amount_cents, account.currency);
    detailValues.tags.value.replaceChildren(renderTagCollection(transaction.tags));
    detailValues.note.value.textContent = transaction.note || "--";
    voidButton.hidden = transaction.status === "voided";
  }

  async function loadOverview({ announce = true } = {}) {
    const requestNumber = ++state.overviewRequestNumber;
    const range = monthRange();
    if (announce) {
      setNotice("Accounting API", "Loading account balances and current-month cash flow.", "info");
    }
    try {
      const [accounts, monthlyTransactions] = await Promise.all([
        getAllPages("/api/accounting/accounts", "accounts"),
        getAllPages(`/api/accounting/transactions?from=${range.from}&to=${range.to}`, "transactions"),
      ]);
      if (requestNumber !== state.overviewRequestNumber) {
        return false;
      }
      state.accounts = accounts;
      state.monthlyTransactions = monthlyTransactions;
      const pageCount = Math.max(1, Math.ceil(accounts.length / accountPageSize));
      state.accountPage = Math.min(state.accountPage, pageCount);
      renderOverview();
      renderAccounts();
      if (announce) {
        setNotice("Accounting API", `Loaded ${accounts.length} private ledger${accounts.length === 1 ? "" : "s"}.`, "info");
      }
      return true;
    } catch (error) {
      if (requestNumber !== state.overviewRequestNumber) {
        return false;
      }
      state.accounts = [];
      state.monthlyTransactions = [];
      renderOverview();
      renderAccounts();
      setNotice("Unable to Load Accounting", error.message, "warning");
      return false;
    }
  }

  async function openLedger(refCode, page = 1, { announce = true } = {}) {
    const requestNumber = ++state.ledgerRequestNumber;
    state.transactionPage = page;
    setView("ledger");
    if (announce) {
      setNotice("Accounting API", `Loading ledger ${refCode}.`, "info");
    }
    try {
      const offset = (page - 1) * transactionPageSize;
      const [accountResult, transactionResult] = await Promise.all([
        getJSON(`/api/accounting/accounts/${encodeURIComponent(refCode)}`),
        getJSON(`/api/accounting/transactions?account_ref_code=${encodeURIComponent(refCode)}&limit=${transactionPageSize}&offset=${offset}`),
      ]);
      if (requestNumber !== state.ledgerRequestNumber) {
        return false;
      }
      if ((transactionResult.transactions ?? []).length === 0 && page > 1) {
        return openLedger(refCode, page - 1, { announce });
      }
      state.selectedAccount = accountResult.account;
      state.transactions = transactionResult.transactions ?? [];
      state.transactionPage = page;
      state.transactionHasMore = Boolean(transactionResult.pagination?.has_more);
      renderLedger();
      if (announce) {
        setNotice("Accounting API", `Showing posted and voided records for ${accountResult.account.name}.`, "info");
      }
      return true;
    } catch (error) {
      if (requestNumber !== state.ledgerRequestNumber) {
        return false;
      }
      setNotice("Unable to Load Ledger", error.message, "warning");
      return false;
    }
  }

  async function openTransaction(refCode) {
    const requestNumber = ++state.transactionRequestNumber;
    setView("transaction-detail");
    setNotice("Accounting API", `Loading immutable record ${refCode}.`, "info");
    try {
      const result = await getJSON(`/api/accounting/transactions/${encodeURIComponent(refCode)}`);
      if (requestNumber !== state.transactionRequestNumber) {
        return;
      }
      state.currentTransaction = result.transaction;
      renderTransactionDetail();
      setNotice("Immutable Transaction", "Posted records cannot be edited. A posted record can only be voided.", "info");
    } catch (error) {
      if (requestNumber !== state.transactionRequestNumber) {
        return;
      }
      setNotice("Unable to Load Transaction", error.message, "warning");
    }
  }

  function showAccountForm() {
    accountForm.reset();
    accountForm.elements.currency.value = "AUD";
    accountForm.elements.opening_balance.value = "0.00";
    openingBalance.prefix.textContent = "AUD";
    setView("new-account");
    setNotice("New Account", "Opening balance is stored in cents; currency uses a three-letter code.", "info");
  }

  function showTransactionForm() {
    if (!state.selectedAccount) {
      return;
    }
    transactionForm.reset();
    transactionForm.elements.occurred_on.value = localDateValue();
    transactionForm.elements.kind.value = "expense";
    amount.prefix.textContent = state.selectedAccount.currency;
    transactionFormTitle.textContent = `New Transaction / ${state.selectedAccount.name}`;
    setView("new-transaction");
    setNotice("New Transaction", "Expenses are recorded as negative amounts; income as positive amounts.", "info");
  }

  async function createAccount(event) {
    event.preventDefault();
    const cents = parseDecimalCents(accountForm.elements.opening_balance.value);
    const currency = accountForm.elements.currency.value.trim().toUpperCase();
    if (cents === null || !/^[A-Z]{3}$/.test(currency)) {
      setNotice("Unable to Create Account", "Provide a valid amount and a three-letter currency code.", "warning");
      return;
    }
    setFormBusy(accountForm, true);
    try {
      const result = await postJSON("/api/accounting/accounts", {
        name: accountForm.elements.name.value.trim(),
        currency,
        opening_balance_cents: cents,
        tags: splitTags(accountForm.elements.tags.value),
      });
      await loadOverview({ announce: false });
      await openLedger(result.account.ref_code, 1, { announce: false });
      setNotice("Account Created", `${result.account.name} is ready for posted transactions.`, "info");
    } catch (error) {
      setNotice("Unable to Create Account", error.message, "warning");
    } finally {
      setFormBusy(accountForm, false);
    }
  }

  async function createTransaction(event) {
    event.preventDefault();
    const account = state.selectedAccount;
    if (!account) {
      return;
    }
    const kind = transactionForm.elements.kind.value;
    let cents = parseDecimalCents(transactionForm.elements.amount.value);
    if (cents === null || cents === 0) {
      setNotice("Unable to Create Transaction", "Amount must be a non-zero value with up to two decimal places.", "warning");
      return;
    }
    if (kind === "income") {
      cents = Math.abs(cents);
    } else if (kind === "expense") {
      cents = -Math.abs(cents);
    }
    setFormBusy(transactionForm, true);
    try {
      await postJSON("/api/accounting/transactions", {
        account_ref_code: account.ref_code,
        occurred_on: transactionForm.elements.occurred_on.value,
        kind,
        amount_cents: cents,
        title: transactionForm.elements.title.value.trim(),
        note: transactionForm.elements.note.value,
        tags: splitTags(transactionForm.elements.tags.value),
      });
      await loadOverview({ announce: false });
      await openLedger(account.ref_code, 1, { announce: false });
      setNotice("Transaction Posted", "The immutable transaction has been written to the ledger.", "info");
    } catch (error) {
      setNotice("Unable to Create Transaction", error.message, "warning");
    } finally {
      setFormBusy(transactionForm, false);
    }
  }

  async function deleteAccount() {
    const account = state.selectedAccount;
    if (!account || !window.confirm(`Delete ${account.name} and all of its transactions?`)) {
      return;
    }
    deleteAccountButton.disabled = true;
    try {
      await deleteJSON(`/api/accounting/accounts/${encodeURIComponent(account.ref_code)}`);
      state.selectedAccount = null;
      state.currentTransaction = null;
      await loadOverview({ announce: false });
      setView("accounts");
      setNotice("Account Deleted", `${account.name} and its ledger records were deleted.`, "info");
    } catch (error) {
      setNotice("Unable to Delete Account", error.message, "warning");
    } finally {
      deleteAccountButton.disabled = false;
    }
  }

  async function voidTransaction() {
    const transaction = state.currentTransaction;
    const account = state.selectedAccount;
    if (!transaction || !account || !window.confirm(`Void ${transaction.ref_code}? This cannot be reversed.`)) {
      return;
    }
    voidButton.disabled = true;
    try {
      const result = await postJSON(`/api/accounting/transactions/${encodeURIComponent(transaction.ref_code)}/void`, {});
      state.currentTransaction = result.transaction;
      await loadOverview({ announce: false });
      await openLedger(account.ref_code, state.transactionPage, { announce: false });
      setView("transaction-detail");
      renderTransactionDetail();
      setNotice("Transaction Voided", `${transaction.ref_code} no longer contributes to the account balance.`, "info");
    } catch (error) {
      setNotice("Unable to Void Transaction", error.message, "warning");
    } finally {
      voidButton.disabled = false;
    }
  }

  newAccountButton.addEventListener("click", showAccountForm);
  cancelAccountButton.addEventListener("click", () => setView("accounts"));
  accountForm.elements.currency.addEventListener("input", () => {
    openingBalance.prefix.textContent = accountForm.elements.currency.value.trim().toUpperCase() || "---";
  });
  accountForm.addEventListener("submit", (event) => void createAccount(event));
  backToAccountsButton.addEventListener("click", () => {
    setView("accounts");
    setNotice("Accounting API", "Showing your private ledgers and current-month totals.", "info");
  });
  newTransactionButton.addEventListener("click", showTransactionForm);
  deleteAccountButton.addEventListener("click", () => void deleteAccount());
  cancelTransactionButton.addEventListener("click", () => {
    setView("ledger");
    setNotice("Accounting API", `Showing posted and voided records for ${state.selectedAccount?.name ?? "this ledger"}.`, "info");
  });
  transactionForm.addEventListener("submit", (event) => void createTransaction(event));
  backToLedgerButton.addEventListener("click", () => {
    setView("ledger");
    renderLedger();
  });
  voidButton.addEventListener("click", () => void voidTransaction());

  setView("accounts");
  void loadOverview();
}
