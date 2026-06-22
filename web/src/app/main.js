// This file boots the authenticated static Saturn web client.
import { getAccessToken, getJSON, login, logout } from "../shared/api/client.js";
import { connectEvents } from "../shared/api/events.js";
import { renderAppShell } from "../shared/layout/app_shell.js";
import { renderAccountingPage } from "../pages/accounting/page.js";
import { renderCalendarPage } from "../pages/calendar/page.js";
import { renderFilesPage } from "../pages/files/page.js";
import { renderLLMPage } from "../pages/llm/page.js";
import { renderNotesPage } from "../pages/notes/page.js";
import { renderSearchPage } from "../pages/search/page.js";
import { renderSettingsPage } from "../pages/settings/page.js";
import { renderLoginPage } from "../pages/login/page.js";
import { findRouteByHash, routes, searchRouteHash } from "./router.js";

const root = document.querySelector("#app");
let disposeAuthenticatedView = () => {};

const routeRenderers = new Map([
  ["search", renderSearchPage],
  ["files", renderFilesPage],
  ["notes", renderNotesPage],
  ["accounting", renderAccountingPage],
  ["calendar", renderCalendarPage],
  ["llm", renderLLMPage],
  ["settings", renderSettingsPage],
]);

async function boot() {
  if (!getAccessToken()) {
    renderLogin();
    return;
  }
  try {
    const session = await getJSON("/api/auth/me");
    await renderAuthenticated(session.user);
  } catch (_error) {
    renderLogin();
  }
}

function renderLogin() {
  disposeAuthenticatedView();
  root.classList.add("login-root");
  root.replaceChildren(renderLoginPage({
    async onSubmit(username, password) {
      const session = await login(username, password);
      await renderAuthenticated(session.user);
    },
  }));
}

async function renderAuthenticated(user) {
  disposeAuthenticatedView();
  const health = await getJSON("/healthz");
  const availableRoutes = user.role === "superuser"
    ? routes
    : routes.filter((route) => route.view !== "settings");
  let activeRoute = findRouteByHash(window.location.hash, availableRoutes);
  let shell;

  async function endSession() {
    await logout();
    renderLogin();
  }

  function renderRoute(route) {
    activeRoute = route;
    shell.setActiveView(activeRoute.view);
    shell.main.replaceChildren();
    routeRenderers.get(activeRoute.view)(shell.main, health, activeRoute, {
      user,
      onLogout: endSession,
    });
    shell.main.focus({ preventScroll: true });
  }

  shell = renderAppShell({
    routes: availableRoutes,
    activeView: activeRoute.view,
    statusLabel: health.status === "ok" ? "Online" : "Degraded",
    statusState: health.status === "ok" ? "online" : "warning",
    systemCode: "S-0001",
    onNavigate(route) {
      if (window.location.hash === route.hash) {
        renderRoute(route);
        return;
      }

      window.location.hash = route.hash;
    },
    onSearch(query) {
      const nextHash = searchRouteHash(query);
      if (window.location.hash === nextHash) {
        renderRoute(findRouteByHash(nextHash, availableRoutes));
        return;
      }
      window.location.hash = nextHash;
    },
  });

  root.classList.remove("login-root");
  root.replaceChildren(shell.element);
  renderRoute(activeRoute);

  const onHashChange = () => {
    const nextRoute = findRouteByHash(window.location.hash, availableRoutes);
    renderRoute(nextRoute);
  };
  window.addEventListener("hashchange", onHashChange);

  const eventStream = connectEvents((event) => {
    document.dispatchEvent(new CustomEvent("saturn:event", { detail: event }));
  });
  disposeAuthenticatedView = () => {
    eventStream.abort();
    window.removeEventListener("hashchange", onHashChange);
    shell.dispose();
    disposeAuthenticatedView = () => {};
  };
}

boot().catch(() => {
  renderLogin();
});
