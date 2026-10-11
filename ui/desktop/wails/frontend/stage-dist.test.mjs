// Regression tests for both shells' stage-dist.mjs: a newer pub frontend
// must not be overwritten by an older ui/web/dist. Run: node --test
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  utimesSync,
  writeFileSync,
} from "node:fs";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const here = dirname(fileURLToPath(import.meta.url));
const desktop = join(here, "..", "..");
const tmpRoot = join(desktop, "..", "..", "test", "tmp");

// writeBuild writes a minimal real web build tagged with name and mtime.
function writeBuild(dir, name, mtime) {
  mkdirSync(join(dir, "assets"), { recursive: true });
  writeFileSync(join(dir, "assets", `${name}.js`), name);
  writeFileSync(
    join(dir, "index.html"),
    `<!doctype html><script src="/assets/${name}.js"></script>`,
  );
  utimesSync(join(dir, "index.html"), mtime, mtime);
}

// runStage copies shell's stage-dist.mjs into a fresh fixture tree, seeds it
// via setup(webDist, pubDist), runs it, and returns the staged index.html.
function runStage(shell, setup) {
  mkdirSync(tmpRoot, { recursive: true });
  const root = mkdtempSync(join(tmpRoot, "stage-"));
  const frontend = join(root, "ui", "desktop", shell, "frontend");
  const webDist = join(root, "ui", "web", "dist", "web");
  const pubDist = join(root, "ui", "desktop", "pub", "frontend", "dist");
  mkdirSync(frontend, { recursive: true });
  copyFileSync(
    join(desktop, shell, "frontend", "stage-dist.mjs"),
    join(frontend, "stage-dist.mjs"),
  );
  setup(webDist, pubDist);
  const r = spawnSync(process.execPath, ["stage-dist.mjs"], {
    cwd: frontend,
    encoding: "utf8",
  });
  assert.equal(r.status, 0, r.stderr);
  return readFileSync(join(pubDist, "index.html"), "utf8");
}

const old = new Date("2026-07-06T00:00:00Z");
const fresh = new Date("2026-10-10T00:00:00Z");

for (const shell of ["wails", "wails2"]) {
  test(`${shell}: keeps newer pub frontend over older ui/web/dist`, () => {
    const html = runStage(shell, (web, pub) => {
      writeBuild(web, "old", old);
      writeBuild(pub, "new", fresh);
    });
    assert.match(html, /assets\/new\.js/);
  });

  test(`${shell}: stages newer ui/web/dist over older pub frontend`, () => {
    const html = runStage(shell, (web, pub) => {
      writeBuild(web, "new", fresh);
      writeBuild(pub, "old", old);
    });
    assert.match(html, /assets\/new\.js/);
  });

  test(`${shell}: replaces placeholder with ui/web/dist`, () => {
    const html = runStage(shell, (web, pub) => {
      writeBuild(web, "web", old);
      mkdirSync(pub, { recursive: true });
      writeFileSync(join(pub, "index.html"), "<!-- CODG_PLACEHOLDER -->");
    });
    assert.match(html, /assets\/web\.js/);
  });

  test(`${shell}: stages into empty pub frontend`, () => {
    const html = runStage(shell, (web) => writeBuild(web, "web", old));
    assert.match(html, /assets\/web\.js/);
  });
}
