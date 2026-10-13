// Chat webview UI. Renders ACP session updates streamed from the
// extension host (src/chat.ts) and sends prompts back. The composer
// supports `/` commands, `@` file mentions, prompt history, and shows
// mode, model, reasoning effort, tool approval (Ask/Yolo) and context
// usage below the input. Each turn's tool calls
// and thinking are grouped under a collapsible "Worked for …" row.

import "./chat.css"
import {
  filterCommands,
  History,
  LOCAL_COMMANDS,
  mentionQuery,
  mergeCommands,
  parseSlash,
  slashQuery,
  usagePercent,
  usageText,
  workLabel,
  type SlashCommand,
  type Usage,
} from "./composer"
import { diffStat, escapeHtml, icon, renderMarkdown, ring, simpleDiff, statusIcon, type IconName } from "./render"

declare function acquireVsCodeApi(): {
  postMessage(msg: unknown): void
  getState(): { history?: string[] } | undefined
  setState(state: { history: string[] }): void
}

const vscode = acquireVsCodeApi()
const $ = <T extends HTMLElement>(id: string) => document.getElementById(id) as T

const messages = $<HTMLElement>("messages")
const form = $<HTMLFormElement>("composer")
const input = $<HTMLTextAreaElement>("input")
const send = $<HTMLButtonElement>("send")
const contextBox = $<HTMLButtonElement>("context")
const contextLabel = $<HTMLElement>("context-label")
const menu = $<HTMLElement>("menu")
const modeChip = $<HTMLButtonElement>("mode")
const modelChip = $<HTMLButtonElement>("model")
const effortChip = $<HTMLButtonElement>("effort")
const approvalChip = $<HTMLButtonElement>("approval")
const cwdEl = $<HTMLElement>("cwd")
const usageEl = $<HTMLElement>("usage")

function fillIcons(root: ParentNode) {
  root.querySelectorAll<HTMLElement>("[data-icon]").forEach((el) => (el.innerHTML = icon(el.dataset.icon as IconName)))
}
fillIcons(document)

let includeContext = true
let busy = false
let busySince = 0
let timer: ReturnType<typeof setInterval> | undefined
// The block currently receiving streamed chunks, so consecutive chunks
// of the same kind append and anything else starts a new block.
let current: { kind: string; el: HTMLElement; text: string } | undefined
let plan: HTMLElement | undefined
// Steps (tools, thinking, plan, interim replies) of the running turn.
let turn: { el: HTMLDetailsElement; steps: HTMLElement; label: HTMLElement } | undefined
const tools = new Map<string, { el: HTMLElement; data: any }>()

let commands: SlashCommand[] = mergeCommands(undefined)
let usage: Usage = {}
const history = new History(vscode.getState()?.history ?? [])

// Open completion menu: slash commands, or files for the @-mention at `start`.
type MenuItem = { label: string; detail?: string; value: string; command?: SlashCommand }
let items: MenuItem[] = []
let active = 0
let mention: { start: number; query: string } | undefined
let fileSearch = 0

function append(el: HTMLElement, parent: HTMLElement = messages) {
  $<HTMLElement>("welcome")?.remove()
  const stick = messages.scrollHeight - messages.scrollTop - messages.clientHeight < 40
  parent.append(el)
  if (stick) messages.scrollTop = messages.scrollHeight
  return el
}

/** Appends to the running turn's steps, or to the transcript outside a turn. */
function appendStep(el: HTMLElement) {
  return append(el, turn?.steps)
}

function startTurn() {
  const el = document.createElement("details")
  el.className = "turn running"
  el.open = true
  el.innerHTML = `<summary><span class=label></span>${icon("chevronRight")}</summary><div class=steps></div>`
  turn = { el, steps: el.querySelector<HTMLElement>(".steps")!, label: el.querySelector<HTMLElement>(".label")! }
  turn.label.textContent = workLabel(0, false)
  append(el)
}

/** Collapses the turn and lifts its trailing reply out as the visible answer. */
function finishTurn() {
  if (!turn) return
  const { el, steps, label } = turn
  const replies: Element[] = []
  for (let last = steps.lastElementChild; last?.matches(".msg.agent"); last = steps.lastElementChild) {
    last.remove()
    replies.unshift(last)
  }
  el.after(...replies)
  label.textContent = workLabel(Date.now() - busySince, true)
  el.classList.remove("running")
  el.classList.toggle("empty", !steps.childElementCount)
  el.open = false
  turn = undefined
  current = undefined
}

function div(cls: string, html = "") {
  const el = document.createElement("div")
  el.className = cls
  el.innerHTML = html
  return el
}

/** Adds Copy / Insert buttons to rendered code blocks. */
function decorateCode(root: HTMLElement) {
  root.querySelectorAll<HTMLElement>("pre:not(.diff)").forEach((pre) => {
    if (pre.parentElement?.classList.contains("code")) return
    const wrap = div("code", `<div class=code-actions><button type=button data-act=copy title="Copy">Copy</button><button type=button data-act=insert title="Insert at cursor">Insert</button></div>`)
    pre.replaceWith(wrap)
    wrap.append(pre)
  })
}

function chunk(kind: "agent" | "thought" | "user", text: string) {
  if (current?.kind !== kind) {
    const el =
      kind === "thought"
        ? div("thought", "<details><summary>Thinking</summary><div class=body></div></details>")
        : div(kind === "user" ? "msg user" : "msg agent")
    current = { kind, el, text: "" }
    if (kind === "user") append(el)
    else appendStep(el)
  }
  current.text += text
  const body = kind === "thought" ? current.el.querySelector<HTMLElement>(".body")! : current.el
  if (kind === "user") {
    body.innerHTML = escapeHtml(current.text).replace(/\n/g, "<br>")
    return
  }
  body.innerHTML = renderMarkdown(current.text)
  if (kind === "agent") decorateCode(body)
  messages.scrollTop = messages.scrollHeight
}

function renderTool(id: string, patch: any) {
  let tool = tools.get(id)
  if (!tool) {
    tool = { el: div("tool"), data: {} }
    tools.set(id, tool)
    appendStep(tool.el)
    current = undefined
  }
  const d = Object.assign(tool.data, Object.fromEntries(Object.entries(patch).filter(([, v]) => v != null)))
  const locations = (d.locations ?? [])
    .map(
      (l: any) =>
        `<a class=loc data-path="${escapeHtml(l.path)}" data-line="${l.line ?? ""}">${escapeHtml(l.path)}${l.line ? `:${l.line}` : ""}</a>`,
    )
    .join(" ")
  const content = (d.content ?? [])
    .map((c: any) => {
      if (c.type === "diff") {
        const diff = simpleDiff(c.oldText, c.newText)
        const { add, del } = diffStat(diff)
        const lines = diff.map((l) => `<span class="${l[0] === "-" ? "del" : "add"}">${escapeHtml(l)}</span>`).join("\n")
        const name = String(c.path).split(/[\\/]/).pop()
        return (
          `<details class=diffcard><summary><a class="loc file" data-path="${escapeHtml(c.path)}" title="${escapeHtml(c.path)}">${escapeHtml(name ?? "")}</a>` +
          `<span class=add>+${add}</span><span class=del>-${del}</span><span class=spacer></span>${icon("chevronDown")}</summary>` +
          `<pre class=diff>${lines}</pre></details>`
        )
      }
      if (c.type === "content" && c.content?.type === "text") return `<pre>${escapeHtml(c.content.text)}</pre>`
      return ""
    })
  const diffs = content.filter((html: string) => html.startsWith("<details class=diffcard")).join("")
  const output = content.filter((html: string) => !html.startsWith("<details class=diffcard")).join("")
  // Re-rendering replaces the markup, so keep the expanded state.
  const open = tool.el.querySelector(".step")?.hasAttribute("open") ? " open" : ""
  const openDiffs = Array.from(tool.el.querySelectorAll<HTMLDetailsElement>(".diffcard"), (x) => x.open)
  const body = `${locations ? `<div class=locs>${locations}</div>` : ""}${output}`
  tool.el.dataset.status = d.status ?? "pending"
  tool.el.innerHTML =
    `<details class=step${open}${body ? "" : " data-empty"}><summary><span class=status>${statusIcon(d.status)}</span>` +
    `<span class=title>${escapeHtml(d.title ?? "Tool")}</span>` +
    `${d.kind ? `<span class=kind>${escapeHtml(d.kind)}</span>` : ""}${body ? icon("chevronRight") : ""}</summary>${body}</details>${diffs}`
  tool.el.querySelectorAll<HTMLDetailsElement>(".diffcard").forEach((x, i) => (x.open = openDiffs[i] ?? false))
}

function renderPlan(entries: any[]) {
  if (!plan) {
    plan = appendStep(div("plan"))
    current = undefined
  }
  plan.innerHTML =
    "<div class=title>Plan</div><ul>" +
    entries.map((e) => `<li class="${escapeHtml(e.status)}">${statusIcon(e.status === "pending" ? undefined : e.status)} ${escapeHtml(e.content)}</li>`).join("") +
    "</ul>"
}

function onUpdate(u: any) {
  switch (u.sessionUpdate) {
    case "agent_message_chunk":
      if (u.content?.type === "text") chunk("agent", u.content.text)
      return
    case "agent_thought_chunk":
      if (u.content?.type === "text") chunk("thought", u.content.text)
      return
    case "user_message_chunk":
      // session/load replays the user's side of past turns.
      if (u.content?.type === "text") chunk("user", u.content.text)
      return
    case "tool_call":
    case "tool_call_update":
      renderTool(u.toolCallId, u)
      return
    case "plan":
      renderPlan(u.entries ?? [])
      return
    case "available_commands_update":
      commands = mergeCommands(u.availableCommands)
      return
    case "usage_update":
      setUsage({ used: u.used, size: u.size, cost: u.cost })
      return
  }
}

function permission(id: number, toolCall: any, options: any[]) {
  const el = append(div("permission", `<div>Allow <b>${escapeHtml(toolCall?.title ?? "tool call")}</b>?</div><div class=buttons></div>`))
  const buttons = el.querySelector(".buttons")!
  for (const o of options) {
    const b = document.createElement("button")
    b.type = "button"
    b.textContent = o.name
    if (String(o.kind).startsWith("reject")) b.className = "secondary"
    b.onclick = () => {
      vscode.postMessage({ type: "permission", id, optionId: o.optionId })
      el.remove()
    }
    buttons.append(b)
  }
  current = undefined
}

function notice(kind: "info" | "error", html: string) {
  append(div(`notice ${kind}`, html))
  current = undefined
}

function setUsage(u: Usage) {
  usage = { ...usage, ...u }
  const pct = usagePercent(usage)
  const text = usageText(usage)
  usageEl.title = text ? `Context window: ${text}` : ""
  usageEl.innerHTML = pct === undefined ? escapeHtml(text) : `${ring(pct)}<span>${pct}%</span>`
}

function renderStatus() {
  if (busy && turn) turn.label.textContent = workLabel(Date.now() - busySince, false)
}

function renderSend() {
  send.innerHTML = icon(busy ? "stop" : "arrowUp")
  send.title = busy ? "Stop (Esc)" : "Send (Enter)"
  send.setAttribute("aria-label", busy ? "Stop" : "Send")
  send.classList.toggle("stop", busy)
  send.classList.toggle("empty", !busy && !input.value.trim())
}

function setBusy(b: boolean) {
  busy = b
  renderSend()
  clearInterval(timer)
  timer = undefined
  if (b) {
    busySince = Date.now()
    timer = setInterval(renderStatus, 1000)
  } else {
    finishTurn()
    document.querySelectorAll(".permission").forEach((el) => el.remove())
  }
  renderStatus()
}

function setIncludeContext(on: boolean) {
  includeContext = on
  contextBox.setAttribute("aria-pressed", String(on))
}

function reset() {
  messages.innerHTML = `<div id=welcome><h2>Codg</h2><p>New chat. Ask Codg anything about this workspace.</p><p class=tips>Type <kbd>/</kbd> for commands, <kbd>@</kbd> to mention files.</p></div>`
  current = undefined
  plan = undefined
  turn = undefined
  tools.clear()
  usage = {}
  setUsage({})
  setBusy(false)
}

function showHelp() {
  const rows = commands
    .map((c) => `<tr><td><code>/${escapeHtml(c.name)}</code></td><td>${escapeHtml(c.description)}</td></tr>`)
    .join("")
  notice(
    "info",
    `<div class=title>Commands</div><table>${rows}</table>` +
      `<div class=title>Shortcuts</div><table>` +
      `<tr><td><kbd>Enter</kbd> / <kbd>Shift+Enter</kbd></td><td>Send / newline</td></tr>` +
      `<tr><td><kbd>↑</kbd> <kbd>↓</kbd></td><td>Prompt history</td></tr>` +
      `<tr><td><kbd>Shift+Tab</kbd></td><td>Cycle agent</td></tr>` +
      `<tr><td><kbd>Esc</kbd></td><td>Close menu / stop</td></tr>` +
      `<tr><td><kbd>@</kbd></td><td>Mention a workspace file</td></tr></table>`,
  )
}

/** Runs an extension-side command; returns false for agent commands. */
function runLocal(name: string): boolean {
  switch (name) {
    case "new":
    case "clear":
      vscode.postMessage({ type: "newChat" })
      return true
    case "model":
    case "mode":
    case "effort":
    case "approval":
      vscode.postMessage({ type: "pick", kind: name })
      return true
    case "terminal":
      vscode.postMessage({ type: "terminal" })
      return true
    case "history":
      vscode.postMessage({ type: "history" })
      return true
    case "help":
      showHelp()
      return true
  }
  return false
}

function setInput(text: string, caret = text.length) {
  input.value = text
  input.setSelectionRange(caret, caret)
  autosize()
}

function autosize() {
  input.style.height = "auto"
  input.style.height = `${Math.min(input.scrollHeight, window.innerHeight * 0.4)}px`
}

function submit() {
  closeMenu()
  if (busy) {
    vscode.postMessage({ type: "cancel" })
    return
  }
  const text = input.value.trim()
  if (!text) return
  history.push(text)
  vscode.setState({ history: history.items })
  setInput("")
  const slash = parseSlash(text)
  if (slash && LOCAL_COMMANDS.some((c) => c.name === slash.name) && runLocal(slash.name)) return
  const withContext = !contextBox.hidden && includeContext
  const label = withContext ? contextLabel.textContent : ""
  append(div("msg user", `${escapeHtml(text).replace(/\n/g, "<br>")}${label ? `<div class=ctx>${escapeHtml(label)}</div>` : ""}`))
  messages.scrollTop = messages.scrollHeight
  current = undefined
  plan = undefined
  startTurn()
  vscode.postMessage({ type: "prompt", text, includeContext: withContext })
}

// --- completion menu ---

function openMenu(next: MenuItem[], empty?: string) {
  items = next
  active = 0
  if (!items.length && !empty) {
    closeMenu()
    return
  }
  menu.innerHTML = items.length
    ? items
        .map(
          (it, i) =>
            `<div class="item${i === active ? " active" : ""}" role=option data-i=${i}><span class=label>${escapeHtml(it.label)}</span>` +
            `${it.detail ? `<span class=detail>${escapeHtml(it.detail)}</span>` : ""}</div>`,
        )
        .join("")
    : `<div class=empty>${escapeHtml(empty!)}</div>`
  menu.hidden = false
}

function closeMenu() {
  menu.hidden = true
  items = []
  mention = undefined
}

function moveActive(delta: number) {
  if (!items.length) return
  active = (active + delta + items.length) % items.length
  menu.querySelectorAll(".item").forEach((el, i) => el.classList.toggle("active", i === active))
  menu.querySelector(".item.active")?.scrollIntoView({ block: "nearest" })
}

/** Accepts the highlighted item; `run` sends argument-less commands right away. */
function accept(run: boolean) {
  const it = items[active]
  if (!it) return
  if (it.command) {
    closeMenu()
    if (run && (it.command.local || !it.command.hint)) {
      setInput(`/${it.command.name}`)
      submit()
      return
    }
    setInput(`/${it.command.name} `)
    return
  }
  if (mention) {
    const caret = input.selectionStart
    const insert = `@${it.value} `
    setInput(input.value.slice(0, mention.start) + insert + input.value.slice(caret), mention.start + insert.length)
    closeMenu()
  }
}

/** Re-evaluates the menu from the text before the caret. */
function updateMenu() {
  const caret = input.selectionStart
  const slash = slashQuery(input.value, caret)
  if (slash !== undefined) {
    mention = undefined
    openMenu(
      filterCommands(commands, slash).map((c) => ({
        label: `/${c.name}${c.hint ? ` <${c.hint}>` : ""}`,
        detail: c.description,
        value: c.name,
        command: c,
      })),
      "No matching commands",
    )
    return
  }
  const m = mentionQuery(input.value, caret)
  if (!m) {
    closeMenu()
    return
  }
  mention = m
  vscode.postMessage({ type: "searchFiles", id: ++fileSearch, query: m.query })
}

function insertAtCaret(text: string) {
  const start = input.selectionStart
  const end = input.selectionEnd
  const before = input.value.slice(0, start)
  const pad = before && !/\s$/.test(before) ? " " : ""
  setInput(before + pad + text + input.value.slice(end), start + pad.length + text.length)
  input.focus()
}

// --- events ---

form.addEventListener("submit", (e) => {
  e.preventDefault()
  submit()
})

input.addEventListener("input", () => {
  autosize()
  renderSend()
  updateMenu()
})

input.addEventListener("keydown", (e) => {
  if (e.isComposing) return
  if (!menu.hidden) {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault()
      moveActive(e.key === "ArrowDown" ? 1 : -1)
      return
    }
    if ((e.key === "Enter" && !e.shiftKey) || e.key === "Tab") {
      if (items.length) {
        e.preventDefault()
        accept(e.key === "Enter")
        return
      }
    }
    if (e.key === "Escape") {
      e.preventDefault()
      closeMenu()
      return
    }
  }
  if (e.key === "Enter" && !e.shiftKey) {
    e.preventDefault()
    submit()
  } else if (e.key === "Escape" && busy) {
    vscode.postMessage({ type: "cancel" })
  } else if (e.key === "Tab" && e.shiftKey) {
    e.preventDefault()
    vscode.postMessage({ type: "cycleMode" })
  } else if (e.key === "ArrowUp" && !input.value.slice(0, input.selectionStart).includes("\n")) {
    const prev = history.prev(input.value)
    if (prev !== undefined) {
      e.preventDefault()
      setInput(prev)
    }
  } else if (e.key === "ArrowDown" && !input.value.slice(input.selectionEnd).includes("\n")) {
    const next = history.next()
    if (next !== undefined) {
      e.preventDefault()
      setInput(next)
    }
  }
})

input.addEventListener("blur", () => setTimeout(() => document.activeElement !== input && closeMenu(), 150))

menu.addEventListener("mousedown", (e) => {
  const el = (e.target as HTMLElement).closest<HTMLElement>(".item")
  if (!el) return
  e.preventDefault()
  active = Number(el.dataset.i)
  accept(true)
})

modeChip.addEventListener("click", () => vscode.postMessage({ type: "pick", kind: "mode" }))
modelChip.addEventListener("click", () => vscode.postMessage({ type: "pick", kind: "model" }))
effortChip.addEventListener("click", () => vscode.postMessage({ type: "pick", kind: "effort" }))
approvalChip.addEventListener("click", () => vscode.postMessage({ type: "pick", kind: "approval" }))
$<HTMLButtonElement>("history").addEventListener("click", () => vscode.postMessage({ type: "history" }))
$<HTMLButtonElement>("mention").addEventListener("click", () => {
  insertAtCaret("@")
  updateMenu()
})
contextBox.addEventListener("click", () => setIncludeContext(!includeContext))

messages.addEventListener("click", (e) => {
  const target = e.target as HTMLElement
  const act = target.closest<HTMLElement>("[data-act]")
  if (act) {
    const code = act.closest(".code")?.querySelector("pre")?.textContent ?? ""
    vscode.postMessage({ type: act.dataset.act === "copy" ? "copy" : "insertCode", text: code })
    if (act.dataset.act === "copy") {
      act.textContent = "Copied"
      setTimeout(() => (act.textContent = "Copy"), 1200)
    }
    return
  }
  const a = target.closest<HTMLElement>("a.loc")
  if (!a) return
  e.preventDefault()
  vscode.postMessage({ type: "openFile", path: a.dataset.path, line: Number(a.dataset.line) || undefined })
})

window.addEventListener("message", (e) => {
  const m = e.data
  switch (m.type) {
    case "update":
      onUpdate(m.update)
      return
    case "status":
      setBusy(m.busy)
      return
    case "permission":
      permission(m.id, m.toolCall, m.options ?? [])
      return
    case "context":
      contextBox.hidden = !m.label
      contextLabel.textContent = m.label ?? ""
      return
    case "session":
      for (const [el, text, enabled] of [
        [modeChip, m.mode, m.modes],
        [modelChip, m.model, m.models],
        [effortChip, m.effort, m.efforts],
        [approvalChip, m.approval, true],
        [cwdEl, m.cwd, false],
      ] as [HTMLElement, string | undefined, boolean][]) {
        el.hidden = !text
        el.querySelector(".text")!.textContent = text ?? ""
        if (el instanceof HTMLButtonElement) el.disabled = !enabled
      }
      approvalChip.classList.toggle("yolo", !!m.yolo)
      approvalChip.classList.toggle("full", m.yolo === "full")
      approvalChip.querySelector(".glyph")!.innerHTML = icon(m.yolo ? "bolt" : "shield")
      modeChip.title = m.agent ? `Agent (${m.agent})` : "Agent"
      return
    case "usage":
      setUsage(m.usage)
      return
    case "files":
      if (m.id !== fileSearch || !mention) return
      openMenu(
        (m.items as string[]).map((p) => {
          const i = p.lastIndexOf("/")
          return { label: p.slice(i + 1), detail: i > 0 ? p.slice(0, i) : undefined, value: p }
        }),
        "No matching files",
      )
      return
    case "insert":
      insertAtCaret(m.text)
      return
    case "info":
    case "error":
      notice(m.type, escapeHtml(m.message))
      return
    case "reset":
      reset()
      input.focus()
      return
  }
})

vscode.postMessage({ type: "ready" })
input.focus()
