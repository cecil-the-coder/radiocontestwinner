package build

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerfileExists(t *testing.T) {
	// Test that Dockerfile exists in the expected location
	_, err := os.Stat("Dockerfile")
	assert.NoError(t, err, "Dockerfile should exist in repo/build/ directory")
}

func TestDockerfileStructure(t *testing.T) {
	// Test that Dockerfile has expected multi-stage structure
	dockerfileContent, err := os.ReadFile("Dockerfile")
	if err != nil {
		t.Skip("Dockerfile not found, skipping structure test")
		return
	}

	content := string(dockerfileContent)

	// Test for multi-stage build structure
	assert.Contains(t, content, "FROM", "Dockerfile should contain FROM instructions")
	// Updated for CUDA support - now uses NGC registry nvidia/cuda images
	assert.Contains(t, content, "nvcr.io/nvidia/cuda", "Dockerfile should use NGC registry nvidia/cuda base images for CUDA support")
	assert.Contains(t, content, "devel-ubuntu", "Dockerfile should use CUDA development image for build stage")
	assert.Contains(t, content, "runtime-ubuntu", "Dockerfile should use CUDA runtime image for final stage")

	// Test for required components
	assert.Contains(t, content, "RUN apt-get update", "Dockerfile should install system dependencies")
	assert.Contains(t, content, "ffmpeg", "Dockerfile should install FFmpeg")
	assert.Contains(t, content, "COPY", "Dockerfile should copy application code")
	assert.Contains(t, content, "go build", "Dockerfile should build Go application")

	// Test for security best practices
	assert.Contains(t, content, "USER", "Dockerfile should create non-root user")
	assert.Contains(t, content, "HEALTHCHECK", "Dockerfile should include health check")
	assert.Contains(t, content, "useradd -r", "Dockerfile should create system user for security")
}

func TestDockerfileCoverage(t *testing.T) {
	// Test that Dockerfile includes coverage testing
	dockerfileContent, err := os.ReadFile("Dockerfile")
	if err != nil {
		t.Skip("Dockerfile not found, skipping coverage test")
		return
	}

	content := string(dockerfileContent)

	// Test for test execution and coverage
	assert.True(t,
		strings.Contains(content, "go test") || strings.Contains(content, "coverage"),
		"Dockerfile should include test execution with coverage",
	)
}

func TestDockerfileOptimization(t *testing.T) {
	// Test that Dockerfile follows optimization best practices
	dockerfileContent, err := os.ReadFile("Dockerfile")
	if err != nil {
		t.Skip("Dockerfile not found, skipping optimization test")
		return
	}

	content := string(dockerfileContent)

	// Test for build optimization
	assert.Contains(t, content, "COPY go.mod", "Dockerfile should copy go.mod first for dependency caching")
	assert.Contains(t, content, "RUN go mod download", "Dockerfile should download dependencies before copying source")
}

func TestDockerfileSecrets(t *testing.T) {
	// Test that Dockerfile doesn't expose secrets
	dockerfileContent, err := os.ReadFile("Dockerfile")
	if err != nil {
		t.Skip("Dockerfile not found, skipping secrets test")
		return
	}

	content := strings.ToLower(string(dockerfileContent))

	// Test for common secret patterns
	secretPatterns := []string{
		"password",
		"secret",
		"key=",
		"token",
		"api_key",
	}

	for _, pattern := range secretPatterns {
		assert.NotContains(t, content, pattern,
			"Dockerfile should not contain hardcoded secrets: %s", pattern)
	}
}
