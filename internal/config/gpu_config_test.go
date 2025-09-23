package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGPUConfigurationMapping(t *testing.T) {
	t.Run("should support both config file formats", func(t *testing.T) {
		// Create a temporary config file with the new gpu.* format
		configContent := `
gpu:
  enabled: true
  auto_detect: true
  device_id: 1
`
		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		assert.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString(configContent)
		assert.NoError(t, err)
		tmpFile.Close()

		cfg, err := NewConfigurationFromFile(tmpFile.Name())
		assert.NoError(t, err)

		// Should read GPU configuration from gpu.* keys
		assert.True(t, cfg.GetCUBLASEnabled())
		assert.True(t, cfg.GetCUBLASAutoDetect())
		assert.Equal(t, 1, cfg.GetGPUDeviceID())
	})

	t.Run("should maintain backward compatibility with whisper.* format", func(t *testing.T) {
		// Create a temporary config file with the old whisper.* format
		configContent := `
whisper:
  cublas_enabled: true
  cublas_auto_detect: false
  gpu_device_id: 2
`
		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		assert.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString(configContent)
		assert.NoError(t, err)
		tmpFile.Close()

		cfg, err := NewConfigurationFromFile(tmpFile.Name())
		assert.NoError(t, err)

		// Should read GPU configuration from whisper.* keys
		assert.True(t, cfg.GetCUBLASEnabled())
		assert.False(t, cfg.GetCUBLASAutoDetect())
		assert.Equal(t, 2, cfg.GetGPUDeviceID())
	})

	t.Run("should prefer gpu.* over whisper.* when both exist", func(t *testing.T) {
		// Create a config with both formats, gpu.* should win
		configContent := `
gpu:
  enabled: true
  auto_detect: true
  device_id: 3
whisper:
  cublas_enabled: false
  cublas_auto_detect: false
  gpu_device_id: 99
`
		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		assert.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString(configContent)
		assert.NoError(t, err)
		tmpFile.Close()

		cfg, err := NewConfigurationFromFile(tmpFile.Name())
		assert.NoError(t, err)

		// Should use gpu.* values, not whisper.* values
		assert.True(t, cfg.GetCUBLASEnabled())
		assert.True(t, cfg.GetCUBLASAutoDetect())
		assert.Equal(t, 3, cfg.GetGPUDeviceID())
	})
}