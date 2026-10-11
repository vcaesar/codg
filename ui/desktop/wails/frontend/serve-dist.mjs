// serve-dist.mjs — minimal static dev server for `wails3 dev`.
//
// Dependency-free (node:http only), cross-platform.
import { createServer } from "node:http";
import { existsSync, statSync, createReadStream } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, extname, join, normalize } from "node:path";

const here = dirname(fileURLToPath(import.meta.url));
// Serve the shared staged build embedded by the pub package.
const dist = join(here, "..", "..", "pub", "frontend", "dist");

// --port N (passed by the Taskfile as `npm run dev -- --port 5176`);
// --strictPort is accepted and ignored (we always bind exactly).
const argv = process.argv.slice(2);
const portIdx = argv.indexOf("--port");
const port = portIdx >= 0 ? Number(argv[portIdx + 1]) : 5176;

const types = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".mjs": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".gif": "image/gif",
  ".ico": "image/x-icon",
  ".woff": "font/woff",
  ".woff2": "font/woff2",
  ".ttf": "font/ttf",
  ".map": "application/json",
  ".txt": "text/plain; charset=utf-8",
  ".wasm": "application/wasm",
  ".webmanifest": "application/manifest+json",
};

const server = createServer((req, res) => {
  // Resolve the path inside dist; normalize() blocks ../ traversal.
  let path = decodeURIComponent(new URL(req.url, "http://x").pathname);
  if (path.endsWith("/")) path += "index.html";
  const file = normalize(join(dist, path));
  if (!file.startsWith(dist) || !existsSync(file) || !statSync(file).isFile()) {
    // Plain 404 — the wails asset middleware replays these on the backend.
    res.writeHead(404, { "content-type": "text/plain" });
    res.end("not found");
    return;
  }
  res.writeHead(200, {
    "content-type": types[extname(file)] ?? "application/octet-stream",
    "cache-control": "no-store", // dev: always fresh
  });
  createReadStream(file).pipe(res);
});

server.listen(port, "127.0.0.1", () => {
  console.log(`serving ${dist} at http://localhost:${port} (dev static server)`);
});
