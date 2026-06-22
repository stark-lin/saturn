// This file composes the shared Saturn application shell.
import { renderControlRack } from "./control_rack.js";
import { renderTopBanner } from "./top_banner.js";

export function renderAppShell(options = {}) {
  const page = document.createElement("div");
  page.className = "page";

  const topBanner = renderTopBanner({
    statusLabel: options.statusLabel,
    statusState: options.statusState,
    systemCode: options.systemCode,
    onSearch: options.onSearch,
  });

  const layout = document.createElement("div");
  layout.className = "app-layout";

  const controlRack = renderControlRack({
    routes: options.routes ?? [],
    activeView: options.activeView,
    onNavigate(route) {
      closeControlRack();
      options.onNavigate?.(route);
    },
  });

  controlRack.element.id = "control-rack";
  topBanner.rackToggle.setAttribute("aria-controls", controlRack.element.id);

  const main = document.createElement("main");
  main.className = "main";
  main.tabIndex = -1;

  layout.append(controlRack.element, main);
  page.append(topBanner.element, layout);

  function setControlRackExpanded(isExpanded) {
    page.classList.toggle("page--rack-open", isExpanded);
    topBanner.rackToggle.setAttribute("aria-expanded", String(isExpanded));
    topBanner.rackToggle.setAttribute(
      "aria-label",
      isExpanded ? "Close Control Rack" : "Open Control Rack",
    );
  }

  function closeControlRack() {
    setControlRackExpanded(false);
  }

  function toggleControlRack() {
    setControlRackExpanded(!page.classList.contains("page--rack-open"));
  }

  topBanner.rackToggle.addEventListener("click", toggleControlRack);

  const closeControlRackOnEscape = (event) => {
    if (event.key === "Escape") {
      closeControlRack();
    }
  };
  document.addEventListener("keydown", closeControlRackOnEscape);

  return {
    element: page,
    main,
    setActiveView(nextView) {
      controlRack.setActiveView(nextView);
    },
    dispose() {
      document.removeEventListener("keydown", closeControlRackOnEscape);
      controlRack.dispose();
    },
  };
}
