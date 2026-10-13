// Minimal Agent Client Protocol (ACP) client: newline-delimited
// JSON-RPC 2.0 over the stdio of `codg acp`. Kept free of any `vscode`
// import so it can be unit-tested with plain streams.

import type { Readable, Writable } from "node:stream"

export const PROTOCOL_VERSION = 1
// ACP error code returned by session/new when no provider is configured.
export const AUTH_REQUIRED = -32000
export const METHOD_NOT_FOUND = -32601

export class RpcError extends Error {
  constructor(
    readonly code: number,
    message: string,
    readonly data?: unknown,
  ) {
    super(message)
  }
}

type Pending = { resolve: (v: any) => void; reject: (e: Error) => void }

export class AcpConnection {
  private nextId = 1
  private pending = new Map<number, Pending>()
  private buf = ""
  private closed?: Error

  /** Agent → client notifications (e.g. session/update). */
  onNotification?: (method: string, params: any) => void
  /** Agent → client requests (e.g. session/request_permission). Throw RpcError to fail. */
  onRequest?: (method: string, params: any) => Promise<unknown>
  /** Fired once when the agent stream ends. */
  onClose?: (err: Error) => void

  constructor(
    input: Readable,
    private readonly output: Writable,
  ) {
    input.setEncoding("utf8")
    input.on("data", (chunk: string) => this.read(chunk))
    input.on("end", () => this.close(new Error("codg agent exited")))
    input.on("error", (err) => this.close(err))
  }

  request<T = any>(method: string, params: unknown): Promise<T> {
    if (this.closed) return Promise.reject(this.closed)
    const id = this.nextId++
    return new Promise<T>((resolve, reject) => {
      this.pending.set(id, { resolve, reject })
      this.write({ jsonrpc: "2.0", id, method, params })
    })
  }

  notify(method: string, params: unknown) {
    if (!this.closed) this.write({ jsonrpc: "2.0", method, params })
  }

  /** Rejects all in-flight requests; later calls fail with err. */
  close(err: Error) {
    if (this.closed) return
    this.closed = err
    for (const p of this.pending.values()) p.reject(err)
    this.pending.clear()
    this.onClose?.(err)
  }

  private write(msg: unknown) {
    this.output.write(JSON.stringify(msg) + "\n")
  }

  private read(chunk: string) {
    this.buf += chunk
    let i: number
    while ((i = this.buf.indexOf("\n")) >= 0) {
      const line = this.buf.slice(0, i).trim()
      this.buf = this.buf.slice(i + 1)
      if (!line) continue
      let msg: any
      try {
        msg = JSON.parse(line)
      } catch {
        continue // agents may log non-JSON noise; ignore it
      }
      this.dispatch(msg)
    }
  }

  private dispatch(msg: any) {
    if (msg.method !== undefined && msg.id !== undefined) {
      void this.answer(msg.id, msg.method, msg.params)
      return
    }
    if (msg.method !== undefined) {
      this.onNotification?.(msg.method, msg.params)
      return
    }
    const p = this.pending.get(msg.id)
    if (!p) return
    this.pending.delete(msg.id)
    if (msg.error) p.reject(new RpcError(msg.error.code, msg.error.message, msg.error.data))
    else p.resolve(msg.result)
  }

  private async answer(id: number | string, method: string, params: unknown) {
    try {
      if (!this.onRequest) throw new RpcError(METHOD_NOT_FOUND, `method not found: ${method}`)
      const result = await this.onRequest(method, params)
      this.write({ jsonrpc: "2.0", id, result: result ?? null })
    } catch (err) {
      const code = err instanceof RpcError ? err.code : -32603
      this.write({ jsonrpc: "2.0", id, error: { code, message: (err as Error).message } })
    }
  }
}
