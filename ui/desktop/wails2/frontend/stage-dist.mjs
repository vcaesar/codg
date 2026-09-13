// stage-dist.mjs — populate the shared pub/frontend/dist with the web UI
// for Wails to embed.
//
// Cross-platform: pure Node, no shell-specific commands.
import {
  cpSync,
  existsSync,
  readFileSync,
  rmSync,
  mkdirSync,
  writeFileSync,
} from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { spawnSync } from "node:child_process";

const here = dirname(fileURLToPath(import.meta.url));
// The embed lives in the shared pub package: frontend -> wails2 -> desktop,
// then pub/frontend/dist.
const dest = join(here, "..", "..", "pub", "frontend", "dist");
// frontend -> wails2 -> desktop -> ui, then web.
const webDir = join(here, "..", "..", "..", "web");
// Prefer the nested web/ dir (vite emits dist/web) and fall back to flat dist/.
const candidates = [join(webDir, "dist", "web"), join(webDir, "dist")];

// Marker embedded in placeholder index.html so isRealBuild() can tell a
// placeholder apart from a genuine web build.
const PLACEHOLDER_MARK = "CODG_PLACEHOLDER";

// isRealBuild reports whether dir already holds a non-empty genuine web build
// whose absolute asset references resolve inside that directory.
function isRealBuild(dir) {
  const index = join(dir, "index.html");
  if (!existsSync(index)) return false;
  try {
    const html = readFileSync(index, "utf8");
    if (!html.trim() || html.includes(PLACEHOLDER_MARK)) return false;
    const refs = [...html.matchAll(/(?:src|href)=["']\/([^"'#?]+)["']/g)];
    return refs.every((match) => existsSync(join(dir, match[1])));
  } catch {
    return false;
  }
}

function stage(src) {
  rmSync(dest, { recursive: true, force: true });
  mkdirSync(dest, { recursive: true });
  cpSync(src, dest, { recursive: true });
  console.log(`staged web UI: ${src} -> ${dest}`);
}

function writePlaceholder(reason) {
  // Never replace a real build with a placeholder — that would orphan the
  // staged JS/CSS and break the embed. Keep whatever real UI is already there.
  if (isRealBuild(dest)) {
    console.log(`kept existing real web UI in ${dest} (skipped placeholder: ${reason})`);
    return;
  }
  mkdirSync(dest, { recursive: true });
  // A tiny, dependency-free fallback. The real UI is served by `codg web`;
  // this only shows when no backend is reachable.
  writeFileSync(
    join(dest, "index.html"),
    `<!doctype html>
<!-- ${PLACEHOLDER_MARK}: fallback so \`//go:embed all:frontend/dist\` compiles.
     ${reason}. At runtime the asset proxy falls back to the codg backend when
     this placeholder is embedded. Overwritten when a real ui/web build is
     available. -->
<html>
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Codg Desktop</title>
    <style>
      html,body{height:100%;margin:0}
      body{display:flex;align-items:center;justify-content:center;
        font:14px/1.5 -apple-system,Segoe UI,Roboto,sans-serif;
        color:#e6e6f0;background:#1a1a2e;text-align:center}
      .box{max-width:30rem;padding:2rem}
      .spin{width:28px;height:28px;margin:0 auto 1rem;border-radius:50%;
        border:3px solid #3a3a5e;border-top-color:#8a7dff;
        animation:s 1s linear infinite}
      @keyframes s{to{transform:rotate(360deg)}}
      code{background:#2a2a44;padding:.1rem .35rem;border-radius:.25rem}
      small{color:#9a9ab0}
    </style>
  </head>
  <body>
    <div class="box">
      <div class="spin"></div>
      <p>Codg backend starting…</p>
      <small>If this persists, ensure the <code>codg</code> binary is on your
        <code>PATH</code> or set <code>CODG_BIN</code>.</small>
    </div>
    <script>
      // Reload once a backend answers on this origin (same-origin desktop mode).
      setInterval(function () {
        fetch("/health", { cache: "no-store" })
          .then(function (r) { if (r.ok) location.reload(); })
          .catch(function () {});
      }, 2000);
    </script>
  </body>
</html>
`
  );
  console.warn(`staged placeholder web UI (${reason})`);
}

// npm on Windows is npm.cmd; spawn through the shell so PATHEXT resolves it.
function run(cmd, args, cwd) {
  const r = spawnSync(cmd, args, { cwd, stdio: "inherit", shell: true });
  return r.status === 0;
}

// 1. Prebuilt assets already present.
const prebuilt = candidates.find((p) => isRealBuild(p));
if (prebuilt) {
  stage(prebuilt);
  process.exit(0);
}

// 2. Source present — build it, then stage.
if (existsSync(join(webDir, "package.json"))) {
  console.log(`no prebuilt web UI; building from source in ${webDir}`);
  const installed =
    existsSync(join(webDir, "node_modules")) ||
    run("npm", ["ci"], webDir) ||
    run("npm", ["install"], webDir);
  if (installed && run("npm", ["run", "build"], webDir)) {
    const built = candidates.find((p) => isRealBuild(p));
    if (built) {
      stage(built);
      process.exit(0);
    }
  }
  // Build attempted but produced nothing usable — don't fail the wails
  // build; keep any real dist, else fall through to a placeholder.
  writePlaceholder("ui/web build did not produce dist");
  process.exit(0);
}

// 3. No prebuilt assets and no source in this tree.
writePlaceholder("ui/web has no build output or source here");
process.exit(0);
