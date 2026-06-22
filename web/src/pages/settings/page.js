// This file renders superuser-only operations views including audit logs and account controls.
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

function actorLabel(log) {
  return log.actor_user_id ? `${log.actor_type} ${log.actor_user_id}` : log.actor_type;
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

  choice.append(head, copy, el("span", "settings-choice__command", "OPEN"));
  return choice;
}

function auditLogTable(logs, onSelectLog) {
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
      { key: "detail", label: "Detail" },
    ],
    rows: logs.map((log) => ({
      time: log.created_at,
      actor: actorLabel(log),
      action: log.action,
      target: log.target_ref_code,
      result: renderTag(log.result),
      source: log.source_ip,
      reason: log.reason ?? "",
      detail: renderButton("OPEN", {
        flat: true,
        chip: true,
        label: `Open audit log ${log.id}`,
        onClick: () => onSelectLog(log),
      }),
    })),
  });
}

function renderAuditDetail(log) {
  return renderDetailPanel({
    title: `Audit Log ${log.id}`,
    note: "Append-only event detail from the platform audit table.",
    children: [renderDetailList([
      { label: "ID", value: log.id },
      { label: "Created At", value: log.created_at },
      { label: "Actor", value: actorLabel(log) },
      { label: "Action", value: log.action },
      { label: "Target", value: log.target_ref_code },
      { label: "Result", value: log.result },
      { label: "Reason", value: log.reason || "empty" },
      { label: "Source IP", value: log.source_ip || "empty" },
      { label: "User Agent", value: log.user_agent || "empty" },
    ])],
  });
}

function renderAuditPage(target) {
  const output = document.createElement("div");
  output.setAttribute("aria-live", "polite");
  output.className = "settings-feedback";
  const detailSlot = el("div", "settings-feedback");

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
    detailSlot.replaceChildren();
    const query = new URLSearchParams(new FormData(form));
    query.set("limit", "50");
    try {
      const result = await getJSON(`/api/platform/audit-logs?${query}`);
      const logs = Array.isArray(result.audit_logs) ? result.audit_logs : [];
      output.replaceChildren(logs.length > 0
        ? auditLogTable(logs, (log) => detailSlot.replaceChildren(renderAuditDetail(log)))
        : renderNotice({ title: "No audit logs", message: "No records match the current filters.", tone: "info" }));
      if (logs.length > 0) {
        detailSlot.replaceChildren(renderAuditDetail(logs[0]));
      }
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
    children: [form, output, detailSlot],
  }));
  loadAuditLogs();
}

function renderAccountOverview(context) {
  const user = context.user ?? {};
  return renderDetailPanel({
    title: "Account Detail",
    note: "Current authenticated principal and available account operations.",
    children: [renderDetailList([
      { label: "User ID", value: user.id ?? "unknown" },
      { label: "Username", value: user.username ?? "unknown" },
      { label: "Email", value: user.email || "empty" },
      { label: "Role", value: user.role ?? "unknown" },
      { label: "Create Account", value: "superuser creates ordinary user accounts" },
      { label: "Password", value: "current user password change requires the current password" },
      { label: "Logout", value: "revokes the current JWT session" },
    ])],
  });
}

function renderCreateUserPanel() {
  const form = el("form", "settings-form");
  const output = el("div", "settings-feedback");
  output.setAttribute("aria-live", "polite");

  const fields = el("div", "control-stack");
  fields.append(
    renderTextField({
      label: "Username",
      name: "username",
      placeholder: "alice",
      autocomplete: "username",
      required: true,
    }),
    renderTextField({
      label: "Email",
      name: "email",
      type: "email",
      placeholder: "alice@example.com",
      autocomplete: "email",
    }),
    renderTextField({
      label: "Password",
      name: "password",
      type: "password",
      autocomplete: "new-password",
      required: true,
    }),
  );

  const actions = el("footer", "settings-form__actions");
  const save = renderButton("SAVE", { type: "submit", variant: "primary", label: "Create account" });
  actions.append(save);
  form.append(fields, actions, output);

  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const username = form.elements.username.value.trim();
    const email = form.elements.email.value.trim();
    const password = form.elements.password.value;
    if (!username || !password.trim()) {
      output.replaceChildren(renderNotice({
        title: "Invalid Account",
        message: "Username and password are required.",
      }));
      return;
    }

    setFormBusy(form, true);
    output.replaceChildren(renderNotice({
      title: "Creating Account",
      message: "Writing a new ordinary user account.",
      tone: "info",
    }));
    try {
      const result = await postJSON("/api/auth/users", {
        username,
        email,
        password,
        role: "user",
      });
      form.reset();
      output.replaceChildren(
        renderNotice({
          title: "Account Created",
          message: `${result.user.username} can now sign in with the initial password.`,
          tone: "info",
        }),
        renderDetailList([
          { label: "User ID", value: result.user.id },
          { label: "Username", value: result.user.username },
          { label: "Email", value: result.user.email || "empty" },
          { label: "Role", value: result.user.role },
        ]),
      );
    } catch (error) {
      output.replaceChildren(renderNotice({ title: "Unable to Create Account", message: error.message }));
    } finally {
      setFormBusy(form, false);
    }
  });

  return renderDetailPanel({
    title: "Create Account",
    note: "Creates an ordinary user account through the authenticated superuser API.",
    children: [form],
  });
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
    case "create":
      return renderCreateUserPanel();
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
    ["create", "NEW", "Create account"],
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
    note: "Create accounts, change password, or end the current session",
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
      description: "Search append-only audit records and open individual event detail.",
      onOpen: () => navigateSettings({ section: "audit" }),
    }),
    renderChoice({
      code: "ACC",
      meta: "Account",
      title: "Account",
      description: "Create user accounts, change your password, and logout.",
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
  } else if (section === "account") {
    renderAccountPage(module, context, action);
  } else {
    renderSettingsHome(module);
  }

  target.append(module);
}
