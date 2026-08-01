# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=0.1.0 -X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown') -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o k8s-telegram-bot ./cmd/k8sbot

# Final stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/k8s-telegram-bot .

# Create non-root user
RUN adduser -D -u 1000 botuser && chown botuser:botuser /app/k8s-telegram-bot
USER botuser

# Default config path
ENV K8SBOT_CONFIG=/app/config.yaml

# Expose webhook port (if using webhook mode)
EXPOSE 8443

ENTRYPOINT ["/app/k8s-telegram-bot"]
CMD ["--config", "/app/config.yaml"]