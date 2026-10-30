#!/usr/bin/env bash
# Build and package the Codg desktop app (WAILS v3 shell) for ONE platform.
#
#
# Host requirements:
#   darwin : Xcode CLT; create-dmg optional (hdiutil fallback)
#   windows: NSIS (choco install nsis); bash (git-bash)
#   linux  : gcc, libgtk-3-dev, libwebkit2gtk-4.1-dev
#   all    : Go, node/npm, Wails v3 CLI (`wails3`):
#            go install github.com/wailsapp/wails/v3/cmd/wails3@latest
set -euo pipefail

PLATFORM="${1:?usage: desktop-release.sh <os/arch> [version]}"
VERSION="${2:-dev}"
VERSION="${VERSION#v}"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] || {
	echo "invalid desktop version: $VERSION" >&2
	exit 1
}

os="${PLATFORM%/*}"
arch="${PLATFORM#*/}"

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APPNAME="codg-desktop"            # APP_NAME in ui/desktop/wails/Taskfile.yml
BUNDLE="Codg" # macOS bundle dir name (Codg.app); the binary stays $APPNAME
WAILS_DIR="$ROOT/ui/desktop/wails" # Wails v3 shell
PUB_DIR="$ROOT/ui/desktop/pub"     # shared module: backend launcher + embedded UI
WEB_DIR="$ROOT/ui/web"
BIN_DIR="$WAILS_DIR/bin" # BIN_DIR in ui/desktop/wails/Taskfile.yml
NSIS_DIR="$WAILS_DIR/build/windows/nsis"
DIST="$ROOT/dist"

# Release version, injected into the SHARED pub module
EXTRA_LDFLAGS="-X github.com/vcaesar/codg/ui/desktop/pub.AppVersion=$VERSION"

# CODG_MAIN_PKG: import path / dir of the codg CLI main package
CODG_MAIN_PKG="${CODG_MAIN_PKG:-./}"

command -v wails3 >/dev/null || {
	echo "Wails v3 CLI not found. Install:" >&2
	echo "  go install github.com/wailsapp/wails/v3/cmd/wails3@latest" >&2
	exit 1
}

mkdir -p "$DIST"

codg_bin_name() { [ "$os" = windows ] && echo "codg.exe" || echo "codg"; }

# sync_web_ui fills ui/desktop/pub/frontend/dist
sync_web_ui() {
	local dest="$PUB_DIR/frontend/dist"
	if [ -f "$WEB_DIR/package.json" ]; then
		echo "==> building web UI from source ($WEB_DIR)"
		( cd "$WEB_DIR" && { npm ci || npm install; } && npm run build )
		local src="$WEB_DIR/dist"
		[ -s "$src/web/index.html" ] && src="$src/web"
		if [ ! -s "$src/index.html" ]; then
			echo "ERROR: ui/web build produced no index.html in $WEB_DIR/dist" >&2
			exit 1
		fi
		rm -rf "$dest"
		mkdir -p "$dest"
		cp -R "$src/." "$dest/"
		echo "==> synced web UI: $src -> $dest"
	else
		echo "==> no ui/web source; embedding $dest"
	fi

	local index="$dest/index.html"
	if [ ! -s "$index" ]; then
		echo "ERROR: no index.html at $index (//go:embed would fail)" >&2
		exit 1
	fi
	if grep -q 'CODG_PLACEHOLDER' "$index"; then
		echo "ERROR: $index is the stage-dist placeholder; a release must embed" \
			"a real web UI (add ui/web source or commit pub/frontend/dist)" >&2
		exit 1
	fi
	# Every absolute asset index.html references must be embedded too, or the
	# WebView loads a blank page (e.g. index.html committed without assets/).
	local ref
	for ref in $(grep -oE '(src|href)="/[^"#?]+"' "$index" | sed -E 's/^[a-z]+="\///; s/"$//'); do
		if [ ! -f "$dest/$ref" ]; then
			echo "ERROR: $index references missing $dest/$ref" >&2
			exit 1
		fi
	done
}

# build_codg makes the standalone codg CLI for the target platform available
# in dist/ so it can be bundled next to the shell.
build_codg() {
	local out="$DIST/codg-${os}-${arch}"
	[ "$os" = windows ] && out="${out}.exe"
	# Never let a leftover from an earlier run masquerade as a fresh build:
	# the release workflow uploads dist/codg-<os>-<arch> as a published asset.
	rm -f "$out"

	if [ -n "${CODG_PREBUILT_BIN:-}" ] && [ -f "${CODG_PREBUILT_BIN}" ]; then
		cp "$CODG_PREBUILT_BIN" "$out"
		# CI artifacts routinely lose the exec bit; the app must be able to
		# spawn this binary.
		chmod +x "$out" 2>/dev/null || true
		echo "==> using prebuilt codg: $CODG_PREBUILT_BIN" >&2
		echo "$out"
		return 0
	fi

	echo "==> building codg CLI ($CODG_MAIN_PKG -> $out)" >&2
	local log
	if log=$( cd "$ROOT" && GOOS="$os" GOARCH="$arch" CGO_ENABLED=1 \
		go build -trimpath -ldflags "-w -s" -o "$out" "$CODG_MAIN_PKG" 2>&1 ); then
		chmod +x "$out" 2>/dev/null || true
		echo "$out"
		return 0
	fi

	if [ "${CODG_REQUIRE_BUNDLE:-0}" = "1" ]; then
		echo "ERROR: codg binary unavailable but CODG_REQUIRE_BUNDLE=1 set." >&2
		echo "       Provide CODG_PREBUILT_BIN or a buildable CODG_MAIN_PKG." >&2
		echo "$log" >&2
		exit 1
	fi
	echo "==> codg CLI unavailable here; shipped app will resolve codg from" \
		"PATH/CODG_BIN at runtime" >&2
	echo ""
}

# bundle_codg copies the built codg binary to <dir> as a sibling of the
# shell executable
bundle_codg() {
	local built="$1" dir="$2" name
	name="$(codg_bin_name)"
	if [ -z "$built" ]; then
		if [ "${CODG_REQUIRE_BUNDLE:-0}" = "1" ]; then
			echo "ERROR: CODG_REQUIRE_BUNDLE=1 but no codg binary to bundle into $dir" >&2
			exit 1
		fi
		echo "==> no codg binary to bundle into $dir (app will resolve codg from PATH/CODG_BIN)" >&2
		return 0
	fi
	if [ ! -d "$dir" ]; then
		echo "ERROR: codg bundle target dir does not exist: $dir" >&2
		exit 1
	fi
	cp "$built" "$dir/$name"
	chmod +x "$dir/$name" 2>/dev/null || true
	if [ ! -s "$dir/$name" ]; then
		echo "ERROR: failed to bundle codg into $dir/$name" >&2
		exit 1
	fi
	echo "==> bundled codg into $dir/$name"
}

# wails_task runs a task from ui/desktop/wails/Taskfile.yml with the release
# version and target arch.
wails_task() {
	local task="$1"
	echo "==> wails3 task $task (ARCH=$arch, version $VERSION)"
	( cd "$WAILS_DIR" && wails3 task "$task" ARCH="$arch" \
		EXTRA_TAGS="${EXTRA_TAGS:-}" EXTRA_LDFLAGS="$EXTRA_LDFLAGS" )
}

sync_web_ui
CODG_BUILT="$(build_codg)"

# Package metadata (build/config.yml, build/darwin/Info.plist,
# build/windows/info.json, build/linux/nfpm/nfpm.yaml)
case "$os" in
darwin)
	[ "$(uname -s)" = Darwin ] || { echo "darwin builds need a macOS host" >&2; exit 1; }
	# darwin:package builds the binary and assembles bin/Codg.app
	# (icons + Info.plist + ad-hoc codesign).
	rm -rf "$BIN_DIR/${BUNDLE}.app"
	wails_task darwin:package

	app="$BIN_DIR/${BUNDLE}.app"
	[ -x "$app/Contents/MacOS/$APPNAME" ] || { echo "no app bundle at $app" >&2; exit 1; }

	# Bundle codg next to the shell binary, then re-sign so the added sibling
	# is covered by the (ad-hoc) signature.
	bundle_codg "$CODG_BUILT" "$app/Contents/MacOS"
	codesign --force --deep --sign - "$app" 2>/dev/null || \
		echo "==> codesign skipped/failed (ad-hoc)" >&2

	# Updater/CLI-friendly zip of the bundle (ditto preserves the
	# signature; plain zip can corrupt it).
	ditto -c -k --keepParent "$app" "$DIST/${APPNAME}-darwin-${arch}.zip"

	# Drag-to-Applications dmg. create-dmg gives the pretty layout;
	# hdiutil (always present) is the fallback.
	dmg="$DIST/${APPNAME}-darwin-${arch}.dmg"
	rm -f "$dmg"
	staging=$(mktemp -d)
	cp -R "$app" "$staging/${BUNDLE}.app"
	if command -v create-dmg >/dev/null; then
		# --skip-jenkins: the Finder-prettifying AppleScript needs a GUI
		# session ("Not authorized to send Apple events to Finder" on CI);
		# skipping it keeps the dmg deterministic on headless runners.
		create-dmg \
			--volname "Codg" \
			--window-size 540 380 \
			--icon-size 110 \
			--icon "${BUNDLE}.app" 150 190 \
			--app-drop-link 390 190 \
			--no-internet-enable \
			--skip-jenkins \
			"$dmg" "$staging" || true
	fi
	# create-dmg can abort mid-way (e.g. AppleScript) and leave its
	# intermediate read-write image behind next to the output;
	rm -f "$DIST"/rw.*."${APPNAME}-darwin-${arch}.dmg" "$DIST"/rw.*.dmg
	if [ ! -f "$dmg" ]; then
		ln -s /Applications "$staging/Applications"
		hdiutil create -volname "Codg" -srcfolder "$staging" \
			-ov -format UDZO "$dmg"
	fi
	rm -rf "$staging"
	;;
windows)
	# The NSIS project installs codg.exe from its own dir when present (see
	# build/windows/nsis/project.nsi)
	rm -f "$NSIS_DIR/codg.exe" "$BIN_DIR/${APPNAME}.exe" "$BIN_DIR"/*-installer.exe
	bundle_codg "$CODG_BUILT" "$NSIS_DIR"
	# windows:package builds bin/codg-desktop.exe and runs makensis, which
	# writes bin/<project>-<arch>-installer.exe.
	wails_task windows:package

	installer=$(ls "$BIN_DIR"/*-"${arch}"-installer.exe 2>/dev/null | head -n1 || true)
	[ -n "$installer" ] || installer=$(ls "$BIN_DIR"/*-installer.exe 2>/dev/null | head -n1 || true)
	[ -n "$installer" ] || { echo "no NSIS installer found in $BIN_DIR/" >&2; exit 1; }
	cp "$installer" "$DIST/${APPNAME}-windows-${arch}-installer.exe"

	# Portable zip: the shell exe + bundled codg.exe sibling.
	portable="$BIN_DIR/${APPNAME}.exe"
	if [ -f "$portable" ]; then
		staging=$(mktemp -d)
		cp "$portable" "$staging/${APPNAME}.exe"
		bundle_codg "$CODG_BUILT" "$staging"
		(cd "$staging" && powershell.exe -NoProfile -Command \
			"Compress-Archive -Force -Path '*' -DestinationPath '$(cygpath -w "$DIST/${APPNAME}-windows-${arch}.zip")'")
		rm -rf "$staging"
	else
		echo "no portable exe at $portable" >&2
		exit 1
	fi
	;;
linux)
	# build/linux/Taskfile.yml routes to the Docker cross image whenever the
	# host is not Linux or the arch differs
	if [ "$(go env GOHOSTOS)" != linux ] || [ "$arch" != "$(go env GOHOSTARCH)" ]; then
		echo "ERROR: linux/$arch must be built on a native linux/$arch host;" \
			"cross builds route through Docker and drop EXTRA_LDFLAGS" >&2
		exit 1
	fi
	# webkit2_41 matches the libwebkit2gtk-4.1-dev packages the release
	# workflow installs (the v2 script passes the same tag).
	EXTRA_TAGS=webkit2_41
	# linux:build emits bin/codg-desktop.
	rm -f "$BIN_DIR/${APPNAME}"
	wails_task linux:build

	linux_binary="$BIN_DIR/${APPNAME}"
	if [ -f "$linux_binary" ]; then
		staging=$(mktemp -d)
		cp "$linux_binary" "$staging/$APPNAME"
		mkdir -p "$staging/share/icons/hicolor/1024x1024/apps" "$staging/share/applications"
		cp build/appicon.png "$staging/share/icons/hicolor/1024x1024/apps/$APPNAME.png"
		cp "build/linux/$APPNAME.desktop" "$staging/share/applications/"
		bundle_codg "$CODG_BUILT" "$staging"
		tar -czf "$DIST/${APPNAME}-linux-${arch}.tar.gz" -C "$staging" .
		rm -rf "$staging"
	else
		echo "no Linux binary found at $linux_binary" >&2
		exit 1
	fi
	;;
*)
	echo "unsupported os: $os (want darwin|windows|linux)" >&2
	exit 1
	;;
esac

echo "==> $VERSION packaged into dist/:"
ls -la "$DIST" | grep -iE "codg" || true
