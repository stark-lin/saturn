// This file renders the native HTML login entry point.
import { renderButton, renderSurface } from "../../shared/components/primitives.js";

export function renderLoginPage(options = {}) {
  const panel = renderSurface("main", { className: "login", labelledBy: "login-title" });

  const brand = document.createElement("header");
  brand.className = "brand login-brand";

  const mark = document.createElement("span");
  mark.className = "brand-mark";
  mark.setAttribute("aria-hidden", "true");

  const identity = document.createElement("div");
  const title = document.createElement("h1");
  title.id = "login-title";
  title.className = "brand-title";
  title.textContent = "SATURN";
  const subtitle = document.createElement("span");
  subtitle.className = "brand-subtitle";
  subtitle.textContent = "Personal Console";
  identity.append(title, subtitle);
  brand.append(mark, identity);

  const form = document.createElement("form");
  form.className = "login-form";

  const username = labeledInput("Username", "username", "text");
  username.input.autocomplete = "username";
  username.input.placeholder = "admin";

  const password = labeledInput("Password", "password", "password");
  password.input.autocomplete = "current-password";
  password.input.placeholder = "••••••••";

  const error = document.createElement("p");
  error.className = "login-error";
  error.setAttribute("role", "alert");
  error.hidden = true;

  const submit = renderButton("LOGIN", {
    type: "submit",
    variant: "primary",
    className: "login-submit",
  });

  const note = document.createElement("p");
  note.className = "login-note";
  note.textContent = "Local access only";

  form.append(username.label, password.label, error, submit);
  panel.append(brand, form, note);

  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    error.hidden = true;
    submit.disabled = true;
    try {
      await options.onSubmit?.(username.input.value, password.input.value);
    } catch (submitError) {
      error.textContent = submitError.message;
      error.hidden = false;
    } finally {
      submit.disabled = false;
    }
  });

  return panel;
}

function labeledInput(text, name, type) {
  const label = document.createElement("label");
  label.className = "login-field";
  const title = document.createElement("span");
  title.textContent = text;
  const input = document.createElement("input");
  input.name = name;
  input.type = type;
  input.required = true;
  label.append(title, input);
  return { label, input };
}
