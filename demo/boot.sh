#!/usr/bin/env bash
# Codg installer — downloads the codg CLI from GitHub releases.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/vcaesar/codg/main/demo/boot.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/vcaesar/codg/main/demo/boot.sh | bash -s -- --version 2.0.2
#   ./boot.sh --binary /path/to/codg
set -euo pipefail

APP=codg
REPO=vcaesar/codg

MUTED='\033[0;2m'
RED='\033[0;31m'
ORANGE='\033[38;5;214m'
NC='\033[0m'

usage() {
    cat <<EOF
Codg Installer

Usage: boot.sh [options]

Options:
    -h, --help              Display this help message
    -v, --version <version> Install a specific version (e.g., 2.0.2)
    -b, --binary <path>     Install from a local binary instead of downloading
        --no-modify-path    Don't modify shell config files (.zshrc, .bashrc, etc.)

Examples:
    curl -fsSL https://raw.githubusercontent.com/${REPO}/main/demo/boot.sh | bash
    curl -fsSL https://raw.githubusercontent.com/${REPO}/main/demo/boot.sh | bash -s -- --version 2.0.2
    ./boot.sh --binary /path/to/codg
EOF
}

print_message() {
    local level=$1
    local message=$2
    local color="${NC}"
    case $level in
        error) color="${RED}" ;;
        warn)  color="${ORANGE}" ;;
    esac
    echo -e "${color}${message}${NC}"
}

requested_version=${VERSION:-}
no_modify_path=false
binary_path=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)
            usage
            exit 0
            ;;
        -v|--version)
            if [[ -n "${2:-}" ]]; then
                requested_version="$2"
                shift 2
            else
                print_message error "Error: --version requires a version argument"
                exit 1
            fi
            ;;
        -b|--binary)
            if [[ -n "${2:-}" ]]; then
                binary_path="$2"
                shift 2
            else
                print_message error "Error: --binary requires a path argument"
                exit 1
            fi
            ;;
        --no-modify-path)
            no_modify_path=true
            shift
            ;;
        *)
            print_message warn "Warning: Unknown option '$1'"
            shift
            ;;
    esac
done

INSTALL_DIR="$HOME/.${APP}/bin"
mkdir -p "$INSTALL_DIR"

detect_target() {
    local raw_os os raw_arch arch
    raw_os=$(uname -s)
    case "$raw_os" in
        Darwin*)               os="darwin" ;;
        Linux*)                os="linux" ;;
        MINGW*|MSYS*|CYGWIN*)  os="windows" ;;
        *)
            print_message error "Unsupported OS: $raw_os"
            exit 1
            ;;
    esac

    raw_arch=$(uname -m)
    case "$raw_arch" in
        x86_64|amd64)  arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *)
            print_message error "Unsupported architecture: $raw_arch"
            exit 1
            ;;
    esac

    # Honour Rosetta 2 so arm64 Macs get the native build.
    if [ "$os" = "darwin" ] && [ "$arch" = "amd64" ]; then
        local rosetta_flag
        rosetta_flag=$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)
        if [ "$rosetta_flag" = "1" ]; then
            arch="arm64"
        fi
    fi

    OS="$os"
    ARCH="$arch"
}

fetch_latest_version() {
    local tag
    tag=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' \
        | head -n1)
    if [ -z "$tag" ]; then
        print_message error "Failed to fetch latest version information"
        exit 1
    fi
    echo "$tag"
}

check_installed_version() {
    local version=$1
    if command -v "$APP" >/dev/null 2>&1; then
        local installed
        installed=$("$APP" --version 2>/dev/null | awk '{print $NF}' | tr -d 'v' || true)
        if [ -n "$installed" ] && [ "$installed" = "$version" ]; then
            print_message info "${MUTED}Version ${NC}${version}${MUTED} already installed${NC}"
            exit 0
        fi
        if [ -n "$installed" ]; then
            echo -e "${MUTED}Installed version: ${NC}${installed}${MUTED} → upgrading to ${NC}${version}"
        fi
    fi
}

download_and_install() {
    detect_target

    if [ -z "$requested_version" ]; then
        requested_version=$(fetch_latest_version)
    fi
    requested_version="${requested_version#v}"

    check_installed_version "$requested_version"

    if ! command -v unzip >/dev/null 2>&1; then
        print_message error "Error: 'unzip' is required but not installed."
        exit 1
    fi

    local filename="${APP}_${OS}_${ARCH}.zip"
    local url="https://github.com/${REPO}/releases/download/v${requested_version}/${filename}"

    # Verify the release exists.
    local http_status
    http_status=$(curl -sI -o /dev/null -w "%{http_code}" \
        "https://github.com/${REPO}/releases/tag/v${requested_version}")
    if [ "$http_status" = "404" ]; then
        print_message error "Error: Release v${requested_version} not found"
        echo -e "${MUTED}Available releases: https://github.com/${REPO}/releases${NC}"
        exit 1
    fi

    echo -e "\n${MUTED}Installing ${NC}${APP} ${MUTED}version: ${NC}${requested_version} ${MUTED}(${OS}/${ARCH})${NC}"

    local tmp_dir
    tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/${APP}_install_XXXXXX")
    trap 'rm -rf "$tmp_dir"' EXIT

    curl -# -fL -o "$tmp_dir/$filename" "$url"
    unzip -q "$tmp_dir/$filename" -d "$tmp_dir/extracted"

    local bin_name="$APP"
    [ "$OS" = "windows" ] && bin_name="${APP}.exe"

    local src
    src=$(command ls "$tmp_dir/extracted/$bin_name" 2>/dev/null || true)
    if [ -z "$src" ]; then
        # Fall back to searching one level deep (some archives nest into a folder).
        src=$(command find "$tmp_dir/extracted" -maxdepth 2 -type f -name "$bin_name" | head -n1)
    fi
    if [ -z "$src" ] || [ ! -f "$src" ]; then
        print_message error "Error: '${bin_name}' not found in downloaded archive"
        exit 1
    fi

    mv "$src" "$INSTALL_DIR/$bin_name"
    chmod 755 "$INSTALL_DIR/$bin_name"
}

install_from_binary() {
    if [ ! -f "$binary_path" ]; then
        print_message error "Error: Binary not found at ${binary_path}"
        exit 1
    fi
    echo -e "\n${MUTED}Installing ${NC}${APP} ${MUTED}from: ${NC}${binary_path}"
    cp "$binary_path" "$INSTALL_DIR/$APP"
    chmod 755 "$INSTALL_DIR/$APP"
}

add_to_path() {
    local config_file=$1
    local command=$2

    if grep -Fxq "$command" "$config_file" 2>/dev/null; then
        echo -e "${MUTED}Command already exists in ${NC}${config_file}${MUTED}, skipping.${NC}"
    elif [[ -w $config_file ]]; then
        {
            echo ""
            echo "# ${APP}"
            echo "$command"
        } >> "$config_file"
        echo -e "${MUTED}Added ${NC}${APP}${MUTED} to \$PATH in ${NC}${config_file}"
    else
        print_message warn "Manually add this line to ${config_file} (or similar):"
        echo -e "  $command"
    fi
}

configure_path() {
    [[ "$no_modify_path" == "true" ]] && return 0
    [[ ":$PATH:" == *":$INSTALL_DIR:"* ]] && return 0

    local xdg_config_home=${XDG_CONFIG_HOME:-$HOME/.config}
    local current_shell
    current_shell=$(basename "${SHELL:-bash}")

    local config_files=""
    case $current_shell in
        fish)
            config_files="$HOME/.config/fish/config.fish"
            ;;
        zsh)
            config_files="${ZDOTDIR:-$HOME}/.zshrc ${ZDOTDIR:-$HOME}/.zshenv $xdg_config_home/zsh/.zshrc $xdg_config_home/zsh/.zshenv"
            ;;
        bash)
            config_files="$HOME/.bashrc $HOME/.bash_profile $HOME/.profile $xdg_config_home/bash/.bashrc $xdg_config_home/bash/.bash_profile"
            ;;
        ash|sh)
            config_files="$HOME/.ashrc $HOME/.profile /etc/profile"
            ;;
        *)
            config_files="$HOME/.profile $HOME/.bashrc"
            ;;
    esac

    local config_file=""
    for file in $config_files; do
        if [[ -f $file ]]; then
            config_file=$file
            break
        fi
    done

    local path_cmd="export PATH=$INSTALL_DIR:\$PATH"
    [ "$current_shell" = "fish" ] && path_cmd="fish_add_path $INSTALL_DIR"

    if [[ -z $config_file ]]; then
        print_message warn "No config file found for ${current_shell}. Manually add:"
        echo -e "  $path_cmd"
    else
        add_to_path "$config_file" "$path_cmd"
    fi
}

if [ -n "$binary_path" ]; then
    install_from_binary
else
    download_and_install
fi

configure_path

if [ -n "${GITHUB_ACTIONS-}" ] && [ "${GITHUB_ACTIONS}" = "true" ]; then
    echo "$INSTALL_DIR" >> "$GITHUB_PATH"
    echo -e "${MUTED}Added ${NC}${INSTALL_DIR}${MUTED} to \$GITHUB_PATH${NC}"
fi

cat <<'BANNER'

                    ▄
 █▀▀█ █▀▀█ █▀▀▄ █▀▀▀
 █░░█ █░░█ █░░█ █░▄▄
 ▀▀▀▀ ▀▀▀▀ ▀  ▀ ▀▀▀▀

BANNER

echo -e "${MUTED}Codg installed to:${NC} ${INSTALL_DIR}/${APP}"
echo ""
echo -e "  cd <project>  ${MUTED}# open a project${NC}"
echo -e "  ${APP}          ${MUTED}# run codg${NC}"
echo ""
echo -e "${MUTED}More info: ${NC}https://github.com/${REPO}"
echo ""
