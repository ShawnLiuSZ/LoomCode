#!/usr/bin/env bash
set -euo pipefail

# Helix CLI 一键安装脚本
# 用法: curl -fsSL https://raw.githubusercontent.com/ShawnLiuSZ/Helix/main/scripts/install.sh | bash

REPO="ShawnLiuSZ/Helix"
BIN_NAME="helix"
INSTALL_DIR="${HELIX_INSTALL_DIR:-$HOME/.helix/bin}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
MUTED='\033[0;2m'
NC='\033[0m'

usage() {
    cat <<EOF
Helix CLI Installer

Usage: install.sh [options]

Options:
    -h, --help              Display this help message
    -v, --version <version> Install a specific version (e.g., v0.1.0)
        --no-modify-path    Don't modify shell config files

Examples:
    curl -fsSL https://raw.githubusercontent.com/ShawnLiuSZ/Helix/main/scripts/install.sh | bash
    curl -fsSL https://raw.githubusercontent.com/ShawnLiuSZ/Helix/main/scripts/install.sh | bash -s -- --version v0.1.0
EOF
}

requested_version=""
no_modify_path=false

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
                echo -e "${RED}Error: --version requires a version argument${NC}"
                exit 1
            fi
            ;;
        --no-modify-path)
            no_modify_path=true
            shift
            ;;
        *)
            echo -e "${YELLOW}Warning: Unknown option '$1'${NC}" >&2
            shift
            ;;
    esac
done

# 检测平台
detect_platform() {
    local os arch
    case "$(uname -s)" in
        Linux*)  os="linux" ;;
        Darwin*) os="darwin" ;;
        MINGW*|MSYS*|CYGWIN*) os="windows" ;;
        *)      echo -e "${RED}Unsupported OS: $(uname -s)${NC}" >&2; exit 1 ;;
    esac
    arch=$(uname -m)
    case "$arch" in
        x86_64)  arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *)       echo -e "${RED}Unsupported arch: ${arch}${NC}" >&2; exit 1 ;;
    esac
    # macOS Rosseta 检测
    if [ "$os" = "darwin" ] && [ "$arch" = "amd64" ]; then
        if [ "$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)" = "1" ]; then
            arch="arm64"
        fi
    fi
    echo "${os}-${arch}"
}

PLATFORM=$(detect_platform)
OS="${PLATFORM%%-*}"
ARCH="${PLATFORM##*-}"

echo -e "${BLUE}Helix CLI Installer${NC}"
echo -e "  Platform: ${PLATFORM}"

# 解析版本
if [ -z "$requested_version" ]; then
    echo -e "  Resolving latest version..."
    specific_version=$(curl -sI "https://github.com/${REPO}/releases/latest" \
        | grep -i '^location:' | head -1 | tr -d '\r' \
        | sed -n 's#.*/tag/v\([0-9][^/[:space:]]*\).*#\1#p')

    if [ -z "$specific_version" ]; then
        echo -e "${RED}Failed to fetch latest version${NC}"
        echo -e "${MUTED}Check your network or install a specific version:${NC}"
        echo -e "${MUTED}  bash -s -- --version v0.1.0${NC}"
        exit 1
    fi
else
    requested_version="${requested_version#v}"
    specific_version="$requested_version"
    http_status=$(curl -sI -o /dev/null -w "%{http_code}" "https://github.com/${REPO}/releases/tag/v${specific_version}")
    if [ "$http_status" = "404" ]; then
        echo -e "${RED}Error: Release v${specific_version} not found${NC}"
        exit 1
    fi
fi

echo -e "  Version:  v${specific_version}"
echo ""

# 检查是否已安装
if command -v "$BIN_NAME" >/dev/null 2>&1; then
    installed_version=$("$BIN_NAME" --version 2>/dev/null || echo "unknown")
    if [ "$installed_version" = "v${specific_version}" ]; then
        echo -e "${GREEN}Helix v${specific_version} is already installed${NC}"
        exit 0
    fi
    echo -e "${MUTED}Updating from ${installed_version} to v${specific_version}...${NC}"
fi

# 下载
FILENAME="${BIN_NAME}-${PLATFORM}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/v${specific_version}/${FILENAME}"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo -e "Downloading ${DOWNLOAD_URL}..."
if command -v curl >/dev/null 2>&1; then
    curl -# -L -o "${TMP_DIR}/${FILENAME}" "${DOWNLOAD_URL}"
elif command -v wget >/dev/null 2>&1; then
    wget -q --show-progress "${DOWNLOAD_URL}" -O "${TMP_DIR}/${FILENAME}"
else
    echo -e "${RED}Error: curl or wget required${NC}"
    exit 1
fi

# 解压
echo -e "Extracting..."
tar -xzf "${TMP_DIR}/${FILENAME}" -C "${TMP_DIR}"

# 安装
mkdir -p "${INSTALL_DIR}"
mv "${TMP_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
chmod 755 "${INSTALL_DIR}/${BIN_NAME}"

# PATH 配置
add_to_path() {
    local config_file=$1
    local command=$2

    if grep -Fxq "$command" "$config_file" 2>/dev/null; then
        return 0
    fi

    if [[ -w $config_file ]]; then
        echo "" >> "$config_file"
        echo "# helix" >> "$config_file"
        echo "$command" >> "$config_file"
        echo -e "${MUTED}Added to ${config_file}${NC}"
    else
        echo -e "${YELLOW}Manually add to ${config_file}:${NC}"
        echo -e "  ${command}"
    fi
}

if [[ "$no_modify_path" != "true" ]] && [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
    current_shell=$(basename "${SHELL:-sh}")
    case "$current_shell" in
        fish)
            config_file="$HOME/.config/fish/config.fish"
            if [[ -f "$config_file" ]]; then
                add_to_path "$config_file" "fish_add_path ${INSTALL_DIR}"
            fi
            ;;
        zsh)
            for f in "${ZDOTDIR:-$HOME}/.zshrc" "${ZDOTDIR:-$HOME}/.zshenv"; do
                if [[ -f "$f" ]]; then
                    add_to_path "$f" "export PATH=${INSTALL_DIR}:\$PATH"
                    break
                fi
            done
            ;;
        bash)
            for f in "$HOME/.bashrc" "$HOME/.bash_profile" "$HOME/.profile"; do
                if [[ -f "$f" ]]; then
                    add_to_path "$f" "export PATH=${INSTALL_DIR}:\$PATH"
                    break
                fi
            done
            ;;
        *)
            echo -e "${YELLOW}Add ${INSTALL_DIR} to your PATH manually${NC}"
            ;;
    esac
fi

echo ""
echo -e "${GREEN}Helix CLI v${specific_version} installed to ${INSTALL_DIR}/${BIN_NAME}${NC}"
echo ""
echo -e "${MUTED}Get started:${NC}"
echo ""
echo -e "  helix setup       ${MUTED}# Configure API keys${NC}"
echo -e "  helix             ${MUTED}# Start interactive TUI${NC}"
echo -e "  helix run \"task\"  ${MUTED}# Single task mode${NC}"
echo ""
