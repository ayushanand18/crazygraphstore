
# Makefile for Storage Engine Graph Database

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=storage-engine
EXAMPLE_BINARY=example

# Build directory
BUILD_DIR=build

.PHONY: all build clean test test-race coverage bench run example example-phase1 example-phase2 deps fmt vet lint check stats help

all: deps test build

## build: Build the binary
build:
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v

## example: Run the Phase 1 example (alias for example-phase1)
example: example-phase1

## example-phase1: Run Phase 1 example (In-memory only)
example-phase1:
	@echo "Running Phase 1 example..."
	$(GOCMD) run cmd/example/main.go

## example-phase2: Run Phase 2 example (SSTable + Cache)
example-phase2: deps
	@echo "Running Phase 2 example..."
	$(GOCMD) run cmd/example-phase2/main.go

## test: Run tests with race detector and coverage
test:
	@echo "Running tests..."
	$(GOTEST) ./pkg/... -v -race -cover

## test-race: Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	$(GOTEST) -v -race ./...

## coverage: Generate test coverage report
coverage:
	@echo "Generating coverage report..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## bench: Run benchmarks with extended time
bench:
	@echo "Running benchmarks..."
	$(GOTEST) ./pkg/... -bench=. -benchmem -benchtime=5s

## clean: Clean build artifacts and data directories
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	rm -rf ./data ./data-phase2
	@echo "Clean complete!"

## deps: Download and install all dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	@echo "Installing Phase 2 dependencies..."
	$(GOGET) github.com/bits-and-blooms/bloom/v3
	$(GOGET) github.com/edsrzf/mmap-go
	$(GOMOD) tidy
	@echo "Dependencies installed!"

## fmt: Format code
fmt:
	@echo "Formatting code..."
	gofmt -l -w .

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

## lint: Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install from: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...

## stats: Show code statistics
stats:
	@echo "Code statistics:"
	@echo "Total Go files:"
	@find pkg cmd -name "*.go" 2>/dev/null | wc -l
	@echo "Total lines of code:"
	@find pkg cmd -name "*.go" -exec wc -l {} + 2>/dev/null | tail -1

## check: Run all checks (fmt, vet, test)
check: fmt vet test

## help: Show this help message
help:
	@echo "Storage Engine Graph Database - Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
	@echo ""
	@echo "Examples:"
	@echo "  make deps           - Install all dependencies"
	@echo "  make example-phase1 - Run Phase 1 example (In-memory only)"
	@echo "  make example-phase2 - Run Phase 2 example (SSTable + Cache)"
	@echo "  make test           - Run all tests with race detector"
	@echo "  make bench          - Run performance benchmarks"
	@echo "  make check          - Run formatting, vetting, and tests"
	@echo "  make clean          - Clean all build artifacts and data"

.DEFAULT_GOAL := help
