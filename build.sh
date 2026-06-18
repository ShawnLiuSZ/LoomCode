#!/usr/bin/env bash
set -euo pipefail

# Helix CLI 构建脚本
# 用法: bash build.sh [dev|release|clean|test|tui]

APP_NAME="helix"
BUILD_DIR="bin"
CMD_DIR="cmd/helix"
VERSION="${VERSION:-dev}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
DATE="$(date -u '+%Y-%m-%d_%H:%M:%S')"
GOFLAGS=(-ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}")

# 颜色
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()   { echo -e "${RED}[ERROR]${NC} $*"; }

CMD="${1:-dev}"

case "$CMD" in
  dev)
    info "Building development binary..."
    go build "${GOFLAGS[@]}" -o "${BUILD_DIR}/${APP_NAME}" "./${CMD_DIR}"
    ok "Binary: ${BUILD_DIR}/${APP_NAME}"
    ls -lh "${BUILD_DIR}/${APP_NAME}"
    ;;

  release)
    info "Building release binaries for all platforms..."
    mkdir -p "${BUILD_DIR}"

    PLATFORMS=(
      "darwin/amd64" "darwin/arm64"
      "linux/amd64"  "linux/arm64"
      "windows/amd64" "windows/arm64"
    )

    for platform in "${PLATFORMS[@]}"; do
      GOOS="${platform%/*}"
      GOARCH="${platform#*/}"
      output="${BUILD_DIR}/${APP_NAME}-${GOOS}-${GOARCH}"
      if [ "$GOOS" = "windows" ]; then
        output="${output}.exe"
      fi
      info "Building ${GOOS}/${GOARCH}..."
      GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 \
        go build ${GOFLAGS} -o "$output" "./${CMD_DIR}"
    done

    ok "Release binaries built in ${BUILD_DIR}/"
    ls -lh "${BUILD_DIR}/"
    ;;

  clean)
    info "Cleaning build artifacts..."
    rm -rf "${BUILD_DIR}" dist/ coverage.out
    ok "Cleaned"
    ;;

  test)
    info "Running tests..."
    go test ./... -count=1 -v
    ok "Tests passed"
    ;;

  test-cover)
    info "Running tests with coverage..."
    go test ./... -cover -coverprofile=coverage.out
    go tool cover -func=coverage.out
    ok "Coverage report generated"
    ;;

  lint)
    info "Running linter..."
    golangci-lint run ./...
    ok "Lint passed"
    ;;

  tui)
    info "Building TUI binary..."
    go build "${GOFLAGS[@]}" -o "${BUILD_DIR}/${APP_NAME}" "./${CMD_DIR}"

    # 检查 TUI 依赖
    info "Verifying TUI dependencies..."
    go list -m github.com/charmbracelet/bubbletea > /dev/null 2>&1 && \
      ok "Bubble Tea: installed" || warn "Bubble Tea: not found"
    go list -m github.com/charmbracelet/lipgloss > /dev/null 2>&1 && \
      ok "Lip Gloss: installed" || warn "Lip Gloss: not found"

    ok "TUI binary ready: ${BUILD_DIR}/${APP_NAME}"
    echo ""
    echo "启动方式:"
    echo "  ./${BUILD_DIR}/${APP_NAME}                          # 直接启动 TUI"
    echo "  ./${BUILD_DIR}/${APP_NAME} --session <id>           # 恢复会话"
    echo "  ./${BUILD_DIR}/${APP_NAME} --provider deepseek      # 指定 Provider"
    echo "  ./${BUILD_DIR}/${APP_NAME} --model deepseek-v4-pro  # 指定模型"
    ;;

  install)
    info "Installing to GOPATH/bin..."
    go install "./${CMD_DIR}"
    ok "Installed: $(which ${APP_NAME})"
    ;;

  *)
    echo "Helix CLI Build Script"
    echo ""
    echo "Usage: bash build.sh [command]"
    echo ""
    echo "Commands:"
    echo "  dev         Build development binary (default)"
    echo "  release     Cross-compile for all platforms"
    echo "  tui         Build TUI binary with dependency check"
    echo "  test        Run all tests"
    echo "  test-cover  Run tests with coverage report"
    echo "  lint        Run linter"
    echo "  install     Install to GOPATH/bin"
    echo "  clean       Remove build artifacts"
    echo ""
    echo "Examples:"
    echo "  bash build.sh              # Quick dev build"
    echo "  bash build.sh tui          # TUI build with checks"
    echo "  VERSION=0.1.0 bash build.sh release  # Versioned release"
    ;;
esac
