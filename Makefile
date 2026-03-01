.PHONY: all build gui gui-windows gui-macos clean test race lint help

BINARY_DIR      := bin
SERVER_BIN      := $(BINARY_DIR)/kvd
CLIENT_BIN      := $(BINARY_DIR)/kvcli
GUI_WINDOWS_BIN := $(BINARY_DIR)/kvgui.exe
SERVER_SRC      := ./cmd/kvd
CLIENT_SRC      := ./cmd/kvcli
GUI_WINDOWS_DIR := ./cmd/kvgui/windows
GUI_MACOS_DIR   := ./cmd/kvgui/macos

all: build

build:
	@echo "Building..."
	@mkdir -p $(BINARY_DIR)
	go build -o $(SERVER_BIN) $(SERVER_SRC)
	go build -o $(CLIENT_BIN) $(CLIENT_SRC)
	@echo "Build complete!"

gui: gui-windows

gui-windows:
	@echo "Building Windows GUI (requires Wails CLI)..."
	@mkdir -p $(BINARY_DIR)
	@cd $(GUI_WINDOWS_DIR) && wails build -platform windows/amd64
	@cp $(GUI_WINDOWS_DIR)/build/bin/kvgui.exe $(GUI_WINDOWS_BIN)
	@echo "Windows GUI build complete! Binary: $(GUI_WINDOWS_BIN)"

gui-macos:
	@echo "Building macOS Intel GUI (requires Wails CLI)..."
	@mkdir -p $(BINARY_DIR)
	@cd $(GUI_MACOS_DIR) && ~/go/bin/wails build -platform darwin/amd64
	@cp -r "$(GUI_MACOS_DIR)/build/bin/kvgui-macos.app" $(BINARY_DIR)/
	@echo "macOS GUI build complete! App: $(BINARY_DIR)/kvgui-macos.app"

clean:
	@echo "Cleaning..."
	@rm -rf $(BINARY_DIR)
	@go clean -cache
	@echo "Clean complete!"

test:
	@echo "Running tests..."
	go test -v ./...

race:
	@echo "Running tests with race detector..."
	go test -race -v ./...

lint:
	@echo "Running lint..."
	@if command -v golint >/dev/null 2>&1; then \
		golint ./...; \
	else \
		echo "golint not installed, skipping"; \
	fi

help:
	@echo "Available targets:"
	@echo "  all          - Build all binaries (default)"
	@echo "  build        - Build kvd and kvcli"
	@echo "  gui          - Alias for gui-windows"
	@echo "  gui-windows  - Build Windows GUI -> $(GUI_WINDOWS_BIN) (requires: go install github.com/wailsapp/wails/v2/cmd/wails@latest)"
	@echo "  gui-macos    - Build macOS Intel GUI -> $(BINARY_DIR)/kvgui.app (requires: go install github.com/wailsapp/wails/v2/cmd/wails@latest)"
	@echo "  clean        - Remove binaries and clean cache"
	@echo "  test         - Run all tests"
	@echo "  race         - Run tests with race detector"
	@echo "  lint         - Run linter"
	@echo "  help         - Show this help message"
