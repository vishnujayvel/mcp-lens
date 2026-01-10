# MCP Lens Makefile

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)"

.PHONY: all build test clean install lint fmt help

all: build

## build: Build the binary
build:
	go build $(LDFLAGS) -o mcp-lens ./cmd/mcp-lens

## test: Run tests
test:
	go test -v ./...

## test-coverage: Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

## clean: Clean build artifacts
clean:
	rm -f mcp-lens
	rm -f coverage.out coverage.html
	rm -rf build/ dist/

## install: Install to GOPATH/bin
install:
	go install $(LDFLAGS) ./cmd/mcp-lens

## lint: Run linters
lint:
	go vet ./...
	@which golangci-lint > /dev/null && golangci-lint run || echo "golangci-lint not installed"

## fmt: Format code
fmt:
	go fmt ./...
	@which goimports > /dev/null && goimports -w . || echo "goimports not installed"

## tidy: Tidy go modules
tidy:
	go mod tidy

## run: Run the server
run: build
	./mcp-lens serve

## dev: Run in development mode (with auto-restart)
dev:
	@which air > /dev/null && air || go run ./cmd/mcp-lens serve

## cross-compile: Build for all platforms
cross-compile: clean
	@mkdir -p build
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o build/mcp-lens-darwin-arm64 ./cmd/mcp-lens
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o build/mcp-lens-darwin-amd64 ./cmd/mcp-lens
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o build/mcp-lens-linux-amd64 ./cmd/mcp-lens
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o build/mcp-lens-linux-arm64 ./cmd/mcp-lens
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o build/mcp-lens-windows-amd64.exe ./cmd/mcp-lens

## help: Show this help
help:
	@echo "MCP Lens - Claude Code Observability Dashboard"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
