// This file renders the shared Control Rack module selector.
import { renderSystemClock } from "../components/system_clock.js";

function setButtonCurrent(buttons, activeView) {
  buttons.forEach((button) => {
    if (button.dataset.view === activeView) {
      button.setAttribute("aria-current", "page");
    } else {
      button.removeAttribute("aria-current");
    }
  });
}

export function renderControlRack(options = {}) {
  const aside = document.createElement("aside");
  aside.className = "sidebar surface fixed-panel";
  aside.setAttribute("aria-label", "Sidebar");

  const title = document.createElement("h2");
  title.className = "rack-title";
  title.textContent = "Control Rack";

  const nav = document.createElement("nav");
  nav.className = "nav-list";
  nav.setAttribute("aria-label", "Primary navigation");

  const buttons = options.routes.map((route) => {
    const button = document.createElement("button");
    button.className = "nav-link";
    button.type = "button";
    button.dataset.view = route.view;
    button.dataset.hash = route.hash;
    button.dataset.requestPath = route.requestPath;

    const label = document.createElement("span");
    label.className = "nav-label";
    label.textContent = route.label;

    const code = document.createElement("span");
    code.className = "code";
    code.textContent = route.code;

    button.append(label, code);
    button.addEventListener("click", () => {
      setButtonCurrent(buttons, route.view);
      options.onNavigate?.(route);
    });
    nav.append(button);
    return button;
  });

  const clock = renderSystemClock();
  aside.append(title, nav, clock);
  setButtonCurrent(buttons, options.activeView);

  return {
    element: aside,
    setActiveView(nextView) {
      setButtonCurrent(buttons, nextView);
    },
    dispose() {
      clock.dispatchEvent(new Event("saturn:dispose"));
    },
  };
}
