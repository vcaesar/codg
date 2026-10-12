#!/usr/bin/env bash
# Build and package the Codg desktop app for ONE platform.
#
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

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APPNAME="codg-desktop" # outputfilename in ui/desktop/wails2/wails.json
BUNDLE="Codg" # macOS bundle dir name (Codg.app); the binary stays $APPNAME
WAILS_DIR="$ROOT/ui/desktop/wails2"
PUB_DIR="$ROOT/ui/desktop/pub" # shared module: backend launcher + embedded UI
WEB_DIR="$ROOT/ui/web"
DIST="$ROOT/dist"

# Release ldflags. appMode=pro disables dev tooling; 
LDFLAGS="-X main.appMode=pro -X github.com/vcaesar/codg/ui/desktop/pub.AppVersion=$VERSION"

# CODG_MAIN_PKG: import path / dir of the codg CLI main package, built and
# bundled next to the shell.
CODG_MAIN_PKG="${CODG_MAIN_PKG:-./}"

command -v wails >/dev/null || {
	echo "Wails v2 CLI not found. Install:" >&2
	echo "  go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0" >&2
	exit 1
}

mkdir -p "$DIST"

codg_bin_name() { [ "$os" = windows ] && echo "codg.exe" || echo "codg"; }

# sync_web_ui fills ui/desktop/pub/frontend/dist, the dir pub/assets.go embeds
# into the shell binary. When ui/web has source it is built fresh and replaces
# pub/frontend/dist; otherwise the checked-in pub/frontend/dist ships as-is.
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
# in dist/ so it can be bundled next to the shell. Resolution:
# Prints the resulting binary path on stdout when available, empty otherwise.
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

sync_web_ui
CODG_BUILT="$(build_codg)"

# # Keep native package metadata aligned with the release version. Restore the
cd "$WAILS_DIR"

case "$os" in
darwin)
	[ "$(uname -s)" = Darwin ] || { echo "darwin builds need a macOS host" >&2; exit 1; }
	# Wails v2 packages the native .app in build/bin. -s skips frontend:build:
	# sync_web_ui already filled the pub/frontend/dist embed.
	wails build -clean -s -platform "darwin/$arch" -ldflags "$LDFLAGS"

	app="build/bin/${BUNDLE}.app"
	[ -x "$app/Contents/MacOS/$APPNAME" ] || { echo "no app bundle at $app" >&2; exit 1; }

	# Bundle codg next to the shell binary, then (re-)sign so the added
	# sibling is covered by the (ad-hoc) signature.
	bundle_codg "$CODG_BUILT" "$app/Contents/MacOS"
	codesign --force --deep --sign - "$app" 2>/dev/null || \
		echo "==> codesign skipped/failed (ad-hoc)" >&2

	# Updater/CLI-friendly zip of the bundle (ditto preserves the
	# signature; plain zip can corrupt it).
	ditto -c -k --keepParent "$app" "$DIST/${APPNAME}-darwin-${arch}.zip"

	# Drag-to-Applications dmg. create-dmg gives the pretty layout;
	# hdiutil (always present) is the fallback. create-dmg may exit
	# nonzero while still writing the image, so gate on the file.
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
	# intermediate read-write image behind next to the output; drop it so
	# dist/ holds only publishable artifacts.
	rm -f "$DIST"/rw.*."${APPNAME}-darwin-${arch}.dmg" "$DIST"/rw.*.dmg
	if [ ! -f "$dmg" ]; then
		ln -s /Applications "$staging/Applications"
		hdiutil create -volname "Codg" -srcfolder "$staging" \
			-ov -format UDZO "$dmg"
	fi
	rm -rf "$staging"
	;;
windows)
	# Wails v2 builds the executable and NSIS installer in build/bin. Stage
	# codg.exe into the custom NSIS project
	rm -f "$WAILS_DIR/build/windows/installer/codg.exe"
	bundle_codg "$CODG_BUILT" "$WAILS_DIR/build/windows/installer"
	wails build -clean -s -platform "windows/$arch" -nsis -ldflags "$LDFLAGS"

	installer=$(ls build/bin/*-installer.exe build/bin/*installer*.exe 2>/dev/null | head -n1 || true)
	[ -n "$installer" ] || { echo "no NSIS installer found in build/bin/" >&2; exit 1; }
	cp "$installer" "$DIST/${APPNAME}-windows-${arch}-installer.exe"

	# Portable zip: the shell exe + bundled codg.exe sibling.
	portable=$(find build/bin -maxdepth 1 -type f -name "*.exe" ! -name "*installer*" | head -n1 || true)
	if [ -n "$portable" ]; then
		staging=$(mktemp -d)
		cp "$portable" "$staging/${APPNAME}.exe"
		bundle_codg "$CODG_BUILT" "$staging"
		(cd "$staging" && powershell.exe -NoProfile -Command \
			"Compress-Archive -Force -Path '*' -DestinationPath '$(cygpath -w "$DIST/${APPNAME}-windows-${arch}.zip")'")
		rm -rf "$staging"
	fi
	;;
linux)
	# Wails v2 emits a native binary in build/bin. Package it with the codg
	# backend sibling as a portable tarball.
	wails build -clean -s -platform "linux/$arch" -tags webkit2_41 \
		-o "$APPNAME" -ldflags "$LDFLAGS"

	linux_binary="build/bin/${APPNAME}"
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
