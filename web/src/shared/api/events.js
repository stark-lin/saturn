// This file connects the static Saturn web client to server-sent events.
import { getAccessToken } from "./client.js";

export function connectEvents(onEvent) {
  const controller = new AbortController();
  void readEventStream(onEvent, controller.signal);
  return controller;
}

async function readEventStream(onEvent, signal) {
  const token = getAccessToken();
  if (!token) {
    onEvent({ type: "error", data: "authentication required" });
    return;
  }

  try {
    const response = await fetch("/api/events", {
      headers: {
        Accept: "text/event-stream",
        Authorization: `Bearer ${token}`,
      },
      signal,
    });
    if (!response.ok || !response.body) {
      throw new Error(`Event stream failed with ${response.status}`);
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let pending = "";
    while (!signal.aborted) {
      const result = await reader.read();
      if (result.done) {
        return;
      }
      pending += decoder.decode(result.value, { stream: true });
      const messages = pending.split("\n\n");
      pending = messages.pop() ?? "";
      for (const message of messages) {
        emitEvent(message, onEvent);
      }
    }
  } catch (error) {
    if (!signal.aborted) {
      onEvent({ type: "error", data: error.message });
    }
  }
}

function emitEvent(message, onEvent) {
  if (!message || message.startsWith(":")) {
    return;
  }
  let type = "message";
  const values = [];
  for (const line of message.split("\n")) {
    if (line.startsWith("event:")) {
      type = line.slice("event:".length).trim();
    }
    if (line.startsWith("data:")) {
      values.push(line.slice("data:".length).trimStart());
    }
  }
  if (values.length > 0) {
    onEvent({ type, data: values.join("\n") });
  }
}
