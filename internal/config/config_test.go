package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfiguration_GetStreamURL(t *testing.T) {
	t.Run("should return configured stream URL", func(t *testing.T) {
		// Arrange
		cfg := NewConfiguration()

		// Act
		url := cfg.GetStreamURL()

		// Assert
		assert.NotEmpty(t, url, "stream URL should not be empty")
		assert.Contains(t, url, "https://", "stream URL should use HTTPS")
	})

	t.Run("should load stream URL from config file", func(t *testing.T) {
		// Arrange - create temporary config file
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")
		configContent := `stream:
  url: "https://test.example.com/stream.aac"`

		err := os.WriteFile(configFile, []byte(configContent), 0644)
		assert.NoError(t, err)

		cfg, err := NewConfigurationFromFile(configFile)
		assert.NoError(t, err)

		// Act
		url := cfg.GetStreamURL()

		// Assert
		assert.Equal(t, "https://test.example.com/stream.aac", url)
	})

	t.Run("should load stream URL from environment variable", func(t *testing.T) {
		// Arrange
		testURL := "https://env.example.com/stream.aac"
		os.Setenv("STREAM_URL", testURL)
		defer os.Unsetenv("STREAM_URL")

		cfg, err := NewConfigurationFromEnv()
		assert.NoError(t, err)

		// Act
		url := cfg.GetStreamURL()

		// Assert
		assert.Equal(t, testURL, url)
	})

	t.Run("should return error for non-existent config file", func(t *testing.T) {
		// Arrange
		nonExistentFile := "/tmp/non-existent-config.yaml"

		// Act
		cfg, err := NewConfigurationFromFile(nonExistentFile)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "failed to read config file")
	})

	t.Run("should return error for invalid config file format", func(t *testing.T) {
		// Arrange - create invalid YAML file
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "invalid.yaml")
		invalidContent := `stream:
  url: "https://test.example.com/stream.aac"
invalid_yaml: [unclosed_bracket`

		err := os.WriteFile(configFile, []byte(invalidContent), 0644)
		assert.NoError(t, err)

		// Act
		cfg, err := NewConfigurationFromFile(configFile)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("should fall back to default URL when config file lacks stream section", func(t *testing.T) {
		// Arrange - create config file without stream section
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "minimal.yaml")
		configContent := `other:
  setting: "value"`

		err := os.WriteFile(configFile, []byte(configContent), 0644)
		assert.NoError(t, err)

		cfg, err := NewConfigurationFromFile(configFile)
		assert.NoError(t, err)

		// Act
		url := cfg.GetStreamURL()

		// Assert
		assert.Equal(t, "https://ais-sa1.streamon.fm:443/7346_48k.aac", url)
	})

	t.Run("should fall back to default URL when environment variable not set", func(t *testing.T) {
		// Arrange - ensure environment variable is not set
		os.Unsetenv("STREAM_URL")

		cfg, err := NewConfigurationFromEnv()
		assert.NoError(t, err)

		// Act
		url := cfg.GetStreamURL()

		// Assert
		assert.Equal(t, "https://ais-sa1.streamon.fm:443/7346_48k.aac", url)
	})
}