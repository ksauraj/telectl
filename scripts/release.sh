#!/bin/bash
# Release script for k8s-telegram-bot

set -euo pipefail

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 v0.1.0"
    exit 1
fi

# Validate version format
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "❌ Version must be in format vX.Y.Z (e.g., v0.1.0)"
    exit 1
fi

echo "🚀 Preparing release $VERSION"

# Check working directory is clean
if [ -n "$(git status --porcelain)" ]; then
    echo "❌ Working directory not clean. Commit or stash changes first."
    exit 1
fi

# Run all checks
echo "🔍 Running checks..."
make check

# Build for all platforms
echo "🏗️ Building for all platforms..."
make build-all

# Create and push tag
echo "🏷️ Creating tag $VERSION..."
git tag -a "$VERSION" -m "Release $VERSION"

echo "📤 Pushing tag..."
git push origin "$VERSION"

echo ""
echo "✅ Release $VERSION initiated!"
echo "GitHub Actions will build and publish the release."
echo "Check: https://github.com/ksauraj/k8s-telegram-bot/actions"