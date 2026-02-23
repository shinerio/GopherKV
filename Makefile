.PHONY: all build gui clean test race lint help

BINARY_DIR := bin
SERVER_BIN := $(BINARY_DIR)/kvd
CLIENT_BIN := $(BINARY_DIR)/kvcli
GUI_BIN    := $(BINARY_DIR)/kvgui
SERVER_SRC := ./cmd/kvd
CLIENT_SRC := ./cmd/kvcli
GUI_DIR    := ./cmd/kvgui

all: build

build:
	@echo "Building..."
	@mkdir -p $(BINARY_DIR)
	go build -o $(SERVER_BIN) $(SERVER_SRC)
	go build -o $(CLIENT_BIN) $(CLIENT_SRC)
	@echo "Build complete!"

gui:
	@echo "Building GUI (requires Wails CLI)..."
	@mkdir -p $(BINARY_DIR)
	@cd $(GUI_DIR) && wails build
	@cp $(GUI_DIR)/build/bin/kvgui $(GUI_BIN)
	@echo "GUI build complete! Binary: $(GUI_BIN)"

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
	@echo "  all      - Build all binaries (default)"
	@echo "  build    - Build kvd and kvcli"
	@echo "  gui      - Build kvgui -> $(GUI_BIN) (requires: go install github.com/wailsapp/wails/v2/cmd/wails@latest)"
	@echo "  clean    - Remove binaries and clean cache"
	@echo "  test     - Run all tests"
	@echo "  race     - Run tests with race detector"
	@echo "  lint     - Run linter"
	@echo "  help     - Show this help message"
