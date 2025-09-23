package build

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerfileUsesCUDABaseImages(t *testing.T) {
	// Read Dockerfile content
	dockerfile, err := os.ReadFile("Dockerfile")
	assert.NoError(t, err)
	content := string(dockerfile)

	// Verify builder stage uses CUDA development image from NGC registry
	assert.Contains(t, content, "FROM nvcr.io/nvidia/cuda:12.4.0-devel-ubuntu22.04 AS builder")

	// Verify runtime stage uses CUDA runtime image from NGC registry
	assert.Contains(t, content, "FROM nvcr.io/nvidia/cuda:12.4.0-runtime-ubuntu22.04 AS runtime")

	// Ensure old base images are not used
	assert.NotContains(t, content, "FROM golang:1.24-bookworm")
	assert.NotContains(t, content, "FROM debian:bookworm-slim")
}

func TestDockerfileInstallsCUDADependencies(t *testing.T) {
	// Read Dockerfile content
	dockerfile, err := os.ReadFile("Dockerfile")
	assert.NoError(t, err)
	content := string(dockerfile)

	// Verify CUDA development libraries are installed in builder stage
	assert.Contains(t, content, "cuda-cudart-dev-12-4")
	assert.Contains(t, content, "cuda-nvrtc-dev-12-4")

	// Verify CUDA runtime library is installed in runtime stage
	assert.Contains(t, content, "cuda-cudart-12-4")
}

func TestDockerfileMaintainsExistingDependencies(t *testing.T) {
	// Read Dockerfile content
	dockerfile, err := os.ReadFile("Dockerfile")
	assert.NoError(t, err)
	content := string(dockerfile)

	// Verify all essential dependencies are still present
	essentialDeps := []string{
		"build-essential",
		"pkg-config",
		"cmake",
		"git",
		"bc",
		"ffmpeg",
		"libopenblas-dev",
		"ca-certificates",
		"libopenblas0",
		"curl",
	}

	for _, dep := range essentialDeps {
		assert.Contains(t, content, dep, "Missing dependency: %s", dep)
	}
}

func TestDockerfileMaintainsSecurityConfiguration(t *testing.T) {
	// Read Dockerfile content
	dockerfile, err := os.ReadFile("Dockerfile")
	assert.NoError(t, err)
	content := string(dockerfile)

	// Verify non-root user configuration is maintained
	assert.Contains(t, content, "USER appuser")
	assert.Contains(t, content, "useradd -r -u 1000 -m -s /bin/bash appuser")

	// Verify working directory and ownership
	assert.Contains(t, content, "WORKDIR /app")
	assert.Contains(t, content, "chown -R appuser:appuser /app")
}

func TestWhisperCppBuildsWithCUDASupport(t *testing.T) {
	// Read Dockerfile content
	dockerfile, err := os.ReadFile("Dockerfile")
	assert.NoError(t, err)
	content := string(dockerfile)

	// Verify Whisper.cpp is built with CUDA support
	assert.Contains(t, content, "WHISPER_CUBLAS=1")
	assert.Contains(t, content, "make clean")
	assert.Contains(t, content, "make -j$(nproc)")
}
