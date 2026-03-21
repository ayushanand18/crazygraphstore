
# Setup Guide

## Prerequisites

- Go 1.21 or later
- Make (optional, for build automation)
- Git

## Installation

### 1. Clone Repository

```bash
git clone <repository-url>
cd storage-engine-graph-db
```

### 2. Install Dependencies

```bash
# Initialize Go module
go mod init github.com/storage-engine-graph-db

# Install required packages
go get github.com/bits-and-blooms/bloom/v3
go get github.com/edsrzf/mmap-go

# Update dependencies
go mod tidy
```

### 3. Build

```bash
# Using Make
make build

# Or directly with Go
go build -o storage-engine cmd/example/main.go
```

### 4. Verify Installation

```bash
# Run example
go run cmd/example/main.go

# Expected output: Example runs successfully
```

## Development Setup

### Install Development Tools

```bash
# Install linters
go install golang.org/x/lint/golint@latest
go install honnef.co/go/tools/cmd/staticcheck@latest

# Install testing tools
go install gotest.tools/gotestsum@latest
```

### IDE Setup (VSCode)

Install recommended extensions:
- Go (golang.go)
- Go Test Explorer
- Go Outliner

### Configure Git Hooks

```bash
# Pre-commit: Run tests
cat > .git/hooks/pre-commit << 'EOF'
#!/bin/bash
go test ./... || exit 1
EOF

chmod +x .git/hooks/pre-commit
```

## Troubleshooting

### Common Issues

**Issue**: `cannot find package`
```bash
# Solution: Run go mod tidy
go mod tidy
```

**Issue**: Build fails with linker errors
```bash
# Solution: Clear build cache
go clean -cache
go build
```

**Issue**: Tests fail
```bash
# Solution: Check Go version
go version  # Should be 1.21+
```
