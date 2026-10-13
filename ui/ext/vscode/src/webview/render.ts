// Pure HTML rendering helpers for the chat webview. No DOM access so
// they can be unit-tested with plain `node --test`.

export function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`)
}

function inline(s: string): string {
  // Split out code spans first so their content is not formatted.
  return s
    .split(/(`[^`\n]+`)/)
    .map((part) => {
      if (part.length > 1 && part.startsWith("`") && part.endsWith("`")) {
        return `<code>${escapeHtml(part.slice(1, -1))}</code>`
      }
      return escapeHtml(part)
        .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
        .replace(/(^|[^*\w])\*([^*\s][^*]*)\*(?!\w)/g, "$1<em>$2</em>")
        .replace(
          /\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g,
          (_m, text: string, url: string) => `<a href="${url}">${text}</a>`,
        )
    })
    .join("")
}

/**
 * Renders the Markdown subset agents emit (fenced code, headings,
 * lists, quotes, inline code/bold/italic/links) into safe HTML. All
 * text is escaped; only http(s) links are produced. Tolerates partial
 * input while a response is still streaming (unclosed fences).
 */
export function renderMarkdown(md: string): string {
  const out: string[] = []
  const lines = md.replace(/\r\n/g, "\n").split("\n")
  let list: "ul" | "ol" | undefined
  let para: string[] = []

  const flushPara = () => {
    if (para.length) out.push(`<p>${para.map(inline).join("<br>")}</p>`)
    para = []
  }
  const closeList = () => {
    if (list) out.push(`</${list}>`)
    list = undefined
  }

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    const fence = /^\s*```(\S*)/.exec(line)
    if (fence) {
      flushPara()
      closeList()
      const code: string[] = []
      for (i++; i < lines.length && !/^\s*```\s*$/.test(lines[i]); i++) code.push(lines[i])
      const lang = fence[1] ? ` data-lang="${escapeHtml(fence[1])}"` : ""
      out.push(`<pre${lang}><code>${escapeHtml(code.join("\n"))}</code></pre>`)
      continue
    }
    const heading = /^(#{1,6})\s+(.*)$/.exec(line)
    const bullet = /^\s*[-*+]\s+(.*)$/.exec(line)
    const ordered = /^\s*\d+[.)]\s+(.*)$/.exec(line)
    const quote = /^>\s?(.*)$/.exec(line)
    if (heading) {
      flushPara()
      closeList()
      const n = heading[1].length
      out.push(`<h${n}>${inline(heading[2])}</h${n}>`)
    } else if (bullet || ordered) {
      flushPara()
      const kind = bullet ? "ul" : "ol"
      if (list !== kind) {
        closeList()
        out.push(`<${kind}>`)
        list = kind
      }
      out.push(`<li>${inline((bullet ?? ordered)![1])}</li>`)
    } else if (quote) {
      flushPara()
      closeList()
      out.push(`<blockquote>${inline(quote[1])}</blockquote>`)
    } else if (!line.trim()) {
      flushPara()
      closeList()
    } else {
      closeList()
      para.push(line)
    }
  }
  flushPara()
  closeList()
  return out.join("")
}

/**
 * Line diff for ACP `diff` tool content: drops the common prefix and
 * suffix and shows the changed middle as -/+ lines.
 */
export function simpleDiff(oldText: string | null | undefined, newText: string): string[] {
  const a = oldText ? oldText.split("\n") : []
  const b = newText.split("\n")
  let start = 0
  while (start < a.length && start < b.length && a[start] === b[start]) start++
  let endA = a.length
  let endB = b.length
  while (endA > start && endB > start && a[endA - 1] === b[endB - 1]) {
    endA--
    endB--
  }
  return [...a.slice(start, endA).map((l) => `-${l}`), ...b.slice(start, endB).map((l) => `+${l}`)]
}

/** Added/removed line counts of a simpleDiff result. */
export function diffStat(lines: string[]): { add: number; del: number } {
  const del = lines.filter((l) => l[0] === "-").length
  return { add: lines.length - del, del }
}

const ICONS = {
  plus: `<path d="M8 3v10M3 8h10"/>`,
  arrowUp: `<path d="M8 13V3M3.5 7.5 8 3l4.5 4.5"/>`,
  stop: `<rect x="4" y="4" width="8" height="8" rx="1.5" fill="currentColor" stroke="none"/>`,
  cursor: `<path d="M5 5l7.5 2.8-3.2 1.3-1.3 3.2z"/><path d="M5.5 1.5v1.5M1.5 5.5H3M2.6 2.6l1 1"/>`,
  laptop: `<rect x="3" y="3.5" width="10" height="7" rx="1"/><path d="M1.5 12.5h13"/>`,
  agent: `<rect x="4" y="2.5" width="8" height="6" rx="2"/><path d="M6.5 5.5v.01M9.5 5.5v.01M3 14c.5-2.5 2.5-3.5 5-3.5s4.5 1 5 3.5"/>`,
  model: `<path d="M8 2l1.5 4.5L14 8l-4.5 1.5L8 14l-1.5-4.5L2 8l4.5-1.5z"/>`,
  shield: `<path d="M8 1.8 13 3.8v3.7c0 3-2.1 5.4-5 6.7-2.9-1.3-5-3.7-5-6.7V3.8z"/>`,
  bolt: `<path d="M9 1.5 3.5 9H8l-1 5.5L12.5 7H8z"/>`,
  history: `<path d="M2.5 8a5.5 5.5 0 1 0 1.6-3.9"/><path d="M2.5 2.5v2.5H5"/><path d="M8 5v3l2 1.5"/>`,
  chevronDown: `<path d="M4.5 6.5 8 10l3.5-3.5"/>`,
  chevronRight: `<path d="M6.5 4.5 10 8l-3.5 3.5"/>`,
}

export type IconName = keyof typeof ICONS

/** Inline 16×16 stroke SVG icon (no font or network needed under the webview CSP). */
export function icon(name: IconName): string {
  return (
    `<svg class="icon-svg" viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" ` +
    `stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICONS[name]}</svg>`
  )
}

/** Circular progress ring for context usage; `pct` is clamped to 0–100. */
export function ring(pct: number): string {
  const c = 2 * Math.PI * 6
  const len = ((Math.min(Math.max(pct, 0), 100) / 100) * c).toFixed(2)
  return (
    `<svg class="ring" viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">` +
    `<circle class="track" cx="8" cy="8" r="6"/>` +
    `<circle class="fill" cx="8" cy="8" r="6" stroke-dasharray="${len} ${c.toFixed(2)}" transform="rotate(-90 8 8)"/></svg>`
  )
}

/** Glyph shown for an ACP tool call status. */
export function statusIcon(status?: string): string {
  switch (status) {
    case "completed":
      return "✓"
    case "failed":
      return "✗"
    case "in_progress":
      return "◐"
    default:
      return "○"
  }
}
