// This file parses structured Note Markdown and renders its untrusted body safely.
export function parseNoteMarkdown(markdown) {
  const normalized = String(markdown ?? "").replace(/\r\n?/g, "\n");
  const [titleLine = "", tagsLine = "", ...bodyLines] = normalized.split("\n");
  const tags = tagsLine
    .split(",")
    .map((tag) => tag.trim())
    .filter((tag, index, values) => tag && values.indexOf(tag) === index);

  return {
    title: titleLine.trim(),
    tags,
    body: bodyLines.join("\n"),
  };
}

export function renderMarkdown(target, markdown) {
  const source = String(markdown ?? "");

  if (!globalThis.marked?.parse || !globalThis.DOMPurify?.sanitize) {
    target.textContent = source;
    return;
  }

  // Markdown parser output is untrusted until it has passed through DOMPurify.
  const rendered = globalThis.marked.parse(source);
  target.innerHTML = globalThis.DOMPurify.sanitize(rendered);
}
