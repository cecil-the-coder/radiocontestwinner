package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestWhisperModelConfiguration(t *testing.T) {
	t.Run("should use explicit model path when provided", func(t *testing.T) {
		cfg := NewConfiguration()
		cfg.viper.Set("whisper.model_path", "/custom/path/model.bin")

		path := cfg.GetWhisperModelPath()
		assert.Equal(t, "/custom/path/model.bin", path)
	})

	t.Run("should construct path from model name", func(t *testing.T) {
		// Create config without default for model_path
		cfg := &Configuration{viper: viper.New()}
		cfg.viper.Set("whisper.model_name", "medium")

		path := cfg.GetWhisperModelPath()
		assert.Equal(t, "/app/models/ggml-medium.bin", path)

		modelName := cfg.GetWhisperModelName()
		assert.Equal(t, "medium", modelName)
	})

	t.Run("should use default when neither path nor name is set", func(t *testing.T) {
		cfg := NewConfiguration()

		path := cfg.GetWhisperModelPath()
		assert.Equal(t, "/app/models/ggml-base.en.bin", path)

		modelName := cfg.GetWhisperModelName()
		assert.Equal(t, "", modelName)
	})

	t.Run("should prioritize explicit path over model name", func(t *testing.T) {
		cfg := NewConfiguration()
		cfg.viper.Set("whisper.model_path", "/explicit/path.bin")
		cfg.viper.Set("whisper.model_name", "medium")

		path := cfg.GetWhisperModelPath()
		assert.Equal(t, "/explicit/path.bin", path)
	})

	t.Run("should read model name from WHISPER_MODEL environment variable", func(t *testing.T) {
		// Set environment variable
		oldValue := os.Getenv("WHISPER_MODEL")
		defer func() {
			if oldValue != "" {
				os.Setenv("WHISPER_MODEL", oldValue)
			} else {
				os.Unsetenv("WHISPER_MODEL")
			}
		}()

		os.Setenv("WHISPER_MODEL", "large-v3")

		// Create config with environment variable binding but no defaults for model_path
		v := viper.New()
		v.BindEnv("whisper.model_name", "WHISPER_MODEL")
		v.AutomaticEnv()
		cfg := &Configuration{viper: v}

		modelName := cfg.GetWhisperModelName()
		assert.Equal(t, "large-v3", modelName)

		path := cfg.GetWhisperModelPath()
		assert.Equal(t, "/app/models/ggml-large-v3.bin", path)
	})

	t.Run("should handle empty model name gracefully", func(t *testing.T) {
		cfg := NewConfiguration()
		cfg.viper.Set("whisper.model_name", "")

		path := cfg.GetWhisperModelPath()
		assert.Equal(t, "/app/models/ggml-base.en.bin", path) // Should fall back to default
	})
}