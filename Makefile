.PHONY: build test lint install release clean tui tui-run dev test-qa test-cache

APP_NAME   := loomcode
SHORT_NAME := loom
BUILD_DIR  := bin
CMD_DIR    := cmd/loomcode

GO         := go
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS    := -s -w \
              -X main.version=$(VERSION) \
              -X main.commit=$(COMMIT) \
              -X main.date=$(BUILD_DATE)
GOFLAGS    := -ldflags="$(LDFLAGS)"

# Build: 同时构建 loomcode 和 loom 两个二进制（同一源码）
build:
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./$(CMD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(SHORT_NAME) ./$(CMD_DIR)

# Dev: build and show info
dev: build
	@echo ""
	@echo "启动方式:"
	@echo "  ./$(BUILD_DIR)/$(APP_NAME)  或  ./$(BUILD_DIR)/$(SHORT_NAME)   # 直接启动 TUI"
	@echo "  ./$(BUILD_DIR)/$(APP_NAME) --session <id>                       # 恢复会话"
	@echo "  ./$(BUILD_DIR)/$(APP_NAME) --provider deepseek                  # 指定 Provider"

# TUI: build with dependency verification
tui:
	@echo "Building TUI binary..."
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./$(CMD_DIR)
	@echo "TUI dependencies:"
	@$(GO) list -m github.com/charmbracelet/bubbletea 2>/dev/null && echo "  ✓ Bubble Tea" || echo "  ✗ Bubble Tea not found"
	@$(GO) list -m github.com/charmbracelet/lipgloss 2>/dev/null && echo "  ✓ Lip Gloss" || echo "  ✗ Lip Gloss not found"
	@echo ""
	@echo "TUI binary: $(BUILD_DIR)/$(APP_NAME)"
	@ls -lh $(BUILD_DIR)/$(APP_NAME)

# TUI run: build and start TUI
tui-run: build
	./$(BUILD_DIR)/$(APP_NAME)

# Test
test:
	$(GO) test ./... -count=1

test-cover:
	$(GO) test ./... -cover -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out

test-cover-html:
	$(GO) test ./... -cover -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out

# Lint
lint:
	golangci-lint run ./...

# Install to local: 同时安装 loomcode 和 loom
install:
	$(GO) install $(GOFLAGS) ./$(CMD_DIR)
	@echo "Installed as loomcode"
	@which loomcode >/dev/null 2>&1 && ln -sf "$$(which loomcode)" "$$(dirname $$(which loomcode))/loom" && echo "Symlinked as loom" || echo "Run manually: ln -sf \$$(which loomcode) /usr/local/bin/loom"

# Cross-compile
release:
	goreleaser build --snapshot --clean

# Clean
clean:
	rm -rf $(BUILD_DIR) dist/ coverage.out

# Dev setup
dev-setup:
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# QA: build + vet + test, all must pass
test-qa:
	$(GO) build ./... && $(GO) vet ./... && $(GO) test ./...
	@echo "QA PASSED"

# Cache: run registry and prefix stability tests
test-cache:
	$(GO) test ./internal/tool/ -run TestRegistry -v
	$(GO) test ./internal/agent/ -run TestPrefix -v || true
	@echo "Cache tests done"
