#!/bin/sh
# install.sh — install the gofi CLI on Linux and macOS
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/joaoprofile/gofi-cli/main/install.sh | sh
#   GOFI_VERSION=v0.2.0 curl -fsSL https://raw.githubusercontent.com/joaoprofile/gofi-cli/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/joaoprofile/gofi-cli/main/install.sh | sh -s -- --bin-dir /opt/gofi/bin
#   curl -fsSL https://raw.githubusercontent.com/joaoprofile/gofi-cli/main/install.sh | sh -s -- --add-to-path

set -eu

REPO="joaoprofile/gofi-cli"
BINARY="gofi"
VERSION="${GOFI_VERSION:-latest}"
BIN_DIR=""
ADD_TO_PATH=0

# parse flags (when piped through `sh -s -- --flag value`)
while [ $# -gt 0 ]; do
    case "$1" in
        --bin-dir)      BIN_DIR="$2"; shift 2 ;;
        --version)      VERSION="$2"; shift 2 ;;
        --add-to-path)  ADD_TO_PATH=1; shift ;;
        -h|--help)
            cat <<EOF
install.sh — install the gofi CLI on Linux and macOS
Flags:
  --bin-dir <path>   target directory (default: /usr/local/bin or \$HOME/.local/bin)
  --version <tag>    install a specific version (default: latest)
  --add-to-path      append PATH export to your shell rc file if missing
EOF
            exit 0 ;;
        *) printf "unknown flag: %s\n" "$1" >&2; exit 1 ;;
    esac
done

err() { printf "\033[31mError:\033[0m %s\n" "$1" >&2; exit 1; }
log() { printf "\033[36m==>\033[0m %s\n" "$1"; }
warn() { printf "\033[33mWarning:\033[0m %s\n" "$1" >&2; }

require() {
    command -v "$1" >/dev/null 2>&1 || err "missing required tool: $1"
}

require uname
require mktemp
require tar
require grep
require awk

# detect download tool
if command -v curl >/dev/null 2>&1; then
    download()  { curl -fsSL "$1" -o "$2"; }
    fetch_url() { curl -fsSL "$1"; }
elif command -v wget >/dev/null 2>&1; then
    download()  { wget -q "$1" -O "$2"; }
    fetch_url() { wget -qO- "$1"; }
else
    err "need curl or wget on PATH"
fi

# detect sha256 tool
if command -v sha256sum >/dev/null 2>&1; then
    sha256_check() { ( cd "$2" && echo "$1  $3" | sha256sum -c - >/dev/null 2>&1 ); }
elif command -v shasum >/dev/null 2>&1; then
    sha256_check() { ( cd "$2" && echo "$1  $3" | shasum -a 256 -c - >/dev/null 2>&1 ); }
else
    err "need sha256sum or shasum on PATH"
fi

# detect platform
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
    linux|darwin) ;;
    *) err "unsupported OS: $os (use install.ps1 on Windows)" ;;
esac

arch=$(uname -m)
case "$arch" in
    x86_64|amd64)   arch="amd64" ;;
    aarch64|arm64)  arch="arm64" ;;
    *) err "unsupported architecture: $arch" ;;
esac

# resolve version
if [ "$VERSION" = "latest" ]; then
    log "resolving latest release"
    VERSION=$(fetch_url "https://api.github.com/repos/$REPO/releases/latest" \
        | grep '"tag_name"' \
        | head -n1 \
        | awk -F'"' '{print $4}')
    [ -n "$VERSION" ] || err "could not resolve latest version (no releases yet?)"
fi

log "installing gofi $VERSION for $os/$arch"

version_no_v="${VERSION#v}"
asset="${BINARY}_${version_no_v}_${os}_${arch}.tar.gz"
asset_url="https://github.com/$REPO/releases/download/$VERSION/$asset"
checksums_url="https://github.com/$REPO/releases/download/$VERSION/checksums.txt"

# download to temp
tmp=$(mktemp -d 2>/dev/null || mktemp -d -t 'gofi-install')
trap 'rm -rf "$tmp"' EXIT INT TERM HUP

log "downloading $asset"
download "$asset_url" "$tmp/$asset" || err "failed to download $asset_url"

log "downloading checksums.txt"
download "$checksums_url" "$tmp/checksums.txt" || err "failed to download checksums.txt"

# verify sha256
expected_hash=$(awk -v a="$asset" '$2==a {print $1}' "$tmp/checksums.txt")
[ -n "$expected_hash" ] || err "asset $asset not listed in checksums.txt"
log "verifying sha256"
sha256_check "$expected_hash" "$tmp" "$asset" || err "checksum mismatch for $asset"

# extract
log "extracting"
tar -xzf "$tmp/$asset" -C "$tmp"
[ -f "$tmp/$BINARY" ] || err "binary $BINARY not found in archive"
chmod +x "$tmp/$BINARY"

# determine install dir
if [ -z "$BIN_DIR" ]; then
    if [ "$(id -u)" = "0" ]; then
        BIN_DIR="/usr/local/bin"
    elif [ -w /usr/local/bin ]; then
        BIN_DIR="/usr/local/bin"
    else
        BIN_DIR="$HOME/.local/bin"
    fi
fi
mkdir -p "$BIN_DIR" || err "could not create $BIN_DIR"

# install
log "installing to $BIN_DIR/$BINARY"
mv "$tmp/$BINARY" "$BIN_DIR/$BINARY" || err "failed to move binary to $BIN_DIR"

# PATH advisory — detect user's shell and offer the precise fix
detect_rc_file() {
    user_shell="${SHELL##*/}"
    case "$user_shell" in
        zsh)
            printf "%s/.zshrc" "${ZDOTDIR:-$HOME}"
            ;;
        bash)
            if [ "$os" = "darwin" ] && [ -f "$HOME/.bash_profile" ]; then
                printf "%s/.bash_profile" "$HOME"
            else
                printf "%s/.bashrc" "$HOME"
            fi
            ;;
        fish)
            printf "%s/.config/fish/config.fish" "$HOME"
            ;;
        *)
            printf ""
            ;;
    esac
}

path_export_line() {
    if [ "${SHELL##*/}" = "fish" ]; then
        printf 'set -gx PATH "%s" $PATH' "$BIN_DIR"
    else
        printf 'export PATH="%s:$PATH"' "$BIN_DIR"
    fi
}

case ":${PATH:-}:" in
    *":$BIN_DIR:"*) ;;
    *)
        rc_file=$(detect_rc_file)
        export_line=$(path_export_line)

        if [ "$ADD_TO_PATH" = "1" ] && [ -n "$rc_file" ]; then
            mkdir -p "$(dirname "$rc_file")"
            touch "$rc_file"
            if grep -Fqs "$BIN_DIR" "$rc_file"; then
                log "$BIN_DIR already referenced in $rc_file — skipping"
            else
                printf '\n# added by gofi installer\n%s\n' "$export_line" >> "$rc_file"
                log "added PATH export to $rc_file"
            fi
            printf "  Open a new terminal or run: source %s\n" "$rc_file"
        else
            warn "$BIN_DIR is not on your PATH."
            if [ -n "$rc_file" ]; then
                printf "  Run this once to fix it:\n"
                printf "    echo '%s' >> %s && source %s\n" "$export_line" "$rc_file" "$rc_file"
                printf "  Or re-run the installer with --add-to-path.\n"
            else
                printf "  Add to your shell config:\n    %s\n" "$export_line"
            fi
        fi
        ;;
esac

log "done — run 'gofi h' to get started"
