# Alyx Server Dockerfile
# Multi-stage build for minimal final image

# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o alyx ./cmd/alyx

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
# - ca-certificates for HTTPS
# - docker CLI for container management (if using docker runtime)
# - curl for healthcheck
RUN apk add --no-cache ca-certificates docker-cli curl

# Create non-root user
RUN addgroup -g 1001 alyx && \
    adduser -D -u 1001 -G alyx alyx

# Create directories
RUN mkdir -p /app/data && \
    chown -R alyx:alyx /app

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/alyx /usr/local/bin/alyx

# Switch to non-root user
USER alyx

# Expose default port
EXPOSE 8090

# Volume for data persistence
VOLUME ["/app/data"]

HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f -s http://localhost:8090/ > /dev/null || exit 1

CMD ["alyx", "dev"]
