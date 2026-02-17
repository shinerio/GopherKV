.PHONY: all build clean test race help

BINARY_DIR := bin
SERVER_BIN := $(BINARY_DIR)/kvd
CLIENT_BIN := $(BINARY_DIR)/kvcli
SERVER_SRC := ./cmd/kvd
CLIENT_SRC := ./cmd/kvcli

all: build

build:
	@echo "Building..."
	@mkdir -p $(BINARY_DIR)
	go build -o $(SERVER_BIN) $(SERVER_SRC)
	go build -o $(CLIENT_BIN) $(CLIENT_SRC)
	@echo "Build complete!"

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
	@echo "  clean    - Remove binaries and clean cache"
	@echo "  test     - Run all tests"
	@echo "  race     - Run tests with race detector"
	@echo "  lint     - Run linter"
	@echo "  help     - Show this help message"
