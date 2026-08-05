# Build stage
FROM golang:1.23-alpine AS builder

# Buildx injects the target platform; the multi-arch build compiles the
# correct binary per platform instead of always producing amd64.
ARG TARGETARCH

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary for the target architecture (CGO disabled so the
# cross-arch build needs no gcc toolchain)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build \
    -ldflags="-w -s -X main.version=0.1.0 -X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown') -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o telectl ./cmd/telectl

# Final stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/telectl .

# Create non-root user
RUN adduser -D -u 1000 telectl && chown telectl:telectl /app/telectl
USER telectl

# Default config path
ENV TELECTL_CONFIG=/app/config.yaml

# Expose webhook port (if using webhook mode)
EXPOSE 8443

ENTRYPOINT ["/app/telectl"]
CMD ["--config", "/app/config.yaml"]