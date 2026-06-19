#!/usr/bin/env bash
set -euo pipefail

# Helix CLI 构建脚本
# 委托给 Makefile 执行，保持单一构建源

CMD="${1:-dev}"

case "$CMD" in
  dev)
    make build
    ;;
  release)
    make release
    ;;
  clean)
    make clean
    ;;
  test)
    make test
    ;;
  test-cover)
    make test-cover
    ;;
  lint)
    make lint
    ;;
  tui)
    make tui
    ;;
  install)
    make install
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
