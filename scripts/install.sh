#!/usr/bin/env bash
set -euo pipefail

# Helix CLI 一键安装脚本
# 用法: curl -fsSL https://raw.githubusercontent.com/ShawnLiuSZ/Helix/main/scripts/install.sh | bash

REPO="ShawnLiuSZ/Helix"
BIN_NAME="helix"
INSTALL_DIR="${HELIX_INSTALL_DIR:-/usr/local/bin}"

# 检测平台
detect_platform() {
    local os arch
    case "$(uname -s)" in
        Linux)  os="linux" ;;
        Darwin) os="darwin" ;;
        *)      echo "Unsupported OS: $(uname -s)" >&2; exit 1 ;;
    esac
    case "$(uname -m)" in
        x86_64)  arch="amd64" ;;
        aarch64) arch="arm64" ;;
        arm64)   arch="arm64" ;;
        *)       echo "Unsupported arch: $(uname -m)" >&2; exit 1 ;;
    esac
    echo "${os}/${arch}"
}

PLATFORM=$(detect_platform)
VERSION="${HELIX_VERSION:-latest}"

echo "Helix CLI Installer"
echo "  Platform: ${PLATFORM}"
echo "  Version:  ${VERSION}"
echo ""

if [ "$VERSION" = "latest" ]; then
    DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${BIN_NAME}-${PLATFORM//\//-}"
else
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BIN_NAME}-${PLATFORM//\//-}"
fi

TMP_DIR=$(mktemp -d)
TMP_FILE="${TMP_DIR}/${BIN_NAME}"

echo "Downloading ${DOWNLOAD_URL}..."
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_FILE}"
elif command -v wget >/dev/null 2>&1; then
    wget -q "${DOWNLOAD_URL}" -O "${TMP_FILE}"
else
    echo "Error: curl or wget required" >&2
    exit 1
fi

chmod +x "${TMP_FILE}"

# 安装
if [ -w "${INSTALL_DIR}" ]; then
    mv "${TMP_FILE}" "${INSTALL_DIR}/${BIN_NAME}"
else
    echo "Need sudo to install to ${INSTALL_DIR}"
    sudo mv "${TMP_FILE}" "${INSTALL_DIR}/${BIN_NAME}"
fi

rm -rf "${TMP_DIR}"

echo ""
echo "Helix CLI installed to ${INSTALL_DIR}/${BIN_NAME}"
echo "Run 'helix setup' to get started."
