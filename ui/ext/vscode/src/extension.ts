// Codg editor extension: launches the codg TUI in an integrated
// terminal beside the editor and shares file/selection context with
// it via @file#Lx-y mentions typed into the TUI prompt.
//
// Works in VS Code and its forks (Cursor, Windsurf, VSCodium); the
// host is reported to the CLI through the CODG_CALLER env var.

import * as vscode from "vscode"
import { CHAT_VIEW_ID, ChatViewProvider, openChatPanel } from "./chat"
import { buildLaunchCommand, formatFileReference, resolveCaller, selectionLineRange } from "./lib"

const TERMINAL_NAME = "codg"
// How long to wait for shell integration before falling back to
// plain sendText, which can race a slow shell startup.
const SHELL_INTEGRATION_TIMEOUT_MS = 3000
// Fallback delays used when shell integration is unavailable.
const EXECUTION_START_TIMEOUT_MS = 2000
const SEED_DELAY_MS = 500

// Working directory of each codg terminal we launched, so mentions are
// built relative to what codg actually sees.
const terminalCwd = new WeakMap<vscode.Terminal, vscode.Uri>()

export function activate(context: vscode.ExtensionContext) {
  const log = vscode.window.createOutputChannel("Codg")
  const chat = new ChatViewProvider(context.extensionUri, log, context.workspaceState)
  context.subscriptions.push(
    log,
    chat,
    vscode.window.registerWebviewViewProvider(CHAT_VIEW_ID, chat, {
      webviewOptions: { retainContextWhenHidden: true },
    }),
    vscode.commands.registerCommand("codg.openChat", () =>
      vscode.commands.executeCommand(`${CHAT_VIEW_ID}.focus`),
    ),
    vscode.commands.registerCommand("codg.newChat", async () => {
      await vscode.commands.executeCommand(`${CHAT_VIEW_ID}.focus`)
      chat.newChat()
    }),
    vscode.commands.registerCommand("codg.openChatInEditor", () =>
      openChatPanel(context.extensionUri, log, context.workspaceState),
    ),
    vscode.commands.registerCommand("codg.history", async () => {
      await vscode.commands.executeCommand(`${CHAT_VIEW_ID}.focus`)
      await chat.showHistory()
    }),
    vscode.commands.registerCommand("codg.addToChat", async () => {
      const ref = getActiveFileReference(vscode.workspace.workspaceFolders?.[0]?.uri)
      if (!ref) return
      await vscode.commands.executeCommand(`${CHAT_VIEW_ID}.focus`)
      chat.insert(`${ref} `)
    }),

    vscode.commands.registerCommand("codg.openTerminal", () => {
      // Focus an existing codg terminal if one is already running.
      const existing = findCodgTerminal()
      if (existing) {
        existing.show()
        return
      }
      openTerminal(context)
    }),

    vscode.commands.registerCommand("codg.openNewTerminal", () => {
      openTerminal(context)
    }),

    vscode.commands.registerCommand("codg.addFilepathToTerminal", () => {
      const terminal = findCodgTerminal()
      if (!terminal) {
        vscode.window.showInformationMessage("No codg terminal is open. Run “Codg: Open Codg” first.")
        return
      }
      const fileRef = getActiveFileReference(terminalCwd.get(terminal))
      if (!fileRef) return
      // Types the mention into the TUI prompt without submitting it.
      terminal.sendText(`${fileRef} `, false)
      terminal.show()
    }),
  )
}

/** Returns the active codg terminal, or any codg terminal still running. */
function findCodgTerminal(): vscode.Terminal | undefined {
  const isLive = (t: vscode.Terminal) => t.name === TERMINAL_NAME && t.exitStatus === undefined
  const active = vscode.window.activeTerminal
  if (active && isLive(active)) return active
  return vscode.window.terminals.find(isLive)
}

function openTerminal(context: vscode.ExtensionContext) {
  const config = vscode.workspace.getConfiguration("codg")
  const command = config.get<string>("command", "codg")
  const args = config.get<string[]>("args", [])

  const editor = vscode.window.activeTextEditor
  const cwd =
    (editor && vscode.workspace.getWorkspaceFolder(editor.document.uri))?.uri ??
    vscode.workspace.workspaceFolders?.[0]?.uri

  const terminal = vscode.window.createTerminal({
    name: TERMINAL_NAME,
    iconPath: {
      light: vscode.Uri.file(context.asAbsolutePath("images/button-dark.svg")),
      dark: vscode.Uri.file(context.asAbsolutePath("images/button-light.svg")),
    },
    location: { viewColumn: vscode.ViewColumn.Beside, preserveFocus: false },
    cwd,
    env: {
      CODG_CALLER: resolveCaller(vscode.env.appName),
    },
  })
  if (cwd) terminalCwd.set(terminal, cwd)
  terminal.show()

  // Seed the prompt with the active file so the agent starts with
  // context. The text is buffered by the PTY and lands in the TUI
  // input once codg starts reading stdin.
  const fileRef = getActiveFileReference(cwd)
  launch(terminal, command, args, () => {
    if (fileRef) terminal.sendText(`${fileRef} `, false)
  })
}

/**
 * Runs codg once the shell is ready. Shell integration lets VS Code
 * wait for the prompt (and quote arguments for the shell) instead of
 * racing shell init; without it we fall back to sendText and timers.
 * onStart is invoked once codg is executing.
 */
function launch(terminal: vscode.Terminal, command: string, args: string[], onStart: () => void) {
  const execute = (shell: vscode.TerminalShellIntegration) => {
    waitFor(vscode.window.onDidStartTerminalShellExecution, terminal, EXECUTION_START_TIMEOUT_MS, onStart, onStart)
    shell.executeCommand(command, args)
  }

  if (terminal.shellIntegration) {
    execute(terminal.shellIntegration)
    return
  }
  waitFor(
    vscode.window.onDidChangeTerminalShellIntegration,
    terminal,
    SHELL_INTEGRATION_TIMEOUT_MS,
    (e) => execute(e.shellIntegration),
    () => {
      terminal.sendText(buildLaunchCommand(command, args))
      setTimeout(onStart, SEED_DELAY_MS)
    },
  )
}

/** Waits once for a terminal event, calling onTimeout if it never fires. */
function waitFor<T extends { terminal: vscode.Terminal }>(
  event: vscode.Event<T>,
  terminal: vscode.Terminal,
  timeoutMs: number,
  onEvent: (e: T) => void,
  onTimeout: () => void,
) {
  const timer = setTimeout(() => {
    listener.dispose()
    onTimeout()
  }, timeoutMs)
  const listener = event((e) => {
    if (e.terminal !== terminal) return
    clearTimeout(timer)
    listener.dispose()
    onEvent(e)
  })
}

/**
 * Returns an @-mention for the active editor's file, including the
 * selected line range when the selection is non-empty. The path is
 * relative when the file lives under the terminal's cwd, absolute
 * otherwise (other workspace roots, files outside the workspace).
 */
function getActiveFileReference(cwd?: vscode.Uri): string | undefined {
  const editor = vscode.window.activeTextEditor
  if (!editor || editor.document.uri.scheme !== "file") return undefined

  const uri = editor.document.uri
  const folder = vscode.workspace.getWorkspaceFolder(uri)
  const path =
    folder && (!cwd || folder.uri.toString() === cwd.toString())
      ? vscode.workspace.asRelativePath(uri, false)
      : uri.fsPath

  const range = selectionLineRange(editor.selection.start, editor.selection.end)
  return range ? formatFileReference(path, range[0], range[1]) : formatFileReference(path)
}

export function deactivate() {}
