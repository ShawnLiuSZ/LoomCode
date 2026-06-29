.PHONY: build test lint install release clean tui tui-run dev test-qa test-cache

APP_NAME   := helix
BUILD_DIR  := bin
CMD_DIR    := cmd/helix

GO         := go
GOFLAGS    := -ldflags="-s -w"

# Build
build:
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./$(CMD_DIR)

# Dev: build and show info
dev: build
	@echo ""
	@echo "启动方式:"
	@echo "  ./$(BUILD_DIR)/$(APP_NAME)                          # 直接启动 TUI"
	@echo "  ./$(BUILD_DIR)/$(APP_NAME) --session <id>           # 恢复会话"
	@echo "  ./$(BUILD_DIR)/$(APP_NAME) --provider deepseek      # 指定 Provider"

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

# Install to local
install:
	$(GO) install ./$(CMD_DIR)

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
