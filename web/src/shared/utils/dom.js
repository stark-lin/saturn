// This file provides small DOM helpers for the static web client.
export function el(tagName, className) {
  const node = document.createElement(tagName);
  if (className) {
    node.className = className;
  }
  return node;
}
