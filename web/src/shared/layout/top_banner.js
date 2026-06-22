// This file renders the fixed Saturn top banner.
import { renderStatusPill } from "../components/primitives.js";
import { renderGlobalSearch } from "./global_search.js";

export function renderTopBanner(options = {}) {
  const topbar = document.createElement("header");
  topbar.className = "topbar surface fixed-panel";
  topbar.setAttribute("aria-label", "SATURN top bar");

  const rackToggle = document.createElement("button");
  rackToggle.className = "rack-toggle";
  rackToggle.type = "button";
  rackToggle.setAttribute("aria-label", "Open Control Rack");
  rackToggle.setAttribute("aria-expanded", "false");
  rackToggle.innerHTML = "<span></span><span></span><span></span>";

  const brand = document.createElement("div");
  brand.className = "brand";
  brand.setAttribute("aria-label", "SATURN Home");

  const mark = document.createElement("span");
  mark.className = "brand-mark";
  mark.setAttribute("aria-hidden", "true");

  const copy = document.createElement("span");
  const title = document.createElement("h1");
  title.className = "brand-title";
  title.textContent = "SATURN";
  const subtitle = document.createElement("span");
  subtitle.className = "brand-subtitle";
  subtitle.textContent = "Personal Data Console";
  copy.append(title, subtitle);

  brand.append(mark, copy);

  const statusGroup = document.createElement("div");
  statusGroup.className = "status-group";
  statusGroup.setAttribute("aria-label", "System status");
  statusGroup.append(
    renderStatusPill(options.statusLabel ?? "Online", { state: options.statusState ?? "online" }),
    renderStatusPill(options.systemCode ?? "S-0001", { state: "neutral" }),
  );
  topbar.append(rackToggle, brand, renderGlobalSearch({ onSearch: options.onSearch }), statusGroup);
  return {
    element: topbar,
    rackToggle,
  };
}
