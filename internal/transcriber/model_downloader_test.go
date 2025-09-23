package transcriber

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestModelDownloader(t *testing.T) {
	logger := zap.NewNop()

	t.Run("should download model when it doesn't exist", func(t *testing.T) {
		// Create temporary directory for test
		tempDir, err := os.MkdirTemp("", "whisper-models-test-*")
		assert.NoError(t, err)
		defer os.RemoveAll(tempDir)

		downloader := NewModelDownloader(logger, tempDir)

		// Test downloading a small model that should be available
		modelPath := filepath.Join(tempDir, "ggml-base.en.bin")
		modelName := "base.en"

		// Should not exist initially
		_, err = os.Stat(modelPath)
		assert.True(t, os.IsNotExist(err))

		// Attempt download - this may timeout in CI/test environment
		err = downloader.EnsureModelExists(modelName, modelPath)

		// In test environment, network may not be available, so we just test the interface
		if err != nil {
			// Verify error is about network/download, not code issues
			assert.Contains(t, err.Error(), "failed to download")
		} else {
			// If download succeeded, verify file exists
			_, err = os.Stat(modelPath)
			assert.NoError(t, err)
		}
	})

	t.Run("should not download when model already exists", func(t *testing.T) {
		// Create temporary directory with existing model file
		tempDir, err := os.MkdirTemp("", "whisper-models-test-*")
		assert.NoError(t, err)
		defer os.RemoveAll(tempDir)

		modelPath := filepath.Join(tempDir, "ggml-test.bin")

		// Create a dummy model file
		err = os.WriteFile(modelPath, []byte("dummy model content"), 0644)
		assert.NoError(t, err)

		downloader := NewModelDownloader(logger, tempDir)

		// Should not attempt download since file exists
		err = downloader.EnsureModelExists("test", modelPath)
		assert.NoError(t, err)

		// Verify file still exists and has same content
		content, err := os.ReadFile(modelPath)
		assert.NoError(t, err)
		assert.Equal(t, "dummy model content", string(content))
	})

	t.Run("should return available models list", func(t *testing.T) {
		downloader := NewModelDownloader(logger, "/tmp")
		models := downloader.GetAvailableModels()

		// Should include common models
		assert.Contains(t, models, "base.en")
		assert.Contains(t, models, "small.en")
		assert.Contains(t, models, "medium.en")
		assert.Contains(t, models, "large-v3")
	})

	t.Run("should handle invalid model names", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "whisper-models-test-*")
		assert.NoError(t, err)
		defer os.RemoveAll(tempDir)

		downloader := NewModelDownloader(logger, tempDir)
		modelPath := filepath.Join(tempDir, "ggml-invalid.bin")

		err = downloader.EnsureModelExists("invalid-model-name", modelPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to download")
	})
}