// This file renders the shared global search control for the Saturn shell.
export function renderGlobalSearch(options = {}) {
  const form = document.createElement("form");
  form.className = "global-search";
  form.role = "search";
  form.setAttribute("aria-label", "Global search");

  const label = document.createElement("span");
  label.className = "global-search__label";
  label.setAttribute("aria-hidden", "true");
  label.textContent = "Search";

  const input = document.createElement("input");
  input.type = "search";
  input.name = "q";
  input.autocomplete = "off";
  input.placeholder = "Open Ref ID: NTE-00000001 / FIL-00000001 / ACC-00000001...";
  input.setAttribute("aria-label", "Open owner-owned metadata detail by Ref ID");

  const key = document.createElement("span");
  key.className = "global-search__key";
  key.setAttribute("aria-hidden", "true");
  key.textContent = "CTRL K";

  const submit = document.createElement("button");
  submit.className = "global-search__submit";
  submit.type = "submit";
  submit.setAttribute("aria-label", "Search Ref ID");
  submit.textContent = "SEARCH";

  const submitQuery = () => {
    options.onSearch?.(input.value.trim());
  };

  form.append(label, input, submit, key);
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    submitQuery();
  });
  input.addEventListener("keydown", (event) => {
    if (event.key !== "Enter") {
      return;
    }
    event.preventDefault();
    submitQuery();
  });

  return form;
}
