// This file renders administrator operations for audit logs, API keys, and account controls.
import { getJSON, logout as logoutSession, patchJSON, postJSON } from "../../shared/api/client.js";
import {
  renderButton,
  renderDataTable,
  renderInputGroupField,
  renderNotice,
  renderSelectField,
  renderSection,
  renderTag,
} from "../../shared/components/primitives.js";

function el(tagName, className, text) {
  const node = document.createElement(tagName);
  if (className) {
    node.className = className;
  }
  if (text !== undefined) {
    node.textContent = text;
  }
  return node;
}

function settingsHash(parameters = {}) {
  const query = new URLSearchParams(parameters);
  const serialized = query.toString();
  return serialized ? `#settings?${serialized}` : "#settings";
}

function navigateSettings(parameters = {}) {
  window.location.hash = settingsHash(parameters);
}

function renderTextField({
  label,
  name,
  type = "text",
  placeholder = "",
  value = "",
  autocomplete = "",
  required = false,
}) {
  const field = el("label", "control-field");
  const labelNode = el("span", "control-label", label);
  const input = el("input", "field");
  input.name = name;
  input.type = type;
  input.placeholder = placeholder;
  input.value = value;
  input.required = required;
  input.setAttribute("aria-label", label);
  if (autocomplete) {
    input.autocomplete = autocomplete;
  }
  field.append(labelNode, input);
  return field;
}

function setFormBusy(form, busy) {
  Array.from(form.elements).forEach((element) => {
    element.disabled = busy;
  });
}

function actorLabel(log, apiKeyNames) {
  const refCode = log.actor_ref_code ?? "unknown";
  const name = apiKeyNames.get(refCode);
  return name ? `${name} · ${refCode}` : refCode;
}

function renderDetailList(items) {
  const list = el("dl", "settings-detail-list");
  items.forEach(({ label, value }) => {
    const term = el("dt", "", label);
    const description = el("dd");
    if (value instanceof Node) {
      description.append(value);
    } else {
      description.textContent = value ?? "";
    }
    list.append(term, description);
  });
  return list;
}

function renderDetailPanel({ title, note, children = [] }) {
  const panel = el("section", "settings-detail-panel");
  const header = el("header", "settings-detail-head");
  header.append(el("h3", "settings-detail-title", title));
  if (note) {
    header.append(el("p", "settings-detail-text", note));
  }
  panel.append(header, ...children.filter(Boolean));
  return panel;
}

function renderChoice({ code, meta, title, description, onOpen }) {
  const choice = el("button", "settings-choice");
  choice.type = "button";
  choice.addEventListener("click", onOpen);

  const head = el("span", "settings-choice__head");
  head.append(el("span", "settings-choice__code", code), el("span", "settings-choice__meta", meta));

  const copy = el("span", "settings-choice__copy");
  copy.append(el("span", "settings-choice__title", title), el("span", "settings-choice__text", description));

  choice.append(head, copy);
  return choice;
}

function auditLogTable(logs, apiKeyNames) {
  return renderDataTable({
    caption: "Append-only audit logs",
    columns: [
      { key: "time", label: "Time" },
      { key: "actor", label: "Actor" },
      { key: "action", label: "Action" },
      { key: "target", label: "Target", className: "ref-code" },
      { key: "result", label: "Result" },
      { key: "source", label: "Source IP" },
      { key: "reason", label: "Reason" },
    ],
    rows: logs.map((log) => ({
      time: log.created_at,
      actor: actorLabel(log, apiKeyNames),
      action: log.action,
      target: log.target_ref_code,
      result: renderTag(log.result),
      source: log.source_ip,
      reason: log.reason ?? "",
    })),
  });
}

function renderAuditPage(target) {
  const output = document.createElement("div");
  output.setAttribute("aria-live", "polite");
  output.className = "settings-feedback";

  const form = document.createElement("form");
  form.className = "control-stack audit-filter-bar";
  const filterSplit = document.createElement("div");
  filterSplit.className = "control-split";
  filterSplit.append(
    renderSelectField({
      label: "Action",
      name: "action",
      options: [
        ["", "All actions"],
        ["CREATE", "CREATE"],
        ["READ", "READ"],
        ["UPDATE", "UPDATE"],
        ["DELETE", "DELETE"],
        ["EXPORT", "EXPORT"],
        ["LOGIN", "LOGIN"],
        ["LOGOUT", "LOGOUT"],
      ],
    }),
    renderSelectField({
      label: "Result",
      name: "result",
      options: [
        ["", "All results"],
        ["SUCCESS", "SUCCESS"],
        ["FAILED", "FAILED"],
        ["DENIED", "DENIED"],
      ],
    }),
  );
  const actions = document.createElement("div");
  actions.className = "audit-filter-actions";
  const refresh = renderButton("RUN", { type: "submit", variant: "primary", label: "Run audit log search" });
  actions.append(refresh);
  form.append(
    renderInputGroupField({
      label: "Target Ref Code",
      name: "target_ref_code",
      prefix: "REF",
      suffix: "ID",
      placeholder: "NTE-00000001",
      type: "search",
    }),
    filterSplit,
    actions,
  );

  async function loadAuditLogs() {
    refresh.disabled = true;
    output.replaceChildren(renderNotice({
      title: "Loading audit logs",
      message: "Reading append-only audit records.",
      tone: "info",
    }));
    const query = new URLSearchParams(new FormData(form));
    query.set("limit", "50");
    try {
      const [result, keyResult] = await Promise.all([
        getJSON(`/api/platform/audit-logs?${query}`),
        getJSON("/api/auth/api-keys").catch(() => ({ api_keys: [] })),
      ]);
      const logs = Array.isArray(result.audit_logs) ? result.audit_logs : [];
      const keys = Array.isArray(keyResult.api_keys) ? keyResult.api_keys : [];
      const apiKeyNames = new Map(keys.map((key) => [key.refcode, key.name]));
      output.replaceChildren(logs.length > 0
        ? auditLogTable(logs, apiKeyNames)
        : renderNotice({ title: "No audit logs", message: "No records match the current filters.", tone: "info" }));
    } catch (error) {
      output.replaceChildren(renderNotice({ title: "Audit logs unavailable", message: error.message }));
    } finally {
      refresh.disabled = false;
    }
  }

  form.addEventListener("submit", (event) => {
    event.preventDefault();
    loadAuditLogs();
  });

  target.append(renderSection({
    title: "Audit",
    note: "Superuser read-only audit access",
    actions: [renderButton("RETURN", { label: "Return to settings selector", onClick: () => navigateSettings() })],
    children: [form, output],
  }));
  loadAuditLogs();
}

function renderAccountOverview(context) {
  const user = context.user ?? {};
  return renderDetailPanel({
    title: "Account Detail",
    note: "Current authenticated principal and available account operations.",
    children: [renderDetailList([
      { label: "RefCode", value: user.refcode ?? "unknown" },
      { label: "Email", value: user.email || "empty" },
      { label: "Principal", value: user.kind ?? "administrator" },
      { label: "Password", value: "current user password change requires the current password" },
      { label: "Logout", value: "revokes the current JWT session" },
    ])],
  });
}

function renderAPIKeyCell(primary, secondary) {
  const cell = el("div", "api-key-cell");
  cell.append(el("strong", "api-key-cell__primary", primary));
  if (secondary) {
    cell.append(el("span", "api-key-cell__secondary", secondary));
  }
  return cell;
}

function renderAPIKeySecretDialog(result, onClose) {
  const dialog = el("dialog", "api-key-dialog");
  dialog.setAttribute("aria-labelledby", "api-key-secret-title");

  const content = el("div", "api-key-dialog__content");
  const header = el("header", "api-key-dialog__head");
  const heading = el("div");
  heading.append(
    el("span", "api-key-dialog__eyebrow", "One-time secret"),
    el("h2", "api-key-dialog__title", "Save this API key now"),
  );
  heading.lastElementChild.id = "api-key-secret-title";
  header.append(heading, renderTag(result.name));

  const message = el(
    "p",
    "api-key-dialog__message",
    "This is the only time the complete key will be shown. Closing this window permanently removes it from the page.",
  );

  const secretField = el("label", "control-field");
  secretField.append(el("span", "control-label", "API key"));
  const secretRow = el("span", "api-key-dialog__secret-row");
  const secretInput = el("input", "field api-key-dialog__secret");
  secretInput.type = "text";
  secretInput.readOnly = true;
  secretInput.value = result.api_key;
  secretInput.spellcheck = false;
  secretInput.autocomplete = "off";
  secretInput.setAttribute("aria-label", "New API key secret");
  secretInput.addEventListener("focus", () => secretInput.select());

  const copy = renderButton("COPY", { label: "Copy the new API key" });
  const feedback = el("p", "api-key-dialog__feedback", "Store it in your password manager or client configuration.");
  feedback.setAttribute("aria-live", "polite");
  copy.addEventListener("click", async () => {
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(secretInput.value);
      } else {
        secretInput.focus();
        secretInput.select();
        if (!document.execCommand("copy")) {
          throw new Error("Copy is unavailable");
        }
      }
      copy.textContent = "COPIED";
      feedback.textContent = "Copied. Keep it somewhere secure before closing this window.";
    } catch (_error) {
      secretInput.focus();
      secretInput.select();
      feedback.textContent = "Automatic copy is unavailable. The key is selected for manual copy.";
    }
  });
  secretRow.append(secretInput, copy);
  secretField.append(secretRow);

  const actions = el("footer", "api-key-dialog__actions");
  const close = renderButton("I SAVED IT", { variant: "primary", label: "Close the one-time API key window" });
  close.addEventListener("click", () => dialog.close());
  actions.append(close);
  content.append(header, message, secretField, feedback, actions);
  dialog.append(content);

  dialog.addEventListener("close", () => {
    secretInput.value = "";
    dialog.replaceChildren();
    dialog.remove();
    onClose?.();
  }, { once: true });
  return dialog;
}

function renderAPIKeyStatusTable(keys, onRevoke) {
  return renderDataTable({
    caption: "Existing API keys",
    columns: [
      { key: "credential", label: "Credential" },
      { key: "access", label: "Access" },
      { key: "status", label: "Status" },
      { key: "activity", label: "Activity" },
      { key: "action", label: "Action" },
    ],
    rows: keys.map((key) => ({
      credential: renderAPIKeyCell(key.name, `${key.refcode} · ${key.key_prefix}`),
      access: renderAPIKeyCell((key.scopes ?? []).join(", ") || "none"),
      status: renderTag(key.status),
      activity: renderAPIKeyCell(
        `Last used: ${key.last_used_at ?? "never"}`,
        `Expires: ${key.expires_at ?? "never"}`,
      ),
      action: key.status === "ACTIVE"
        ? renderButton("REVOKE", { label: `Revoke ${key.name}`, onClick: () => onRevoke(key) })
        : "—",
    })),
  });
}

function renderAPIKeysPage(target) {
  const form = el("form", "settings-form");
  const output = el("div", "settings-feedback");
  output.setAttribute("aria-live", "polite");

  const createRegion = el("section", "api-key-region api-key-create");
  const createHeader = el("header", "api-key-region__head");
  const createHeading = el("h3", "api-key-region__title", "Create a key");
  createHeading.id = "create-api-key-title";
  createHeader.append(
    createHeading,
    el("p", "api-key-region__note", "Choose the smallest access scope your client needs."),
  );
  createRegion.setAttribute("aria-labelledby", createHeading.id);

  const listRegion = el("section", "api-key-region api-key-list");
  const listHeader = el("header", "api-key-region__head");
  const listHeading = el("h3", "api-key-region__title", "Existing keys");
  listHeading.id = "existing-api-keys-title";
  const listCount = el("span", "api-key-count", "—");
  listHeader.append(listHeading, listCount);
  listRegion.setAttribute("aria-labelledby", listHeading.id);

  const listOutput = el("div", "settings-feedback");
  listOutput.setAttribute("aria-live", "polite");

  const fields = el("div", "api-key-form-grid");
  fields.append(
    renderTextField({
      label: "Name",
      name: "name",
      placeholder: "saturn-mcp",
      required: true,
    }),
    renderTextField({
      label: "Expires at (optional, RFC3339)",
      name: "expires_at",
      placeholder: "2027-07-19T10:00:00Z",
    }),
  );

  const scopeField = el("fieldset", "api-key-scopes");
  scopeField.append(el("legend", "control-label", "Scopes"));
  [
    ["data:read", "Read instance data"],
    ["data:write", "Create and modify instance data"],
  ].forEach(([value, label]) => {
    const option = el("label", "api-key-scope-option");
    const checkbox = el("input");
    checkbox.type = "checkbox";
    checkbox.name = "scopes";
    checkbox.value = value;
    checkbox.checked = value === "data:read";
    option.append(checkbox, document.createTextNode(label));
    scopeField.append(option);
  });
  fields.append(scopeField);

  const actions = el("footer", "settings-form__actions");
  const save = renderButton("CREATE", { type: "submit", variant: "primary", label: "Create API key" });
  actions.append(save);
  form.append(fields, actions);
  createRegion.append(createHeader, form, output);
  listRegion.append(listHeader, listOutput);

  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const name = form.elements.name.value.trim();
    const expiresAt = form.elements.expires_at.value.trim();
    const scopes = Array.from(form.querySelectorAll('input[name="scopes"]:checked')).map((input) => input.value);
    if (!name || scopes.length === 0) {
      output.replaceChildren(renderNotice({
        title: "Invalid API Key",
        message: "A name and at least one scope are required.",
      }));
      return;
    }

    setFormBusy(form, true);
    output.replaceChildren(renderNotice({
      title: "Creating API Key",
      message: "Generating a new secret on the server.",
      tone: "info",
    }));
    try {
      const result = await postJSON("/api/auth/api-keys", {
        name,
        scopes,
        expires_at: expiresAt || null,
      });
      form.reset();
      form.querySelector('input[name="scopes"][value="data:read"]').checked = true;
      output.replaceChildren(renderNotice({
        title: "API Key Created",
        message: "Copy the secret from the one-time window before closing it.",
        tone: "info",
      }));
      const secretDialog = renderAPIKeySecretDialog(result, () => {
        output.replaceChildren(renderNotice({
          title: "Secret Removed",
          message: `${result.name} remains active, but its complete key can no longer be shown.`,
          tone: "info",
        }));
      });
      target.append(secretDialog);
      secretDialog.showModal();
      await loadKeys();
    } catch (error) {
      output.replaceChildren(renderNotice({ title: "Unable to Create API Key", message: error.message }));
    } finally {
      setFormBusy(form, false);
    }
  });

  async function revokeKey(key) {
    if (!window.confirm(`Revoke ${key.name} (${key.refcode})?`)) {
      return;
    }
    try {
      await postJSON(`/api/auth/api-keys/${encodeURIComponent(key.refcode)}/revoke`, {});
      await loadKeys();
    } catch (error) {
      listOutput.replaceChildren(renderNotice({ title: "Unable to Revoke API Key", message: error.message }));
    }
  }

  async function loadKeys() {
    listCount.textContent = "—";
    listOutput.replaceChildren(renderNotice({ title: "Loading API Keys", message: "Reading key metadata.", tone: "info" }));
    try {
      const result = await getJSON("/api/auth/api-keys");
      const keys = Array.isArray(result.api_keys) ? result.api_keys : [];
      listCount.textContent = `${keys.length} total`;
      listOutput.replaceChildren(keys.length > 0
        ? renderAPIKeyStatusTable(keys, revokeKey)
        : renderNotice({ title: "No API Keys", message: "Create a key for MCP, agents, CLI, or automation.", tone: "info" }));
    } catch (error) {
      listCount.textContent = "Unavailable";
      listOutput.replaceChildren(renderNotice({ title: "API Keys Unavailable", message: error.message }));
    }
  }

  target.append(renderSection({
    title: "API Keys",
    note: "Create, inspect, and revoke programmatic credentials",
    actions: [renderButton("RETURN", { label: "Return to settings selector", onClick: () => navigateSettings() })],
    children: [createRegion, listRegion],
  }));
  loadKeys();
}

function renderChangePasswordPanel() {
  const form = el("form", "settings-form");
  const output = el("div", "settings-feedback");
  output.setAttribute("aria-live", "polite");

  const fields = el("div", "control-stack");
  fields.append(
    renderTextField({
      label: "Current Password",
      name: "current_password",
      type: "password",
      autocomplete: "current-password",
      required: true,
    }),
    renderTextField({
      label: "New Password",
      name: "new_password",
      type: "password",
      autocomplete: "new-password",
      required: true,
    }),
    renderTextField({
      label: "Confirm Password",
      name: "confirm_password",
      type: "password",
      autocomplete: "new-password",
      required: true,
    }),
  );

  const actions = el("footer", "settings-form__actions");
  const save = renderButton("SAVE", { type: "submit", variant: "primary", label: "Change password" });
  actions.append(save);
  form.append(fields, actions, output);

  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const currentPassword = form.elements.current_password.value;
    const newPassword = form.elements.new_password.value;
    const confirmPassword = form.elements.confirm_password.value;
    if (!newPassword.trim() || newPassword !== confirmPassword) {
      output.replaceChildren(renderNotice({
        title: "Invalid Password",
        message: "New password must be non-empty and match the confirmation.",
      }));
      return;
    }

    setFormBusy(form, true);
    output.replaceChildren(renderNotice({
      title: "Changing Password",
      message: "Verifying current password and storing the new hash.",
      tone: "info",
    }));
    try {
      await patchJSON("/api/auth/me/password", {
        current_password: currentPassword,
        new_password: newPassword,
      });
      form.reset();
      output.replaceChildren(renderNotice({
        title: "Password Changed",
        message: "The current account password was updated.",
        tone: "info",
      }));
    } catch (error) {
      output.replaceChildren(renderNotice({ title: "Unable to Change Password", message: error.message }));
    } finally {
      setFormBusy(form, false);
    }
  });

  return renderDetailPanel({
    title: "Change Password",
    note: "Updates the current account password after validating the current password.",
    children: [form],
  });
}

function renderLogoutPanel(context) {
  const output = el("div", "settings-feedback");
  output.setAttribute("aria-live", "polite");
  const actions = el("div", "settings-actions");
  const logout = renderButton("LOGOUT", { variant: "primary", label: "Log out current session" });
  actions.append(logout);

  logout.addEventListener("click", async () => {
    logout.disabled = true;
    output.replaceChildren(renderNotice({
      title: "Logging Out",
      message: "Revoking the current JWT session.",
      tone: "info",
    }));
    try {
      if (context.onLogout) {
        await context.onLogout();
      } else {
        await logoutSession();
        window.location.reload();
      }
    } catch (error) {
      output.replaceChildren(renderNotice({ title: "Unable to Logout", message: error.message }));
      logout.disabled = false;
    }
  });

  return renderDetailPanel({
    title: "Logout",
    note: "Ends the current browser session and returns to login.",
    children: [renderDetailList([
      { label: "Session", value: "current JWT token" },
      { label: "Effect", value: "token id is revoked and local token storage is cleared" },
    ]), actions, output],
  });
}

function renderAccountDetail(action, context) {
  switch (action) {
    case "password":
      return renderChangePasswordPanel();
    case "logout":
      return renderLogoutPanel(context);
    default:
      return renderAccountOverview(context);
  }
}

function renderAccountPage(target, context, action) {
  const nav = el("nav", "settings-account-nav");
  nav.setAttribute("aria-label", "Account settings");
  [
    ["overview", "INFO", "Account detail"],
    ["password", "PASSWORD", "Change password"],
    ["logout", "LOGOUT", "Logout"],
  ].forEach(([value, label, ariaLabel]) => {
    nav.append(renderButton(label, {
      flat: value !== action,
      pressed: value === action,
      label: ariaLabel,
      onClick: () => navigateSettings(value === "overview"
        ? { section: "account" }
        : { section: "account", action: value }),
    }));
  });

  const layout = el("div", "settings-account-layout");
  layout.append(nav, renderAccountDetail(action, context));

  target.append(renderSection({
    title: "Account",
    note: "Manage the single administrator account and browser session",
    actions: [renderButton("RETURN", { label: "Return to settings selector", onClick: () => navigateSettings() })],
    children: [layout],
  }));
}

function renderSettingsHome(target) {
  const choices = el("div", "settings-choice-grid");
  choices.append(
    renderChoice({
      code: "AUD",
      meta: "Audit",
      title: "Audit",
      description: "Search append-only audit records.",
      onOpen: () => navigateSettings({ section: "audit" }),
    }),
    renderChoice({
      code: "KEY",
      meta: "Credentials",
      title: "API Keys",
      description: "Create, inspect, and revoke programmatic credentials.",
      onOpen: () => navigateSettings({ section: "api-keys" }),
    }),
    renderChoice({
      code: "ACC",
      meta: "Account",
      title: "Account",
      description: "Inspect the administrator account, change its password, and logout.",
      onOpen: () => navigateSettings({ section: "account" }),
    }),
  );

  target.append(renderSection({
    title: "Settings / DevOps",
    note: "Select an operations page",
    children: [choices],
  }));
}

export function renderSettingsPage(target, _health, route, context = {}) {
  const module = el("div", "settings-module");
  const parameters = route?.searchParameters ?? new URLSearchParams();
  const section = parameters.get("section") ?? "";
  const action = parameters.get("action") ?? "overview";

  if (section === "audit") {
    renderAuditPage(module);
  } else if (section === "api-keys") {
    renderAPIKeysPage(module);
  } else if (section === "account") {
    renderAccountPage(module, context, action);
  } else {
    renderSettingsHome(module);
  }

  target.append(module);
}
