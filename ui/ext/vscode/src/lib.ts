// Pure helpers shared by the extension entry point. Kept free of any
// `vscode` import so they can be unit-tested with plain `node --test`.

/**
 * Formats a codg file mention: `@path`, `@path#L3`, or `@path#L3-9`.
 * Line numbers are 1-based.
 */
export function formatFileReference(
  relativePath: string,
  startLine?: number,
  endLine?: number,
): string {
  let ref = `@${relativePath}`
  if (startLine !== undefined) {
    if (endLine !== undefined && endLine !== startLine) {
      ref += `#L${startLine}-${endLine}`
    } else {
      ref += `#L${startLine}`
    }
  }
  return ref
}

/**
 * Converts a 0-based editor selection into a 1-based inclusive line
 * range, or undefined when the selection is empty. A selection that
 * ends at column 0 of a later line (shift+down, triple-click, dragging
 * to a line start) does not include that line.
 */
export function selectionLineRange(
  start: { line: number; character: number },
  end: { line: number; character: number },
): [number, number] | undefined {
  if (start.line === end.line && start.character === end.character) return undefined
  const endLine = end.character === 0 && end.line > start.line ? end.line : end.line + 1
  return [start.line + 1, endLine]
}

/**
 * Maps the host application name (vscode.env.appName) to the
 * CODG_CALLER value so the CLI knows which editor launched it.
 */
export function resolveCaller(appName: string): string {
  const name = appName.toLowerCase()
  if (name.includes("cursor")) return "cursor"
  if (name.includes("windsurf")) return "windsurf"
  if (name.includes("vscodium")) return "vscodium"
  return "vscode"
}

/** Editor context attached to a chat prompt. */
export interface PromptContext {
  /** Absolute file:// URI of the active file. */
  uri: string
  /** Display name, usually the workspace-relative path. */
  name: string
  selection?: { text: string; startLine: number; endLine: number }
}

/** Short label for the context chip, e.g. `src/a.ts:L3-9`. */
export function contextLabel(ctx: PromptContext): string {
  if (!ctx.selection) return ctx.name
  const { startLine, endLine } = ctx.selection
  return startLine === endLine ? `${ctx.name}:L${startLine}` : `${ctx.name}:L${startLine}-${endLine}`
}

/**
 * Builds ACP prompt content blocks: the user's text, a resource_link
 * for the active file, the selected text as an embedded resource, and a
 * resource_link per @-mentioned file (deduplicated by URI).
 */
export function buildPrompt(text: string, ctx?: PromptContext, mentions: { uri: string; name: string }[] = []): unknown[] {
  const blocks: unknown[] = [{ type: "text", text }]
  const linked = new Set<string>()
  const link = (uri: string, name: string) => {
    if (linked.has(uri)) return
    linked.add(uri)
    blocks.push({ type: "resource_link", uri, name })
  }
  if (ctx) {
    link(ctx.uri, ctx.name)
    if (ctx.selection) {
      const { startLine, endLine } = ctx.selection
      blocks.push({
        type: "resource",
        resource: { uri: `${ctx.uri}#L${startLine}-${endLine}`, text: ctx.selection.text },
      })
    }
  }
  for (const m of mentions) link(m.uri, m.name)
  return blocks
}

/** A selectable session option (ACP mode, model, effort or approval). */
export interface Choice {
  id: string
  name: string
  description?: string
}

export interface ChoiceSet {
  current?: string
  options: Choice[]
  /** ACP session config option id; set via session/set_config_option. */
  configId?: string
  /** Legacy ACP setter for sets without a configId. */
  method?: "session/set_mode" | "session/set_model"
}

export type ChoiceKind = "mode" | "model" | "effort" | "approval"
export type SessionChoices = Partial<Record<ChoiceKind, ChoiceSet>>

const CATEGORY_KINDS: Record<string, ChoiceKind> = {
  mode: "mode",
  model: "model",
  thought_level: "effort",
  _approval: "approval",
  _permission: "approval",
}

// Option sets offering a yolo/auto-approve value are approval policies,
// whatever the agent calls them.
const isApproval = (options: Choice[]) => options.some((o) => /yolo/i.test(`${o.id} ${o.name}`))

/** Footer kind of a config option: its category, else keywords in its id/name. */
function configKind(o: any): ChoiceKind | undefined {
  const byCategory = CATEGORY_KINDS[o?.category]
  if (byCategory) return byCategory
  const key = `${o?.id ?? ""} ${o?.name ?? ""}`.toLowerCase()
  if (/approv|permission|yolo/.test(key)) return "approval"
  if (/effort|reason|thought|think/.test(key)) return "effort"
  if (/model/.test(key)) return "model"
  if (/agent|mode/.test(key)) return "mode"
  return undefined
}

function toChoice(v: any): Choice | undefined {
  const id = [v?.value, v?.id, v?.modelId].find((x) => typeof x === "string" && x)
  return id ? { id, name: v.name || id, description: v.description || undefined } : undefined
}

function choiceList(list: unknown): Choice[] {
  if (!Array.isArray(list)) return []
  return list
    .flatMap((v) => (Array.isArray(v?.options) ? v.options : [v]))
    .map(toChoice)
    .filter((c): c is Choice => !!c)
}

/**
 * Maps ACP `configOptions` (select type, flat or grouped) to footer
 * choices. The category decides the kind; options without one (it is
 * optional in ACP) are matched by id/name, e.g. `agent`, `model`,
 * `reasoning_effort`, `permission`.
 */
export function configChoices(list: unknown): SessionChoices {
  const out: SessionChoices = {}
  if (!Array.isArray(list)) return out
  for (const o of list) {
    let kind = configKind(o)
    if (!kind || (o.type !== undefined && o.type !== "select")) continue
    const options = choiceList(o.options)
    if (!options.length) continue
    if (kind === "mode" && isApproval(options)) kind = "approval"
    if (out[kind]) continue
    out[kind] = { current: typeof o.currentValue === "string" ? o.currentValue : undefined, options, configId: o.id }
  }
  return out
}

/**
 * Normalizes the `modes` / `models` of an ACP session/new (or
 * session/load) response; `configOptions` take precedence. Legacy modes
 * that offer a yolo value become the approval policy.
 */
export function sessionChoices(res: any): SessionChoices {
  const set = (current: unknown, list: unknown, method: ChoiceSet["method"]): ChoiceSet | undefined => {
    const options = choiceList(list)
    if (!options.length) return undefined
    return { current: typeof current === "string" ? current : undefined, options, method }
  }
  const out: SessionChoices = {}
  const modes = set(res?.modes?.currentModeId, res?.modes?.availableModes, "session/set_mode")
  if (modes) out[isApproval(modes.options) ? "approval" : "mode"] = modes
  const models = set(res?.models?.currentModelId, res?.models?.availableModels, "session/set_model")
  if (models) out.model = models
  return { ...out, ...configChoices(res?.configOptions) }
}

export type Approval = "ask" | "yolo-work" | "yolo-full"

/** Client-side tool approval used when the agent offers no policy of its own. */
export function approvalChoices(current: Approval): ChoiceSet {
  return {
    current,
    options: [
      { id: "ask", name: "Ask", description: "Ask before every tool call" },
      { id: "yolo-work", name: "Yolo (work)", description: "Auto-approve reads and edits inside the workspace; ask for shell, network and deletes" },
      { id: "yolo-full", name: "Yolo (full)", description: "Auto-approve every tool call" },
    ],
  }
}

/**
 * The allow option to pick for a permission request, if any.
 * Prefers allow_once: codg maps allow_always to a process-lifetime grant,
 * which would keep skipping prompts after switching back to Ask.
 */
export function autoApproveOption(options: unknown): string | undefined {
  if (!Array.isArray(options)) return undefined
  const pick = options.find((o) => o?.kind === "allow_once") ?? options.find((o) => o?.kind === "allow_always")
  return typeof pick?.optionId === "string" ? pick.optionId : undefined
}

const WORK_KINDS = new Set(["read", "search", "think", "edit", "move"])

/** True when `p` is `root` or inside it (POSIX or Windows paths). */
export function isInside(root: string, p: string): boolean {
  const norm = (s: string) => s.replace(/\\/g, "/").replace(/\/+$/, "")
  const r = norm(root)
  const x = norm(p)
  return !!r && (x === r || x.startsWith(`${r}/`))
}

/**
 * Answers a session/request_permission under a client-side policy:
 * `yolo-full` approves everything; `yolo-work` approves read/search/
 * think/edit/move calls whose locations all lie inside `cwd` (edits and
 * moves must name a location); `ask` (and anything else) returns
 * undefined so the user is asked.
 */
export function autoApprove(policy: Approval, toolCall: any, options: unknown, cwd: string): string | undefined {
  if (policy === "yolo-full") return autoApproveOption(options)
  if (policy !== "yolo-work" || !WORK_KINDS.has(toolCall?.kind)) return undefined
  const paths: unknown[] = Array.isArray(toolCall?.locations) ? toolCall.locations.map((l: any) => l?.path) : []
  if ((toolCall.kind === "edit" || toolCall.kind === "move") && !paths.length) return undefined
  if (!paths.every((p) => typeof p === "string" && isInside(cwd, p))) return undefined
  return autoApproveOption(options)
}

/** Agent name and version from the ACP initialize response (`agentInfo`). */
export function agentInfo(init: any): { name: string; version?: string } | undefined {
  const info = init?.agentInfo
  const name = info?.title || info?.name
  if (typeof name !== "string" || !name) return undefined
  return { name, version: typeof info.version === "string" && info.version ? info.version : undefined }
}

/** Default (empty id) agent plus codg agent templates, selected via an `@name` prompt prefix. */
export function templateAgentChoices(files: string[], current = ""): ChoiceSet {
  const names = [
    ...new Set(files.map((f) => /^(.+?)(?:\.md\.tpl|\.tpl|\.md)$/.exec(f)?.[1]).filter((n): n is string => !!n)),
  ].sort()
  return {
    current,
    options: [
      { id: "", name: "Agent", description: "Codg default agent" },
      ...names.map((n) => ({ id: n, name: `@${n}`, description: "Agent template (~/.codg/agents/templates)" })),
    ],
  }
}

/** Parses `codg models` output (`provider/model` per line) into model choices. */
export function parseModelList(stdout: string): Choice[] {
  const seen = new Set<string>()
  const out: Choice[] = []
  for (const line of stdout.split(/\r?\n/)) {
    const m = /^\s*([\w.-]+)\/(\S+)\s*$/.exec(line)
    if (!m || seen.has(m[0].trim())) continue
    seen.add(m[0].trim())
    out.push({ id: `${m[1]}/${m[2]}`, name: m[2], description: m[1] })
  }
  return out
}

/** A past chat for the history picker. */
export interface SessionEntry {
  sessionId: string
  cwd: string
  title?: string
  /** ISO 8601 timestamp of the last activity. */
  updatedAt?: string
}

/** Sessions of an ACP `session/list` response. */
export function parseSessionList(res: any): SessionEntry[] {
  if (!Array.isArray(res?.sessions)) return []
  return res.sessions
    .filter((s: any) => typeof s?.sessionId === "string" && typeof s?.cwd === "string")
    .map((s: any): SessionEntry => ({
      sessionId: s.sessionId,
      cwd: s.cwd,
      title: typeof s.title === "string" && s.title ? s.title : undefined,
      updatedAt: typeof s.updatedAt === "string" ? s.updatedAt : undefined,
    }))
}

/** Merges history lists by id (first list wins), newest first, up to `limit`. */
export function mergeSessions(lists: SessionEntry[][], limit = 50): SessionEntry[] {
  const byId = new Map<string, SessionEntry>()
  for (const list of lists) for (const s of list) if (!byId.has(s.sessionId)) byId.set(s.sessionId, s)
  return [...byId.values()]
    .sort((a, b) => (b.updatedAt ?? "").localeCompare(a.updatedAt ?? ""))
    .slice(0, limit)
}

/** Chat title from its first prompt: first line, at most 60 characters. */
export function sessionTitle(text: string): string {
  const line = text.trim().split("\n")[0].trim()
  return line.length > 60 ? `${line.slice(0, 59)}…` : line
}

/** "just now", "5m ago", "3h ago", "2d ago", else the date. */
export function relativeTime(iso: string | undefined, now = Date.now()): string {
  const t = iso ? Date.parse(iso) : NaN
  if (Number.isNaN(t)) return ""
  const s = Math.max(0, Math.floor((now - t) / 1000))
  if (s < 60) return "just now"
  if (s < 3600) return `${Math.floor(s / 60)}m ago`
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`
  if (s < 7 * 86400) return `${Math.floor(s / 86400)}d ago`
  return new Date(t).toISOString().slice(0, 10)
}

/** Display name of the current choice, falling back to its id. */
export function choiceName(set?: ChoiceSet): string | undefined {
  if (!set?.current) return undefined
  return set.options.find((o) => o.id === set.current)?.name ?? set.current
}

/** Unique `@path` mentions in a prompt, without any `#L3-9` suffix. */
export function extractMentions(text: string): string[] {
  const paths = [...text.matchAll(/(?:^|\s)@([^\s@#]+)(?:#L\d+(?:-\d+)?)?/g)].map((m) => m[1])
  return [...new Set(paths)]
}

/**
 * Ranks workspace-relative paths for the @-mention picker: basename
 * prefix, then basename substring, then path substring (case-insensitive).
 * An empty query keeps the input order.
 */
export function rankFiles(paths: string[], query: string, limit = 20): string[] {
  const q = query.toLowerCase()
  if (!q) return paths.slice(0, limit)
  const score = (p: string) => {
    const lower = p.toLowerCase()
    const base = lower.slice(lower.lastIndexOf("/") + 1)
    if (base.startsWith(q)) return 0
    if (base.includes(q)) return 1
    return lower.includes(q) ? 2 : -1
  }
  return paths
    .map((p) => [p, score(p)] as const)
    .filter(([, s]) => s >= 0)
    .sort((a, b) => a[1] - b[1] || a[0].length - b[0].length)
    .slice(0, limit)
    .map(([p]) => p)
}

export interface TerminalAuthMethod {
  id: string
  name: string
  type: "terminal"
  args?: string[]
  env?: Record<string, string>
}

/** Returns the first terminal auth method (e.g. `codg login`) advertised by the agent. */
export function findTerminalAuth(methods: unknown): TerminalAuthMethod | undefined {
  if (!Array.isArray(methods)) return undefined
  return methods.find((m) => m?.type === "terminal")
}

/** Builds the shell command line used to launch codg. */
export function buildLaunchCommand(command: string, args: string[]): string {
  return [command, ...args]
    .map((part) => (/[\s"']/.test(part) ? JSON.stringify(part) : part))
    .join(" ")
}
