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

	// Verify whisper.cpp CUDA container is used as source
	assert.Contains(t, content, "FROM ghcr.io/ggml-org/whisper.cpp:main-cuda")

	// Verify builder stage uses CUDA development image from NGC registry
	assert.Contains(t, content, "FROM nvcr.io/nvidia/cuda:12.4.0-devel-ubuntu22.04 AS builder")

	// Verify runtime stage uses CUDA 12.4.0 runtime image to match whisper.cpp container
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

	// Verify CUDA base images are used (they provide necessary CUDA development tools)
	assert.Contains(t, content, "nvcr.io/nvidia/cuda:12.4.0-devel-ubuntu22.04")
	assert.Contains(t, content, "nvcr.io/nvidia/cuda:12.4.0-runtime-ubuntu22.04")

	// Verify pre-built whisper.cpp CUDA binaries are copied
	assert.Contains(t, content, "COPY --from=whisper-source /app/build/bin/whisper-cli")
	assert.Contains(t, content, "COPY --from=whisper-source /app/build/src/libwhisper.so")
	assert.Contains(t, content, "COPY --from=whisper-source /app/build/ggml/src/ggml-cuda/libggml-cuda.so")

	// Verify dynamic linker cache is updated for CUDA libraries
	assert.Contains(t, content, "RUN ldconfig")
}

func TestDockerfileMaintainsExistingDependencies(t *testing.T) {
	// Read Dockerfile content
	dockerfile, err := os.ReadFile("Dockerfile")
	assert.NoError(t, err)
	content := string(dockerfile)

	// Verify essential dependencies are still present (cmake not needed since we use pre-built binaries)
	essentialDeps := []string{
		"build-essential",
		"pkg-config",
		"git",
		"ffmpeg",
		"libopenblas-dev",
		"ca-certificates",
		"libopenblas0",
		"curl",
	}

	for _, dep := range essentialDeps {
		assert.Contains(t, content, dep, "Missing dependency: %s", dep)
	}

	// Verify cmake and bc are no longer required since we use pre-built whisper.cpp binaries
	assert.NotContains(t, content, "cmake", "cmake should not be required with pre-built binaries")
	// Check that 'bc' is not listed as a package dependency (avoid false positive from container hash)
	assert.NotContains(t, content, "bc \\", "bc package should not be required with pre-built binaries")
	assert.NotContains(t, content, " bc", "bc package should not be required with pre-built binaries")
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

	// Verify we use the official whisper.cpp CUDA container instead of building from source
	assert.Contains(t, content, "FROM ghcr.io/ggml-org/whisper.cpp:main-cuda")
	assert.Contains(t, content, "AS whisper-source")

	// Verify we copy pre-built CUDA binaries instead of building
	assert.Contains(t, content, "COPY --from=whisper-source /app/build/bin/whisper-cli")
	assert.Contains(t, content, "COPY --from=whisper-source /app/build/ggml/src/ggml-cuda/libggml-cuda.so")

	// Verify we copy the included base model
	assert.Contains(t, content, "COPY --from=whisper-source /app/models/ggml-base.en.bin")

	// Verify we no longer build whisper.cpp from source (these should not be present)
	assert.NotContains(t, content, "DGGML_CUDA=ON", "Should not build whisper.cpp from source")
	assert.NotContains(t, content, "cmake -B build", "Should not compile whisper.cpp")
	assert.NotContains(t, content, "git clone", "Should not clone whisper.cpp repository")
}
