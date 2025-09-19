# Multi-stage Dockerfile for Radio Contest Winner
# Stage 1: Build stage with Go 1.24 and all dependencies
FROM golang:1.24-bookworm AS builder

# Install system dependencies for build
RUN apt-get update && apt-get install -y \
    build-essential \
    pkg-config \
    cmake \
    git \
    bc \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Run tests with intelligent coverage requirement
RUN chmod +x scripts/coverage.sh && ./scripts/coverage.sh

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o radiocontestwinner ./cmd/radiocontestwinner

# Stage 2: Final runtime stage with debian:slim
FROM debian:bookworm-slim AS runtime

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ffmpeg \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean

# Create non-root user for security
RUN useradd -r -u 1000 -m -s /bin/bash appuser

# Create application directory and set ownership
RUN mkdir -p /app && chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Set working directory
WORKDIR /app

# Copy built application from builder stage
COPY --from=builder --chown=appuser:appuser /app/radiocontestwinner .

# Create directory for output files
RUN mkdir -p /app/output

# Health check to ensure application can start
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ./radiocontestwinner --help || exit 1

# Expose port (if needed for future web interface)
EXPOSE 8080

# Run the application
CMD ["./radiocontestwinner"]