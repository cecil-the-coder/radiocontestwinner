package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Configuration provides type-safe access to application settings
type Configuration struct {
	viper *viper.Viper
}

// NewConfiguration creates a new Configuration instance with default settings
func NewConfiguration() *Configuration {
	v := viper.New()
	v.SetDefault("stream.url", "https://ais-sa1.streamon.fm:443/7346_48k.aac")
	v.SetDefault("whisper.model_path", "./models/ggml-base.en.bin")
	v.SetDefault("buffer.duration_ms", 2500)
	v.SetDefault("allowlist.numbers", []string{})
	return &Configuration{viper: v}
}

// NewConfigurationFromFile creates a Configuration instance from a config file
func NewConfigurationFromFile(configFile string) (*Configuration, error) {
	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetDefault("stream.url", "https://ais-sa1.streamon.fm:443/7346_48k.aac")
	v.SetDefault("whisper.model_path", "./models/ggml-base.en.bin")
	v.SetDefault("buffer.duration_ms", 2500)
	v.SetDefault("allowlist.numbers", []string{})

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}

	// Validate buffer duration
	bufferDuration := v.GetInt("buffer.duration_ms")
	if bufferDuration < 1000 || bufferDuration > 10000 {
		return nil, fmt.Errorf("buffer duration must be between 1000 and 10000 milliseconds, got %d", bufferDuration)
	}

	return &Configuration{viper: v}, nil
}

// NewConfigurationFromEnv creates a Configuration instance that reads from environment variables
func NewConfigurationFromEnv() (*Configuration, error) {
	v := viper.New()
	v.SetDefault("stream.url", "https://ais-sa1.streamon.fm:443/7346_48k.aac")
	v.SetDefault("whisper.model_path", "./models/ggml-base.en.bin")
	v.SetDefault("buffer.duration_ms", 2500)
	v.SetDefault("allowlist.numbers", []string{})

	// Set up environment variable mapping
	v.SetEnvPrefix("RADIO")
	v.AutomaticEnv()

	// Map specific environment variables
	v.BindEnv("stream.url", "STREAM_URL")
	v.BindEnv("whisper.model_path", "WHISPER_MODEL_PATH")
	v.BindEnv("buffer.duration_ms", "BUFFER_DURATION_MS")
	v.BindEnv("allowlist.numbers", "ALLOWLIST_NUMBERS")

	return &Configuration{viper: v}, nil
}

// GetStreamURL returns the configured stream URL
func (c *Configuration) GetStreamURL() string {
	return c.viper.GetString("stream.url")
}

// GetWhisperModelPath returns the configured Whisper model path
func (c *Configuration) GetWhisperModelPath() string {
	return c.viper.GetString("whisper.model_path")
}

// GetBufferDurationMS returns the configured buffer duration in milliseconds
func (c *Configuration) GetBufferDurationMS() int {
	return c.viper.GetInt("buffer.duration_ms")
}

// GetAllowlist returns the configured allowlist of numbers
func (c *Configuration) GetAllowlist() []string {
	// Check if we have an array (from config file)
	allowlistSlice := c.viper.GetStringSlice("allowlist.numbers")

	// If we have exactly one element that contains commas, it's likely from environment variable
	if len(allowlistSlice) == 1 && strings.Contains(allowlistSlice[0], ",") {
		// Split comma-separated values and trim spaces
		numbers := strings.Split(allowlistSlice[0], ",")
		result := make([]string, 0, len(numbers))
		for _, num := range numbers {
			trimmed := strings.TrimSpace(num)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}

	// Return the slice as-is (could be empty, single element, or multiple elements)
	return allowlistSlice
}