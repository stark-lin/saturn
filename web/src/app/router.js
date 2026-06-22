// This file defines root-owned view metadata for the Saturn web client.
export const routes = [
  { view: "files", hash: "#files", requestPath: "/", label: "Files", code: "FIL" },
  { view: "notes", hash: "#notes", requestPath: "/api/notes", label: "Notes", code: "NTE" },
  { view: "accounting", hash: "#accounting", requestPath: "/api/accounting/accounts", label: "Accounting", code: "ACC" },
  { view: "calendar", hash: "#calendar", requestPath: "/api/calendar/view", label: "Calendar", code: "CAL" },
  { view: "llm", hash: "#llm", requestPath: "/api/llm/sessions", label: "LLM", code: "LLM" },
  { view: "search", hash: "#search", requestPath: "/api/platform/search", label: "Search", code: "SEARCH" },
  { view: "settings", hash: "#settings", requestPath: "/api/platform/audit-logs", label: "Settings / DevOps", code: "OPS" },
];

export function findRoute(view, availableRoutes = routes) {
  return availableRoutes.find((route) => route.view === view) ?? availableRoutes[0];
}

export function findRouteByHash(hash, availableRoutes = routes) {
  if (!hash) {
    return withSearchParameters(availableRoutes[0], "");
  }

  const normalizedHash = hash.startsWith("#") ? hash : `#${hash}`;
  const [routeHash, parameters = ""] = normalizedHash.split("?", 2);
  return withSearchParameters(
    availableRoutes.find((route) => route.hash === routeHash) ?? availableRoutes[0],
    parameters,
  );
}

export function searchRouteHash(refCode) {
  const normalizedCode = String(refCode ?? "").trim();
  return normalizedCode ? `#search?ref_code=${encodeURIComponent(normalizedCode)}` : "#search";
}

function withSearchParameters(route, parameters) {
  return { ...route, searchParameters: new URLSearchParams(parameters) };
}
