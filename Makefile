# telectl Makefile

.PHONY: build test lint fmt vet clean install run docker-build docker-push help

# Variables
BINARY_NAME := telectl
VERSION := 0.1.0
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags="-w -s -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)"
GO_FILES := $(shell find . -name '*.go' -not -path './vendor/*' -not -path './.git/*')
DOCKER_IMAGE := ksauraj/telectl
DOCKER_TAG := $(VERSION)

# Default target
all: build

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/telectl
	@echo "Build complete: $(BINARY_NAME)"

build-all: ## Build for all platforms
	@echo "Building for all platforms..."
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 ./cmd/telectl
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 ./cmd/telectl
	@GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 ./cmd/telectl
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 ./cmd/telectl
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/telectl
	@echo "Build complete in dist/"

test: ## Run tests
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...

test-unit: ## Run unit tests only
	@go test -v -short ./internal/...

test-integration: ## Run integration tests
	@go test -v -tags=integration ./...

coverage: test ## Generate coverage report
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint: ## Run linters
	@echo "Running linters..."
	@golangci-lint run ./...

fmt: ## Format code
	@echo "Formatting code..."
	@gofmt -w $(GO_FILES)
	@goimports -w $(GO_FILES)

vet: ## Run go vet
	@go vet ./...

staticcheck: ## Run staticcheck
	@staticcheck ./...

clean: ## Clean build artifacts
	@rm -f $(BINARY_NAME)
	@rm -rf dist/
	@rm -f coverage.out coverage.html
	@go clean -cache

install: build ## Install binary to GOPATH/bin
	@go install $(LDFLAGS) ./cmd/telectl

run: build ## Build and run the bot (requires config)
	@./$(BINARY_NAME) --config config.yaml

run-dev: ## Run with development settings
	@TELEGRAM_BOT_TOKEN=$(TELEGRAM_BOT_TOKEN) KUBECONFIG=$(KUBECONFIG) go run ./cmd/telectl --log-level debug

docker-build: ## Build Docker image
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) -t $(DOCKER_IMAGE):latest .

docker-push: docker-build ## Push Docker image
	@docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	@docker push $(DOCKER_IMAGE):latest

docker-run: docker-build ## Run Docker container
	@docker run --rm -it \
		-v ~/.kube:/home/telectl/.kube:ro \
		-v $(PWD)/config.yaml:/app/config.yaml:ro \
		-e TELEGRAM_BOT_TOKEN=$(TELEGRAM_BOT_TOKEN) \
		$(DOCKER_IMAGE):latest

generate: ## Generate code (mocks, etc.)
	@go generate ./...

tidy: ## Tidy go modules
	@go mod tidy
	@go mod verify

vendor: ## Vendor dependencies
	@go mod vendor

check: fmt vet staticcheck test ## Run all checks

ci: check ## CI pipeline

release: ## Create release (tags current commit)
	@git tag -a v$(VERSION) -m "Release v$(VERSION)"
	@git push origin v$(VERSION)

# Development helpers
dev-setup: ## Setup development environment
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install honnef.co/go/tools/cmd/staticcheck@latest
	@go install github.com/golang/mock/mockgen@latest

mock: ## Generate mocks
	@mockgen -source=internal/k8s/client.go -destination=internal/k8s/mock_client.go -package=k8s

# Kubernetes helpers
kubeconfig-example: ## Show example kubeconfig
	@echo "apiVersion: v1"
	@echo "kind: Config"
	@echo "clusters:"
	@echo "  - name: my-cluster"
	@echo "    cluster:"
	@echo "      server: https://kubernetes.example.com"
	@echo "      certificate-authority-data: LS0tLS1CRUdJTi..."
	@echo "users:"
	@echo "  - name: my-user"
	@echo "    user:"
	@echo "      client-certificate-data: LS0tLS1CRUdJTi..."
	@echo "      client-key-data: LS0tLS1CRUdJTi..."
	@echo "contexts:"
	@echo "  - name: my-context"
	@echo "    context:"
	@echo "      cluster: my-cluster"
	@echo "      user: my-user"
	@echo "      namespace: default"
	@echo "current-context: my-context"