// This file wraps JSON HTTP requests for the static Saturn web client.
const accessTokenKey = "saturn.access_token";

export function getAccessToken() {
  return window.sessionStorage.getItem(accessTokenKey);
}

export function clearAccessToken() {
  window.sessionStorage.removeItem(accessTokenKey);
}

export async function getJSON(path) {
  return requestJSON(path);
}

export async function postJSON(path, value) {
  return requestJSON(path, {
    method: "POST",
    body: JSON.stringify(value),
  });
}

export async function postFormData(path, value) {
  return requestJSON(path, {
    method: "POST",
    body: value,
  });
}

export async function patchJSON(path, value) {
  return requestJSON(path, {
    method: "PATCH",
    body: JSON.stringify(value),
  });
}

export async function deleteJSON(path) {
  return requestJSON(path, { method: "DELETE" });
}

export async function login(username, password) {
  const result = await requestJSON("/api/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
  window.sessionStorage.setItem(accessTokenKey, result.token);
  return result;
}

export async function logout() {
  try {
    await requestJSON("/api/auth/logout", { method: "POST" });
  } finally {
    clearAccessToken();
  }
}

async function requestJSON(path, options = {}) {
  const headers = new Headers(options.headers);
  headers.set("Accept", "application/json");
  const isFormData = typeof FormData !== "undefined" && options.body instanceof FormData;
  if (options.body && !isFormData) {
    headers.set("Content-Type", "application/json");
  }
  const token = getAccessToken();
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  const response = await fetch(path, { ...options, headers });

  if (!response.ok) {
    if (response.status === 401) {
      clearAccessToken();
    }
    const result = await response.json().catch(() => null);
    const message = result?.error?.message ?? `Request failed with ${response.status}`;
    const error = new Error(message);
    error.body = result ?? {
      error: {
        code: "request_failed",
        message,
      },
    };
    throw error;
  }

  if (response.status === 204) {
    return null;
  }

  return response.json();
}
