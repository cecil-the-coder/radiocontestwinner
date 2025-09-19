# Build Troubleshooting Guide

This guide provides solutions for common issues encountered when building the Radio Contest Winner application.

## Prerequisites

- Docker Engine 26.0+ installed
- Git for version control
- 4GB+ RAM available for Docker builds
- Stable internet connection for downloading dependencies

## Quick Diagnostic

Run this command to verify your environment:
```bash
docker --version && go version && git --version
```

## Common Build Issues

### 1. Docker Build Fails with "image not found"

**Error:** `docker.io/library/debian:slim: not found`

**Solution:**
```bash
# Update Dockerfile to use correct Debian image
FROM debian:bookworm-slim AS runtime
```

**Root Cause:** Debian image tags change over time. Use versioned tags like `bookworm-slim`.

### 2. Go Module Download Failures

**Error:** `go mod download: module lookup failed`

**Solutions:**
```bash
# Clear Go module cache
go clean -modcache

# Verify network connectivity
curl -I https://proxy.golang.org

# Check go.mod syntax
go mod verify
```

### 3. CGo Compilation Errors

**Error:** `cgo: C compiler "gcc" not found`

**Solution:**
Ensure build-essential is installed in Dockerfile:
```dockerfile
RUN apt-get update && apt-get install -y \
    build-essential \
    pkg-config \
    cmake \
    git \
    bc
```

### 4. Test Coverage Failures

**Error:** `Coverage 45.2% is below required 80%`

**Solution:**
The build uses intelligent coverage validation that only enforces 80% when the codebase exceeds 100 lines. For initial development:

```bash
# Check current line count
find . -name "*.go" -not -path "./build/*" -exec wc -l {} + | tail -1

# If under 100 lines, coverage enforcement is skipped
# If over 100 lines, add more tests to reach 80% coverage
```

### 5. FFmpeg Installation Issues

**Error:** `Package 'ffmpeg' has no installation candidate`

**Solution:**
```bash
# Verify Debian repositories are updated
RUN apt-get update && apt-get install -y ffmpeg
```

### 6. Permission Denied Errors

**Error:** `permission denied: /app/radiocontestwinner`

**Solutions:**
```bash
# Ensure correct ownership in Dockerfile
COPY --from=builder --chown=appuser:appuser /app/radiocontestwinner .

# Make binary executable
RUN chmod +x /app/radiocontestwinner
```

### 7. Memory Issues During Build

**Error:** `Build failed: not enough memory`

**Solutions:**
```bash
# Increase Docker memory limit (Docker Desktop)
# Settings > Resources > Advanced > Memory: 4GB+

# Use multi-stage build to reduce memory usage
# Clean up in same RUN command:
RUN apt-get update && apt-get install -y pkg && \
    rm -rf /var/lib/apt/lists/*
```

## Advanced Troubleshooting

### Debug Docker Build Process

```bash
# Build with detailed output
docker build -f build/Dockerfile --progress=plain --no-cache -t radiocontestwinner:debug .

# Inspect intermediate layers
docker run --rm -it <layer-id> /bin/bash

# Check build context size
du -sh .
```

### Container Runtime Issues

```bash
# Test container interactively
docker run --rm -it radiocontestwinner:test /bin/bash

# Check container logs
docker run --rm --name rcw-test radiocontestwinner:test &
docker logs rcw-test

# Inspect container filesystem
docker run --rm radiocontestwinner:test ls -la /app
```

### Dependency Verification

```bash
# Verify Go dependencies
go mod verify
go mod graph

# Check system dependencies in container
docker run --rm radiocontestwinner:test ldd /app/radiocontestwinner

# Test FFmpeg availability
docker run --rm radiocontestwinner:test ffmpeg -version
```

## Environment-Specific Issues

### macOS (M1/M2)

```bash
# Force AMD64 architecture for compatibility
docker build --platform linux/amd64 -f build/Dockerfile -t radiocontestwinner:test .
```

### Windows

```bash
# Use Windows line endings
git config core.autocrlf true

# Use PowerShell for commands
docker build -f build/Dockerfile -t radiocontestwinner:test .
```

### Limited Internet Access

```bash
# Pre-download Go modules
go mod download
docker build --network=host -f build/Dockerfile -t radiocontestwinner:test .

# Use proxy settings
docker build --build-arg http_proxy=http://proxy:8080 -f build/Dockerfile .
```

## Performance Optimization

### Faster Builds

```bash
# Use BuildKit for parallel builds
DOCKER_BUILDKIT=1 docker build -f build/Dockerfile -t radiocontestwinner:test .

# Cache Go modules
docker build --mount=type=cache,target=/go/pkg/mod -f build/Dockerfile .
```

### Smaller Images

```bash
# Verify final image size
docker images radiocontestwinner:test

# Expected size: ~200-300MB (with FFmpeg)
# If larger, check for unnecessary files in final stage
```

## Build Scripts

### Automated Build Script

```bash
#!/bin/bash
# scripts/build.sh

set -e

echo "Building Radio Contest Winner..."

# Verify prerequisites
command -v docker >/dev/null 2>&1 || { echo "Docker required but not installed"; exit 1; }

# Clean previous builds
docker rmi radiocontestwinner:test 2>/dev/null || true

# Build with error handling
if docker build -f build/Dockerfile -t radiocontestwinner:test .; then
    echo "Build successful!"
    echo "Testing container..."
    docker run --rm radiocontestwinner:test --help
    echo "Container test passed!"
else
    echo "Build failed. Check the logs above for details."
    exit 1
fi
```

### Development Build

```bash
# scripts/dev-build.sh
DOCKER_BUILDKIT=1 docker build \
    --target builder \
    -f build/Dockerfile \
    -t radiocontestwinner:dev .
```

## Getting Help

If you encounter issues not covered in this guide:

1. **Check logs carefully** - Docker provides detailed error messages
2. **Verify prerequisites** - Ensure Docker, Go, and system requirements are met
3. **Test components individually** - Build and test each stage separately
4. **Check network connectivity** - Many issues stem from download failures
5. **Review recent changes** - Compare working vs non-working commits

## Monitoring Build Health

```bash
# Check Docker system status
docker system df
docker system prune -f

# Monitor build progress
docker build --progress=plain -f build/Dockerfile .

# Resource usage
docker stats --no-stream
```

## Emergency Recovery

If builds completely fail:

```bash
# Reset Docker completely
docker system prune -a -f
docker volume prune -f

# Clear Go module cache
go clean -modcache

# Reset repository (if needed)
git clean -fdx
git reset --hard HEAD
```

Remember: The build process is designed to be reproducible. If it worked once, it should work again with the same environment and inputs.