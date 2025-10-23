# Makefile for AethonX

# Variables
BINARY_NAME=aethonx
INSTALLER_NAME=install-deps
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Paths
CMD_PATH=./cmd/aethonx
INSTALLER_PATH=./cmd/install-deps
BUILD_DIR=./build

# Colors for output
GREEN=\033[0;32m
YELLOW=\033[0;33m
RED=\033[0;31m
NC=\033[0m # No Color

.PHONY: help build test clean install lint fmt vet run dev

# Default target
help: ## Show this help message
	@echo "$(GREEN)AethonX - Available commands:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "$(GREEN)Building $(BINARY_NAME)...$(NC)"
	@go build $(LDFLAGS) -o $(BINARY_NAME) $(CMD_PATH)
	@echo "$(GREEN)✓ Build complete: ./$(BINARY_NAME)$(NC)"

build-all: ## Build for multiple platforms
	@echo "$(GREEN)Building for multiple platforms...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_PATH)
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_PATH)
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_PATH)
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_PATH)
	@echo "$(GREEN)✓ Multi-platform build complete in $(BUILD_DIR)/$(NC)"

install: build ## Install binary to $GOPATH/bin
	@echo "$(GREEN)Installing $(BINARY_NAME)...$(NC)"
	@go install $(LDFLAGS) $(CMD_PATH)
	@echo "$(GREEN)✓ Installed to $(shell go env GOPATH)/bin/$(BINARY_NAME)$(NC)"

build-installer: ## Build the dependency installer
	@echo "$(GREEN)Building $(INSTALLER_NAME)...$(NC)"
	@go build -o $(INSTALLER_NAME) $(INSTALLER_PATH)
	@echo "$(GREEN)✓ Build complete: ./$(INSTALLER_NAME)$(NC)"

install-deps: build-installer ## Install all AethonX dependencies
	@echo "$(GREEN)Installing AethonX dependencies...$(NC)"
	@./$(INSTALLER_NAME)

check-deps: build-installer ## Check AethonX dependencies status
	@echo "$(GREEN)Checking AethonX dependencies...$(NC)"
	@./$(INSTALLER_NAME) --check

test: ## Run tests
	@echo "$(GREEN)Running tests...$(NC)"
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)✓ Tests complete$(NC)"

test-short: ## Run tests without -race for faster feedback
	@echo "$(GREEN)Running tests (short mode)...$(NC)"
	@go test -v -coverprofile=coverage.out ./...
	@echo "$(GREEN)✓ Tests complete$(NC)"

test-coverage: test ## Run tests and show coverage
	@go tool cover -html=coverage.out

test-coverage-report: test ## Run tests and show coverage summary
	@echo "$(GREEN)Coverage Summary:$(NC)"
	@go tool cover -func=coverage.out | grep total:

coverage: test ## Alias for test with coverage summary
	@echo "$(GREEN)Coverage by Package:$(NC)"
	@go test -cover ./internal/... 2>&1 | grep -E "coverage:"
	@echo ""
	@echo "$(GREEN)Total Coverage:$(NC)"
	@go tool cover -func=coverage.out | grep total:

coverage-html: test ## Generate HTML coverage report and open in browser
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Coverage report generated: coverage.html$(NC)"

lint: ## Run linters
	@echo "$(GREEN)Running linters...$(NC)"
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "$(YELLOW)golangci-lint not installed, skipping...$(NC)"; \
		echo "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

fmt: ## Format code
	@echo "$(GREEN)Formatting code...$(NC)"
	@go fmt ./...
	@echo "$(GREEN)✓ Code formatted$(NC)"

vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(NC)"
	@go vet ./...
	@echo "$(GREEN)✓ Vet complete$(NC)"

tidy: ## Tidy go modules
	@echo "$(GREEN)Tidying modules...$(NC)"
	@go mod tidy
	@echo "$(GREEN)✓ Modules tidied$(NC)"

clean: ## Clean build artifacts
	@echo "$(GREEN)Cleaning...$(NC)"
	@rm -rf $(BINARY_NAME) $(INSTALLER_NAME) $(BUILD_DIR) coverage.out coverage.html aethonx_out/
	@echo "$(GREEN)✓ Clean complete$(NC)"

run: build ## Build and run with example
	@echo "$(GREEN)Running $(BINARY_NAME)...$(NC)"
	@./$(BINARY_NAME) -target example.com

dev: ## Run without building (go run)
	@echo "$(GREEN)Running in dev mode...$(NC)"
	@go run $(CMD_PATH) -target example.com

version: ## Show version info
	@echo "Version:  $(VERSION)"
	@echo "Commit:   $(COMMIT)"
	@echo "Built:    $(DATE)"

check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)
	@echo "$(GREEN)✓ All checks passed$(NC)"

ci: tidy vet test ## CI pipeline: tidy, vet, test with coverage
	@echo "$(GREEN)Running CI pipeline...$(NC)"
	@echo ""
	@echo "$(GREEN)1/3 Tidying modules...$(NC)"
	@go mod tidy
	@echo "$(GREEN)2/3 Running go vet...$(NC)"
	@go vet ./...
	@echo "$(GREEN)3/3 Running tests with coverage...$(NC)"
	@go test -race -coverprofile=coverage.out ./...
	@echo ""
	@echo "$(GREEN)Coverage Summary:$(NC)"
	@go tool cover -func=coverage.out | grep total:
	@echo ""
	@echo "$(GREEN)✓ CI pipeline complete$(NC)"

ci-lint: ci lint ## CI pipeline with linting
	@echo "$(GREEN)✓ CI pipeline with linting complete$(NC)"

bench: ## Run benchmarks
	@echo "$(GREEN)Running benchmarks...$(NC)"
	@go test -bench=. -benchmem ./...
	@echo "$(GREEN)✓ Benchmarks complete$(NC)"

# Example targets for common operations
scan-example: build ## Scan example.com
	./$(BINARY_NAME) -target example.com

scan-json: build ## Scan example.com with JSON output
	./$(BINARY_NAME) -target example.com -out.json -out results/

# Show project structure
tree: ## Show project structure
	@tree -I '.git|build' -L 3

# Count lines of code
loc: ## Count lines of code
	@echo "$(GREEN)Lines of code:$(NC)"
	@find . -name "*.go" -not -path "./vendor/*" | xargs wc -l | tail -1
