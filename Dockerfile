# Multi-stage build for Telegram Trader Coder Bot

# Build stage (Debian/glibc to avoid musl CGO issues with go-sqlite3)
FROM golang:1.24-bookworm AS builder

# Install build dependencies (CGO toolchain)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata build-essential pkg-config git && \
    rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application for linux/amd64 explicitly
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o bot ./cmd/bot

# Final stage (Debian runtime with glibc)
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata wget sqlite3 && \
    rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -u 1001 -m appuser

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/bot .

# Copy any additional files if needed
# COPY --from=builder /app/config ./config

# Create data directory for SQLite
RUN mkdir -p /app/data && \
    chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 9095

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9095/healthz || exit 1

# Run the application
CMD ["./bot"]
