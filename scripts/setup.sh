#!/bin/bash
# telectl Development Setup Script

set -euo pipefail

echo "🔧 Setting up telectl development environment..."

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
REQUIRED_VERSION="1.23"
if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
    echo "❌ Go version $REQUIRED_VERSION+ required, found $GO_VERSION"
    exit 1
fi
echo "✅ Go version: $GO_VERSION"

# Install development tools
echo "📦 Installing development tools..."

# golangci-lint
if ! command -v golangci-lint &> /dev/null; then
    echo "  Installing golangci-lint..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
else
    echo "  golangci-lint already installed"
fi

# goimports
if ! command -v goimports &> /dev/null; then
    echo "  Installing goimports..."
    go install golang.org/x/tools/cmd/goimports@latest
else
    echo "  goimports already installed"
fi

# staticcheck
if ! command -v staticcheck &> /dev/null; then
    echo "  Installing staticcheck..."
    go install honnef.co/go/tools/cmd/staticcheck@latest
else
    echo "  staticcheck already installed"
fi

# mockgen
if ! command -v mockgen &> /dev/null; then
    echo "  Installing mockgen..."
    go install github.com/golang/mock/mockgen@latest
else
    echo "  mockgen already installed"
fi

# Download dependencies
echo "📥 Downloading Go modules..."
go mod download
go mod tidy

# Verify modules
echo "✅ Verifying modules..."
go mod verify

# Run initial checks
echo "🔍 Running initial checks..."
make fmt
make vet

echo ""
echo "✨ Setup complete! You can now run:"
echo "  make build        - Build the binary"
echo "  make test         - Run tests"
echo "  make check        - Run all checks (fmt, vet, staticcheck, test)"
echo "  make run-dev      - Run in development mode (requires TELEGRAM_BOT_TOKEN)"
echo ""
echo "📝 Don't forget to:"
echo "  1. Copy config.yaml.example to ~/.config/telectl.yaml"
echo "  2. Add your TELEGRAM_BOT_TOKEN"
echo "  3. Configure ALLOWED_USER_IDS for security"