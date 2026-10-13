// Chat panel host: runs `codg acp` and bridges ACP traffic to a chat
// webview (sidebar view or editor tab), like the Claude Code / Codex
// panels. One agent process per webview; "New Chat" opens a fresh
// ACP session on the same process.

import { execFile, spawn, type ChildProcess } from "node:child_process"
import { readdir } from "node:fs/promises"
import * as os from "node:os"
import * as path from "node:path"
import * as vscode from "vscode"
import { AcpConnection, AUTH_REQUIRED, METHOD_NOT_FOUND, PROTOCOL_VERSION, RpcError } from "./acp"
import {
  agentInfo,
  approvalChoices,
  autoApprove,
  buildPrompt,
  choiceName,
  configChoices,
  contextLabel,
  extractMentions,
  findTerminalAuth,
  mergeSessions,
  parseModelList,
  parseSessionList,
  rankFiles,
  relativeTime,
  resolveCaller,
  selectionLineRange,
  sessionChoices,
  sessionTitle,
  templateAgentChoices,
  type Approval,
  type Choice,
  type ChoiceKind,
  type ChoiceSet,
  type PromptContext,
  type SessionChoices,
  type SessionEntry,
} from "./lib"

export const CHAT_VIEW_ID = "codg.chat"

const PICK_PLACEHOLDER: Record<ChoiceKind, string> = {
  mode: "Select agent",
  model: "Select model",
  effort: "Select reasoning effort",
  approval: "Select tool approval",
}

// Chats this extension started, per workspace (workspaceState key).
const HISTORY_KEY = "codg.sessions"
// `codg models` can be slow on first run (provider discovery).
const MODELS_TIMEOUT_MS = 30_000

// Workspace file list for @-mentions is cached this long.
const FILE_CACHE_MS = 30_000
const MAX_FILES = 5000
// Updates that arrive before session/new resolves are replayed, up to this many.
const MAX_EARLY_UPDATES = 50

type FromWebview =
  | { type: "ready" }
  | { type: "prompt"; text: string; includeContext: boolean }
  | { type: "cancel" }
  | { type: "newChat" }
  | { type: "permission"; id: number; optionId?: string }
  | { type: "openFile"; path: string; line?: number }
  | { type: "pick"; kind: ChoiceKind }
  | { type: "cycleMode" }
  | { type: "terminal" }
  | { type: "searchFiles"; id: number; query: string }
  | { type: "copy"; text: string }
  | { type: "insertCode"; text: string }
  | { type: "history" }

export class ChatSession implements vscode.Disposable {
  private proc?: ChildProcess
  private conn?: AcpConnection
  private connecting?: Promise<AcpConnection>
  private authMethods: unknown
  private agent?: { name: string; version?: string }
  private canLoad = false
  private canList = false
  private sessionId?: string
  private cwd = workspaceCwd()
  private starting?: Promise<string>
  private early: any[] = []
  private choices: SessionChoices = {}
  // Client-side tool approval, used when the agent offers no policy.
  private approval: Approval = "ask"
  // Agent template sent as an `@name` prompt prefix when the agent has no agent/mode option.
  private agentTemplate = ""
  private files?: { at: number; paths: Promise<string[]> }
  private busy = false
  private ready = false
  private queued: unknown[] = []
  private nextPermission = 1
  private permissions = new Map<number, (optionId?: string) => void>()
  private readonly disposables: vscode.Disposable[] = []

  constructor(
    private readonly webview: vscode.Webview,
    extensionUri: vscode.Uri,
    private readonly log: vscode.OutputChannel,
    private readonly state: vscode.Memento,
  ) {
    const dist = vscode.Uri.joinPath(extensionUri, "dist")
    webview.options = { enableScripts: true, localResourceRoots: [dist] }
    webview.html = chatHtml(webview, dist)
    this.disposables.push(
      webview.onDidReceiveMessage((m: FromWebview) => this.onMessage(m)),
      vscode.window.onDidChangeActiveTextEditor(() => this.postContext()),
      vscode.window.onDidChangeTextEditorSelection(() => this.postContext()),
    )
  }

  /** Starts a fresh conversation (keeps the agent process). */
  newChat() {
    this.cancel()
    this.sessionId = undefined
    this.starting = undefined
    void this.post({ type: "reset" })
    this.warmUp()
  }

  /** Types text (e.g. an @file mention) into the composer. */
  insert(text: string) {
    void this.post({ type: "insert", text })
  }

  dispose() {
    this.resolvePermissions()
    this.conn?.close(new Error("chat closed"))
    this.proc?.kill()
    for (const d of this.disposables) d.dispose()
  }

  private async onMessage(m: FromWebview) {
    switch (m.type) {
      case "ready":
        this.ready = true
        for (const msg of this.queued.splice(0)) void this.webview.postMessage(msg)
        this.postContext()
        this.warmUp()
        return
      case "prompt":
        await this.prompt(m.text, m.includeContext)
        return
      case "cancel":
        this.cancel()
        return
      case "newChat":
        this.newChat()
        return
      case "permission":
        this.permissions.get(m.id)?.(m.optionId)
        this.permissions.delete(m.id)
        return
      case "openFile":
        await this.openFile(m.path, m.line)
        return
      case "pick":
        await this.pick(m.kind)
        return
      case "cycleMode":
        await this.cycleMode()
        return
      case "terminal":
        await vscode.commands.executeCommand("codg.openTerminal")
        return
      case "searchFiles":
        await this.searchFiles(m.id, m.query)
        return
      case "copy":
        await vscode.env.clipboard.writeText(m.text)
        return
      case "insertCode":
        await this.insertCode(m.text)
        return
      case "history":
        await this.showHistory()
        return
    }
  }

  /** Lists past chats (ACP session/list plus chats started here) and reopens the picked one. */
  async showHistory() {
    try {
      await this.connect(workspaceCwd())
      let remote: SessionEntry[] = []
      if (this.canList) {
        try {
          remote = parseSessionList(await this.conn!.request("session/list", { cwd: workspaceCwd() }))
        } catch (err) {
          this.log.appendLine(`session/list: ${(err as Error).message}`)
        }
      }
      const sessions = mergeSessions([remote, this.state.get<SessionEntry[]>(HISTORY_KEY, [])])
      if (!sessions.length) {
        void this.post({ type: "info", message: "No past chats yet." })
        return
      }
      const item = await vscode.window.showQuickPick(
        sessions.map((s) => ({
          label: s.title || "Untitled chat",
          description: relativeTime(s.updatedAt),
          detail: `${path.basename(s.cwd)}${s.sessionId === this.sessionId ? " · current" : ""}`,
          session: s,
        })),
        { placeHolder: "Open a past chat", matchOnDescription: true },
      )
      if (item && item.session.sessionId !== this.sessionId) await this.loadSession(item.session)
    } catch (err) {
      void this.post({ type: "error", message: (err as Error).message })
    }
  }

  /** Reopens a chat with ACP session/load; the agent replays it as session/update notifications. */
  private async loadSession(entry: SessionEntry) {
    if (!this.canLoad) {
      void this.post({ type: "info", message: "This codg acp cannot reopen past chats (loadSession: false). Update codg." })
      return
    }
    this.cancel()
    this.starting = undefined
    this.sessionId = entry.sessionId
    void this.post({ type: "reset", title: entry.title })
    this.setBusy(true)
    try {
      const res = await this.conn!.request("session/load", { sessionId: entry.sessionId, cwd: entry.cwd, mcpServers: [] })
      this.cwd = entry.cwd
      this.applyChoices(sessionChoices(res))
      this.remember(entry.sessionId)
    } catch (err) {
      this.sessionId = undefined
      void this.post({ type: "error", message: `Could not reopen chat: ${(err as Error).message}` })
    } finally {
      this.setBusy(false)
    }
  }

  /** Records the current chat for the history picker; the first prompt titles it. */
  private remember(sessionId: string, firstPrompt?: string) {
    const saved = this.state.get<SessionEntry[]>(HISTORY_KEY, [])
    const prev = saved.find((s) => s.sessionId === sessionId)
    const entry: SessionEntry = {
      sessionId,
      cwd: this.cwd,
      title: prev?.title ?? (firstPrompt ? sessionTitle(firstPrompt) : undefined),
      updatedAt: new Date().toISOString(),
    }
    this.state.update(HISTORY_KEY, mergeSessions([[entry], saved])).then(undefined, (err) =>
      this.log.appendLine(`save history: ${(err as Error).message}`),
    )
  }

  private async prompt(text: string, includeContext: boolean) {
    this.setBusy(true)
    try {
      const sessionId = await this.ensureSession()
      this.remember(sessionId, text)
      const mentions = await resolveMentions(text, this.cwd)
      const sent = this.agentTemplate ? `@${this.agentTemplate} ${text}` : text
      const prompt = buildPrompt(sent, includeContext ? activeContext() : undefined, mentions)
      const res = await this.conn!.request<{ stopReason: string; usage?: { totalTokens?: number } }>(
        "session/prompt",
        { sessionId, prompt },
      )
      if (res.usage?.totalTokens) void this.post({ type: "usage", usage: { totalTokens: res.usage.totalTokens } })
      if (res.stopReason !== "end_turn" && res.stopReason !== "cancelled") {
        void this.post({ type: "info", message: `Stopped: ${res.stopReason}` })
      }
    } catch (err) {
      void this.post({ type: "error", message: (err as Error).message })
    } finally {
      this.setBusy(false)
    }
  }

  private cancel() {
    if (this.busy && this.sessionId) this.conn?.notify("session/cancel", { sessionId: this.sessionId })
    this.resolvePermissions()
  }

  private resolvePermissions() {
    for (const resolve of this.permissions.values()) resolve(undefined)
    this.permissions.clear()
  }

  private setBusy(busy: boolean) {
    this.busy = busy
    void this.post({ type: "status", busy })
  }

  /**
   * Starts the session in the background so the footer can show the
   * mode, model and agent slash commands before the first prompt. A
   * missing login is reported, not prompted; the next prompt runs it.
   */
  private warmUp() {
    this.ensureSession(false).catch((err) => {
      if (err instanceof RpcError && err.code === AUTH_REQUIRED) {
        void this.post({ type: "info", message: "Codg needs a provider login — send a message to sign in." })
      } else {
        void this.post({ type: "error", message: (err as Error).message })
      }
    })
  }

  private ensureSession(interactive = true): Promise<string> {
    if (this.sessionId) return Promise.resolve(this.sessionId)
    if (this.starting && !interactive) return this.starting
    if (this.starting) {
      // A background start that hit the login wall is retried with login.
      return this.starting.catch((err) => {
        if (err instanceof RpcError && err.code === AUTH_REQUIRED) return this.ensureSession(true)
        throw err
      })
    }
    const starting: Promise<string> = this.startSession(interactive).then(
      (id) => {
        // A New Chat while starting abandons this session.
        if (this.starting !== starting) return id
        this.sessionId = id
        for (const params of this.early.splice(0)) this.onSessionUpdate(params)
        return id
      },
      (err) => {
        if (this.starting === starting) this.starting = undefined
        throw err
      },
    )
    this.starting = starting
    return starting
  }

  private async startSession(interactive: boolean): Promise<string> {
    const cwd = workspaceCwd()
    this.early = []
    const conn = await this.connect(cwd)
    let res: any
    try {
      res = await conn.request("session/new", { cwd, mcpServers: [] })
    } catch (err) {
      const method = findTerminalAuth(this.authMethods)
      if (!interactive || !(err instanceof RpcError) || err.code !== AUTH_REQUIRED || !method) throw err
      void this.post({ type: "info", message: "Codg needs a provider login — finish it in the terminal." })
      const { command } = launchConfig()
      if (!(await runInTerminal(command, method.args ?? [], method.env ?? {}, cwd))) {
        throw new Error("Codg login did not complete.")
      }
      res = await conn.request("session/new", { cwd, mcpServers: [] })
    }
    this.cwd = cwd
    this.applyChoices(sessionChoices(res))
    return res.sessionId
  }

  /**
   * Adopts the agent's choices, then fills what it does not offer:
   * agent templates (sent as an `@name` prefix) and `codg models`
   * (switched with session/set_model).
   */
  private applyChoices(choices: SessionChoices) {
    this.choices = choices
    this.postSession()
    void this.fillFallbacks(choices)
  }

  private async fillFallbacks(choices: SessionChoices) {
    const [templates, models] = await Promise.all([
      choices.mode ? undefined : listAgentTemplates(this.log),
      choices.model ? undefined : listModels(this.log),
    ])
    if (this.choices !== choices) return
    if (templates) choices.mode = templateAgentChoices(templates, this.agentTemplate)
    if (models?.length) choices.model = { options: models, method: "session/set_model" }
    this.postSession()
  }

  /** Starts `codg acp` once; concurrent callers share the start. */
  private connect(cwd: string): Promise<AcpConnection> {
    if (this.conn) return Promise.resolve(this.conn)
    this.connecting ??= this.startAgent(cwd).finally(() => (this.connecting = undefined))
    return this.connecting
  }

  private onSessionUpdate(params: any) {
    if (params?.sessionId !== this.sessionId) return
    const update = params.update
    if (update?.sessionUpdate === "current_mode_update") {
      const set = this.choices.mode?.method ? this.choices.mode : this.choices.approval
      if (set?.method === "session/set_mode") set.current = update.currentModeId
      this.postSession()
    }
    if (update?.sessionUpdate === "config_option_update") {
      Object.assign(this.choices, configChoices(update.configOptions))
      this.postSession()
    }
    void this.post({ type: "update", update })
  }

  private async startAgent(cwd: string): Promise<AcpConnection> {
    const { command, env } = launchConfig()
    const proc = spawn(command, ["acp"], { cwd, env: { ...process.env, ...env } })
    const conn = new AcpConnection(proc.stdout, proc.stdin)
    proc.stderr.on("data", (d) => this.log.append(String(d)))
    proc.on("error", (err) =>
      conn.close(new Error(`Failed to start "${command}": ${err.message}. Set "codg.command" to the codg path.`)),
    )
    conn.onClose = (err) => {
      this.log.appendLine(`codg acp: ${err.message}`)
      if (this.conn === conn) {
        this.conn = undefined
        this.sessionId = undefined
        this.starting = undefined
      }
    }
    conn.onNotification = (method, params) => {
      if (method !== "session/update") return
      // Agents may announce commands/modes before session/new resolves.
      if (!this.sessionId && this.starting && this.early.length < MAX_EARLY_UPDATES) {
        this.early.push(params)
        return
      }
      this.onSessionUpdate(params)
    }
    conn.onRequest = (method, params) => this.onAgentRequest(method, params)
    this.proc = proc
    this.conn = conn

    const init = await conn.request("initialize", {
      protocolVersion: PROTOCOL_VERSION,
      clientCapabilities: { fs: { readTextFile: false, writeTextFile: false }, terminal: false, auth: { terminal: true } },
      clientInfo: { name: "codg-vscode", version: vscode.version },
    })
    this.authMethods = init.authMethods
    this.agent = agentInfo(init)
    this.canLoad = !!init?.agentCapabilities?.loadSession
    this.canList = !!init?.agentCapabilities?.sessionCapabilities?.list
    return conn
  }

  private async onAgentRequest(method: string, params: any): Promise<unknown> {
    if (method !== "session/request_permission") throw new RpcError(METHOD_NOT_FOUND, `method not found: ${method}`)
    // An agent-side approval policy decides itself; the client one answers here.
    const auto = this.choices.approval ? undefined : autoApprove(this.approval, params.toolCall, params.options, this.cwd)
    if (auto) return { outcome: { outcome: "selected", optionId: auto } }
    const id = this.nextPermission++
    const optionId = await new Promise<string | undefined>((resolve) => {
      this.permissions.set(id, resolve)
      void this.post({ type: "permission", id, toolCall: params.toolCall, options: params.options })
    })
    return { outcome: optionId ? { outcome: "selected", optionId } : { outcome: "cancelled" } }
  }

  private postContext() {
    const ctx = activeContext()
    void this.post({ type: "context", label: ctx ? contextLabel(ctx) : undefined })
  }

  private choiceSet(kind: ChoiceKind): ChoiceSet | undefined {
    if (kind === "approval") return this.choices.approval ?? approvalChoices(this.approval)
    return this.choices[kind]
  }

  private postSession() {
    const { mode, model, effort } = this.choices
    const approval = this.choiceSet("approval")!
    const level = approval.current ?? ""
    void this.post({
      type: "session",
      mode: choiceName(mode) ?? "Agent",
      agent: this.agent && `${this.agent.name}${this.agent.version ? ` ${this.agent.version}` : ""}`,
      model: choiceName(model) ?? (model ? "Model" : undefined),
      effort: choiceName(effort),
      approval: choiceName(approval),
      yolo: /yolo/i.test(level) ? (/work/i.test(level) ? "work" : "full") : undefined,
      modes: (mode?.options.length ?? 0) > 1,
      models: (model?.options.length ?? 0) > (model?.current ? 1 : 0),
      efforts: (effort?.options.length ?? 0) > 1,
      cwd: path.basename(this.cwd),
    })
  }

  /** Quick pick for the agent, model, reasoning effort or tool approval. */
  private async pick(kind: ChoiceKind) {
    try {
      if (kind !== "approval") await this.ensureSession()
      const set = this.choiceSet(kind)
      if (!set?.options.length) {
        void this.post({ type: "info", message: `Codg does not offer ${kind} selection for this session.` })
        return
      }
      const item = await vscode.window.showQuickPick(
        set.options.map((o) => ({
          label: o.name,
          description: o.id === set.current ? "current" : undefined,
          detail: o.description,
          id: o.id,
        })),
        { placeHolder: PICK_PLACEHOLDER[kind], matchOnDescription: true, matchOnDetail: true },
      )
      if (item) await this.select(kind, item.id)
    } catch (err) {
      void this.post({ type: "error", message: (err as Error).message })
    }
  }

  /** Switches to the next mode (Shift+Tab in the composer). */
  private async cycleMode() {
    const set = this.choices.mode
    if (!set || set.options.length < 2) return
    const i = set.options.findIndex((o) => o.id === set.current)
    try {
      await this.select("mode", set.options[(i + 1) % set.options.length].id)
    } catch (err) {
      void this.post({ type: "error", message: (err as Error).message })
    }
  }

  private async select(kind: ChoiceKind, id: string) {
    const set = this.choiceSet(kind)
    if (!set || id === set.current) return
    if (set.configId || set.method) {
      if (!this.sessionId) return
      await this.setRemote(set, id)
    } else if (kind === "approval") {
      this.approval = id as Approval
    } else if (kind === "mode") {
      this.agentTemplate = id
      set.current = id
    }
    this.postSession()
  }

  private async setRemote(set: ChoiceSet, id: string) {
    const sessionId = this.sessionId
    if (set.configId) {
      const res = await this.conn!.request<{ configOptions?: unknown }>("session/set_config_option", {
        sessionId,
        configId: set.configId,
        value: id,
      })
      set.current = id
      // The response carries the full state (e.g. efforts of a new model).
      Object.assign(this.choices, configChoices(res?.configOptions))
      return
    }
    const params = set.method === "session/set_mode" ? { sessionId, modeId: id } : { sessionId, modelId: id }
    try {
      await this.conn!.request(set.method!, params)
    } catch (err) {
      if (err instanceof RpcError && err.code === METHOD_NOT_FOUND) {
        throw new Error(`This codg acp does not support ${set.method}. Update codg to switch it from the editor.`)
      }
      throw err
    }
    set.current = id
  }

  /** Answers the @-mention picker: open editors first, then workspace files. */
  private async searchFiles(id: number, query: string) {
    if (!this.files || Date.now() - this.files.at > FILE_CACHE_MS) {
      this.files = { at: Date.now(), paths: listWorkspaceFiles() }
    }
    let paths: string[]
    try {
      paths = await this.files.paths
    } catch (err) {
      this.files = undefined
      this.log.appendLine(`file search: ${(err as Error).message}`)
      paths = []
    }
    const open = vscode.workspace.textDocuments
      .filter((d) => d.uri.scheme === "file")
      .map((d) => vscode.workspace.asRelativePath(d.uri, false))
    void this.post({ type: "files", id, items: rankFiles([...new Set([...open, ...paths])], query) })
  }

  /** Inserts a code block at the cursor of the last focused editor. */
  private async insertCode(text: string) {
    const editor = vscode.window.activeTextEditor ?? vscode.window.visibleTextEditors[0]
    if (!editor) {
      void this.post({ type: "info", message: "Open an editor to insert code." })
      return
    }
    await editor.edit((b) => b.replace(editor.selection, text))
    await vscode.window.showTextDocument(editor.document, editor.viewColumn)
  }

  private async openFile(path: string, line?: number) {
    const pos = new vscode.Position(Math.max((line ?? 1) - 1, 0), 0)
    await vscode.window.showTextDocument(vscode.Uri.file(path), {
      viewColumn: vscode.ViewColumn.One,
      selection: new vscode.Range(pos, pos),
    })
  }

  /** Messages sent before the webview script is listening are queued. */
  private post(msg: unknown): Thenable<boolean> {
    if (!this.ready) {
      this.queued.push(msg)
      return Promise.resolve(true)
    }
    return this.webview.postMessage(msg)
  }
}

/** Hosts the chat in the activity bar sidebar. */
export class ChatViewProvider implements vscode.WebviewViewProvider, vscode.Disposable {
  private session?: ChatSession

  constructor(
    private readonly extensionUri: vscode.Uri,
    private readonly log: vscode.OutputChannel,
    private readonly state: vscode.Memento,
  ) {}

  resolveWebviewView(view: vscode.WebviewView) {
    this.session?.dispose()
    const session = new ChatSession(view.webview, this.extensionUri, this.log, this.state)
    this.session = session
    view.onDidDispose(() => {
      session.dispose()
      if (this.session === session) this.session = undefined
    })
  }

  newChat() {
    this.session?.newChat()
  }

  insert(text: string) {
    this.session?.insert(text)
  }

  showHistory() {
    return this.session?.showHistory()
  }

  dispose() {
    this.session?.dispose()
  }
}

/** Opens the chat in an editor tab beside the active editor. */
export function openChatPanel(extensionUri: vscode.Uri, log: vscode.OutputChannel, state: vscode.Memento) {
  const panel = vscode.window.createWebviewPanel("codg.chatPanel", "Codg", vscode.ViewColumn.Beside, {
    retainContextWhenHidden: true,
  })
  panel.iconPath = {
    light: vscode.Uri.joinPath(extensionUri, "images/button-dark.svg"),
    dark: vscode.Uri.joinPath(extensionUri, "images/button-light.svg"),
  }
  const session = new ChatSession(panel.webview, extensionUri, log, state)
  panel.onDidDispose(() => session.dispose())
}

function launchConfig() {
  const config = vscode.workspace.getConfiguration("codg")
  return {
    command: config.get<string>("command", "codg"),
    env: { CODG_CALLER: resolveCaller(vscode.env.appName) },
  }
}

function workspaceCwd(): string {
  const editor = vscode.window.activeTextEditor
  const folder = (editor && vscode.workspace.getWorkspaceFolder(editor.document.uri)) ?? vscode.workspace.workspaceFolders?.[0]
  return folder?.uri.fsPath ?? os.homedir()
}

async function listWorkspaceFiles(): Promise<string[]> {
  const uris = await vscode.workspace.findFiles("**/*", "**/{node_modules,.git}/**", MAX_FILES)
  return uris.map((u) => vscode.workspace.asRelativePath(u, false))
}

/** Files in ~/.codg/agents/templates (none when the directory is missing). */
async function listAgentTemplates(log: vscode.OutputChannel): Promise<string[]> {
  try {
    return await readdir(path.join(os.homedir(), ".codg", "agents", "templates"))
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code !== "ENOENT") log.appendLine(`agent templates: ${(err as Error).message}`)
    return []
  }
}

let modelList: Promise<Choice[]> | undefined

/** `codg models` (provider/model per line), cached for the extension's lifetime once non-empty. */
function listModels(log: vscode.OutputChannel): Promise<Choice[]> {
  modelList ??= new Promise<Choice[]>((resolve) => {
    const { command, env } = launchConfig()
    execFile(command, ["models"], { env: { ...process.env, ...env }, timeout: MODELS_TIMEOUT_MS }, (err, stdout) => {
      if (err) log.appendLine(`codg models: ${err.message}`)
      const models = parseModelList(stdout ?? "")
      if (!models.length) modelList = undefined
      resolve(models)
    })
  })
  return modelList
}

/** Resolves `@path` mentions in a prompt to existing files under cwd (or absolute paths). */
async function resolveMentions(text: string, cwd: string): Promise<{ uri: string; name: string }[]> {
  const found: { uri: string; name: string }[] = []
  for (const p of extractMentions(text)) {
    const uri = vscode.Uri.file(path.isAbsolute(p) ? p : path.join(cwd, p))
    try {
      const stat = await vscode.workspace.fs.stat(uri)
      if (stat.type & vscode.FileType.File) found.push({ uri: uri.toString(), name: p })
    } catch {
      // Not a file (e.g. an @handle in prose): leave it as plain text.
    }
  }
  return found
}

/** Active file (and non-empty selection) as prompt context. */
function activeContext(): PromptContext | undefined {
  const editor = vscode.window.activeTextEditor
  if (!editor || editor.document.uri.scheme !== "file") return undefined
  const uri = editor.document.uri
  const ctx: PromptContext = { uri: uri.toString(), name: vscode.workspace.asRelativePath(uri, false) }
  const range = selectionLineRange(editor.selection.start, editor.selection.end)
  if (range) ctx.selection = { text: editor.document.getText(editor.selection), startLine: range[0], endLine: range[1] }
  return ctx
}

/** Runs an ACP terminal auth method (e.g. `codg login`); resolves true on exit code 0. */
function runInTerminal(command: string, args: string[], env: Record<string, string>, cwd: string): Promise<boolean> {
  const terminal = vscode.window.createTerminal({ name: "codg login", shellPath: command, shellArgs: args, env, cwd })
  terminal.show()
  return new Promise((resolve) => {
    const listener = vscode.window.onDidCloseTerminal((t) => {
      if (t !== terminal) return
      listener.dispose()
      resolve(t.exitStatus?.code === 0)
    })
  })
}

function chatHtml(webview: vscode.Webview, dist: vscode.Uri): string {
  const nonce = Array.from({ length: 32 }, () => Math.floor(Math.random() * 36).toString(36)).join("")
  const script = webview.asWebviewUri(vscode.Uri.joinPath(dist, "webview.js"))
  const style = webview.asWebviewUri(vscode.Uri.joinPath(dist, "webview.css"))
  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src ${webview.cspSource}; img-src ${webview.cspSource} https: data:; script-src 'nonce-${nonce}';">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<link rel="stylesheet" href="${style}">
<title>Codg</title>
</head>
<body>
<main id="messages">
  <div id="welcome">
    <h2>Codg</h2>
    <p>Ask Codg to explain, edit, or build code in this workspace.</p>
    <p class="tips">Type <kbd>/</kbd> for commands, <kbd>@</kbd> to mention files.</p>
  </div>
</main>
<form id="composer">
  <div id="menu" role="listbox" hidden></div>
  <div class="box">
    <textarea id="input" rows="2" placeholder="Ask Codg anything — / for commands, @ for files"></textarea>
    <div class="toolbar">
      <button type="button" id="mention" class="round" title="Add a file (@)" aria-label="Add a file"><span data-icon="plus"></span></button>
      <button type="button" id="context" class="pill" aria-pressed="true" title="Attach the active file and selection (click to toggle)" hidden><span data-icon="cursor"></span><span id="context-label"></span></button>
      <span class="spacer"></span>
      <button type="button" id="history" class="round ghost" title="Chat history" aria-label="Chat history"><span data-icon="history"></span></button>
      <button id="send" class="send empty" type="submit" title="Send (Enter)" aria-label="Send"><span data-icon="arrowUp"></span></button>
    </div>
  </div>
  <div id="footer">
    <span id="cwd" class="meta" title="Workspace folder" hidden><span data-icon="laptop"></span><span class="text"></span></span>
    <button type="button" id="mode" class="chip" title="Agent" hidden><span data-icon="agent"></span><span class="text"></span><span data-icon="chevronDown" class="chev"></span></button>
    <button type="button" id="model" class="chip" title="Switch model" hidden><span data-icon="model"></span><span class="text"></span><span data-icon="chevronDown" class="chev"></span></button>
    <button type="button" id="effort" class="chip" title="Reasoning effort" hidden><span class="text"></span><span data-icon="chevronDown" class="chev"></span></button>
    <button type="button" id="approval" class="chip" title="Tool approval: Ask, Yolo (work) or Yolo (full)" hidden><span class="glyph" data-icon="shield"></span><span class="text"></span><span data-icon="chevronDown" class="chev"></span></button>
    <span class="spacer"></span>
    <span id="usage" class="meta"></span>
  </div>
</form>
<script nonce="${nonce}" src="${script}"></script>
</body>
</html>`
}
