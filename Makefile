# Makefile for Telegram Trader Coder Bot

# Auto-load environment variables from .env for all targets
# Note: This only affects commands run via `make` (it won't export to your parent shell)
ifneq (,$(wildcard .env))
include .env
# Export all keys present in .env so they are available to recipes
export $(shell sed -n 's/^\s*\([A-Za-z_][A-Za-z0-9_]*\)\s*=.*/\1/p' .env)
endif

# Variables
BINARY_NAME=bot
BUILD_DIR=bin
DOCKER_IMAGE=jamesguan777/telegram-trader-bot
DOCKER_TAG=latest
# Target platform for Docker image builds (set to Docker Hub runtime target)
DOCKER_PLATFORM=linux/amd64

# Go related variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_UNIX=$(BUILD_DIR)/$(BINARY_NAME)_unix

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(shell git describe --tags --always --dirty)"

# Default target
.DEFAULT_GOAL := build

# Build the application
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/bot
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for multiple platforms
.PHONY: build-all
build-all: clean
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	
	# Linux
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/bot
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/bot
	
	# macOS
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/bot
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/bot
	
	# Windows
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/bot
	
	@echo "Multi-platform build complete!"

# Run the application
.PHONY: run
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run in development mode
.PHONY: dev
dev:
	@echo "Running in development mode..."
	$(GOCMD) run ./cmd/bot

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete!"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Update dependencies
.PHONY: deps-update
deps-update:
	@echo "Updating dependencies..."
	$(GOCMD) get -u ./...
	$(GOMOD) tidy

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

# Lint code
.PHONY: lint
lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Docker targets
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	@echo "Platform: $(DOCKER_PLATFORM)"
	@echo "Image: $(DOCKER_IMAGE):$(DOCKER_TAG)"
	docker buildx build --platform $(DOCKER_PLATFORM) -t $(DOCKER_IMAGE):$(DOCKER_TAG) --load .

.PHONY: docker-run
docker-run: docker-build
	@echo "Running Docker container..."
	docker run --rm -p 9095:9095 \
		-e TELEGRAM_BOT_TOKEN=$$TELEGRAM_BOT_TOKEN \
		-e WEBHOOK_PUBLIC_URL=$$WEBHOOK_PUBLIC_URL \
		-e OPENAI_API_KEY=$$OPENAI_API_KEY \
		-e PORT=9095 \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: docker-push
docker-push: docker-build
	@echo "Pushing Docker image..."
	@echo "Platform: $(DOCKER_PLATFORM)"
	@echo "Image: $(DOCKER_IMAGE):$(DOCKER_TAG)"
	docker buildx build --platform $(DOCKER_PLATFORM) -t $(DOCKER_IMAGE):$(DOCKER_TAG) --push .

.PHONY: docker-login
docker-login:
	@echo "Logging in to Docker registry..."
	docker login

# Docker Compose helpers
.PHONY: compose-up compose-down compose-logs compose-ps compose-restart compose-pull
compose-up:
	@echo "Starting services with Docker Compose..."
	docker compose up -d --build

compose-down:
	@echo "Stopping services with Docker Compose..."
	docker compose down

compose-logs:
	@echo "Tailing Docker Compose logs (Ctrl+C to stop)..."
	docker compose logs -f

compose-ps:
	@echo "Listing Docker Compose services..."
	docker compose ps

compose-restart:
	@echo "Restarting bot service..."
	docker compose restart telegram-bot

compose-pull:
	@echo "Pulling images defined in docker-compose.yml..."
	docker compose pull

# Development helpers
.PHONY: install-tools
install-tools:
	@echo "Installing development tools..."
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOCMD) install golang.org/x/tools/cmd/goimports@latest

.PHONY: check-env
check-env:
	@echo "Checking environment variables..."
	@if [ -z "$$TELEGRAM_BOT_TOKEN" ]; then echo "❌ TELEGRAM_BOT_TOKEN not set"; else echo "✅ TELEGRAM_BOT_TOKEN is set"; fi
	@if [ -z "$$WEBHOOK_PUBLIC_URL" ]; then echo "❌ WEBHOOK_PUBLIC_URL not set"; else echo "✅ WEBHOOK_PUBLIC_URL is set"; fi
	@if [ -z "$$OPENAI_API_KEY" ]; then echo "❌ OPENAI_API_KEY not set"; else echo "✅ OPENAI_API_KEY is set"; fi

.PHONY: load-env
load-env:
	@echo "Loading environment variables from .env file..."
	@if [ -f .env ]; then \
		export $$(cat .env | grep -v '^#' | xargs); \
		echo "✅ Environment variables loaded successfully!"; \
	else \
		echo "❌ .env file not found!"; \
		exit 1; \
	fi

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build          - Build the application"
	@echo "  build-all      - Build for multiple platforms"
	@echo "  run            - Build and run the application"
	@echo "  dev            - Run in development mode"
	@echo "  clean          - Clean build artifacts"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  deps           - Install dependencies"
	@echo "  deps-update    - Update dependencies"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Build and run Docker container"
	@echo "  docker-push    - Build and push Docker image"
	@echo "  install-tools  - Install development tools"
	@echo "  check-env      - Check environment variables"
	@echo "  load-env       - Load environment variables from .env file"
	@echo "  help           - Show this help message"
