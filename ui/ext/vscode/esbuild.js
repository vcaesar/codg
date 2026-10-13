// Bundles the extension for the VS Code extension host.
// Usage: node esbuild.js [--watch] [--production]
const esbuild = require("esbuild")

const watch = process.argv.includes("--watch")
const production = process.argv.includes("--production")

/** @type {import("esbuild").BuildOptions[]} */
const builds = [
  {
    // Extension host.
    entryPoints: ["src/extension.ts"],
    outfile: "dist/extension.js",
    external: ["vscode"],
    format: "cjs",
    platform: "node",
    target: "node18",
  },
  {
    // Chat webview script + CSS (dist/webview.js, dist/webview.css).
    entryPoints: ["src/webview/main.ts"],
    outfile: "dist/webview.js",
    format: "iife",
    platform: "browser",
    target: "es2022",
  },
].map((b) => ({ ...b, bundle: true, sourcemap: !production, minify: production }))

async function main() {
  if (watch) {
    for (const options of builds) await (await esbuild.context(options)).watch()
    console.log("watching...")
    return
  }
  await Promise.all(builds.map((options) => esbuild.build(options)))
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
