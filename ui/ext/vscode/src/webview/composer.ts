// Pure composer helpers for the chat webview: slash commands, @-mention
// parsing, prompt history and footer formatting. No DOM access so they
// can be unit-tested with plain `node --test`.

export interface SlashCommand {
  name: string
  description: string
  /** Argument hint; commands with a hint wait for input before sending. */
  hint?: string
  /** Handled by the extension instead of being sent to the agent. */
  local?: boolean
}

export const LOCAL_COMMANDS: SlashCommand[] = [
  { name: "new", description: "Start a new chat", local: true },
  { name: "clear", description: "Clear the conversation and start over", local: true },
  { name: "model", description: "Switch the model", local: true },
  { name: "mode", description: "Switch the agent mode", local: true },
  { name: "effort", description: "Switch the reasoning effort", local: true },
  { name: "approval", description: "Ask, Yolo (work) or Yolo (full) tool approval", local: true },
  { name: "history", description: "Open a past chat", local: true },
  { name: "terminal", description: "Open Codg in a terminal", local: true },
  { name: "help", description: "Show commands and shortcuts", local: true },
]

/** Merges local commands with ACP `availableCommands`; local names win. */
export function mergeCommands(agent: unknown): SlashCommand[] {
  const local = new Set(LOCAL_COMMANDS.map((c) => c.name))
  const remote = (Array.isArray(agent) ? agent : [])
    .filter((c) => typeof c?.name === "string" && !local.has(c.name))
    .map((c): SlashCommand => ({ name: c.name, description: c.description ?? "", hint: c.input?.hint || undefined }))
  return [...LOCAL_COMMANDS, ...remote]
}

/** Command query while the input is just `/word` up to the caret. */
export function slashQuery(text: string, caret: number): string | undefined {
  return /^\/(\S*)$/.exec(text.slice(0, caret))?.[1]
}

/** The `@query` being typed before the caret, with the index of `@`. */
export function mentionQuery(text: string, caret: number): { start: number; query: string } | undefined {
  const m = /(^|\s)@([^\s@]*)$/.exec(text.slice(0, caret))
  return m ? { start: caret - m[2].length - 1, query: m[2] } : undefined
}

/** Prefix matches first, then names or descriptions containing the query. */
export function filterCommands(commands: SlashCommand[], query: string): SlashCommand[] {
  const q = query.toLowerCase()
  const prefix = commands.filter((c) => c.name.toLowerCase().startsWith(q))
  const rest = commands.filter(
    (c) => !prefix.includes(c) && `${c.name} ${c.description}`.toLowerCase().includes(q),
  )
  return [...prefix, ...rest]
}

/** Splits a submitted `/name args` prompt. */
export function parseSlash(text: string): { name: string; args: string } | undefined {
  const m = /^\/(\S+)(?:\s+([\s\S]*))?$/.exec(text.trim())
  return m ? { name: m[1], args: m[2] ?? "" } : undefined
}

/** Shell-style prompt history navigated with ↑/↓; keeps the unsent draft. */
export class History {
  private index: number
  private draft = ""

  constructor(
    readonly items: string[] = [],
    private readonly limit = 50,
  ) {
    this.index = items.length
  }

  push(text: string) {
    if (text && this.items[this.items.length - 1] !== text) this.items.push(text)
    if (this.items.length > this.limit) this.items.splice(0, this.items.length - this.limit)
    this.index = this.items.length
    this.draft = ""
  }

  prev(current: string): string | undefined {
    if (this.index === 0) return undefined
    if (this.index === this.items.length) this.draft = current
    return this.items[--this.index]
  }

  next(): string | undefined {
    if (this.index >= this.items.length) return undefined
    this.index++
    return this.index === this.items.length ? this.draft : this.items[this.index]
  }
}

/** 950 → "950", 12345 → "12.3k", 2000000 → "2M". */
export function formatTokens(n: number): string {
  if (n < 1000) return String(n)
  const [v, unit] = n < 1e6 ? [n / 1e3, "k"] : [n / 1e6, "M"]
  return `${v.toFixed(1).replace(/\.0$/, "")}${unit}`
}

export interface Usage {
  /** Tokens in the context window (ACP usage_update). */
  used?: number
  size?: number
  cost?: { amount: number; currency: string }
  /** Tokens of the last turn (PromptResponse.usage). */
  totalTokens?: number
}

/** Footer usage text, e.g. "12.3k / 200k (6%) · $0.12". */
export function usageText(u: Usage): string {
  const parts: string[] = []
  if (u.used != null && u.size) {
    parts.push(`${formatTokens(u.used)} / ${formatTokens(u.size)} (${Math.round((u.used / u.size) * 100)}%)`)
  } else if (u.totalTokens) {
    parts.push(`${formatTokens(u.totalTokens)} tokens`)
  }
  if (u.cost) {
    const amount = u.cost.amount.toFixed(2)
    parts.push(u.cost.currency === "USD" ? `$${amount}` : `${amount} ${u.cost.currency}`)
  }
  return parts.join(" · ")
}

/** Context window usage in percent, when the window size is known. */
export function usagePercent(u: Usage): number | undefined {
  if (u.used == null || !u.size) return undefined
  return Math.min(100, Math.round((u.used / u.size) * 100))
}

/** Turn summary: "Working… 12s" while running, "Worked for 6m 53s" when done. */
export function workLabel(ms: number, done: boolean): string {
  return done ? `Worked for ${formatElapsed(ms)}` : `Working… ${formatElapsed(ms)}`
}

/** 5000 → "5s", 65000 → "1m 05s". */
export function formatElapsed(ms: number): string {
  const s = Math.floor(ms / 1000)
  return s < 60 ? `${s}s` : `${Math.floor(s / 60)}m ${String(s % 60).padStart(2, "0")}s`
}
