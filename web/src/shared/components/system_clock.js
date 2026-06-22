// This file renders the shared system clock panel.
function formatUTCOffset(date) {
  const offsetMinutes = -date.getTimezoneOffset();
  const sign = offsetMinutes >= 0 ? "+" : "-";
  const absMinutes = Math.abs(offsetMinutes);
  const hours = String(Math.floor(absMinutes / 60)).padStart(2, "0");
  const minutes = String(absMinutes % 60).padStart(2, "0");
  return `UTC${sign}${hours}:${minutes}`;
}

export function renderSystemClock() {
  const panel = document.createElement("section");
  panel.className = "clock-panel surface";
  panel.setAttribute("aria-labelledby", "system-clock-title");

  const head = document.createElement("div");
  head.className = "clock-head";

  const title = document.createElement("h3");
  title.className = "clock-title";
  title.id = "system-clock-title";
  title.textContent = "System Clock";

  const dot = document.createElement("span");
  dot.className = "clock-dot";
  dot.setAttribute("aria-hidden", "true");
  head.append(title, dot);

  const timeNode = document.createElement("div");
  timeNode.className = "clock-time";
  timeNode.textContent = "--:--";

  const dateNode = document.createElement("div");
  dateNode.className = "clock-meta";
  dateNode.textContent = "---";

  const zoneNode = document.createElement("div");
  zoneNode.className = "clock-meta";
  zoneNode.textContent = "LOCAL";

  function updateClock() {
    const now = new Date();
    timeNode.textContent = now.toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    });
    dateNode.textContent = now.toLocaleDateString("en-US", {
      weekday: "short",
      day: "2-digit",
      month: "short",
    }).toUpperCase();
    zoneNode.textContent = `LOCAL - ${formatUTCOffset(now)}`;
  }

  updateClock();
  const timer = window.setInterval(updateClock, 30000);
  panel.addEventListener("saturn:dispose", () => window.clearInterval(timer), { once: true });
  panel.append(head, timeNode, dateNode, zoneNode);
  return panel;
}
