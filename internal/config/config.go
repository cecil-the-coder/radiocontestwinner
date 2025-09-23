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
	v.SetDefault("whisper.model_path", "/app/models/ggml-base.en.bin")
	v.SetDefault("buffer.duration_ms", 2500)
	v.SetDefault("transcription.chunk_duration_sec", 5) // Smaller chunks for streaming
	v.SetDefault("transcription.overlap_sec", 1)        // Smaller overlap for speed
	v.SetDefault("transcription.timeout_sec", 30)       // Timeout after 30 seconds of no audio
	v.SetDefault("allowlist.numbers", []string{})
	v.SetDefault("debug_mode", false)
	v.SetDefault("log.file_path", "./logs/contest_output.log")
	// GPU configuration defaults (legacy format, used as fallback)
	v.SetDefault("whisper.cublas_enabled", true)     // Enable CUDA acceleration
	v.SetDefault("whisper.cublas_auto_detect", true) // Auto-detect GPU availability
	v.SetDefault("whisper.gpu_device_id", 0)         // Default GPU device ID
	v.SetDefault("whisper.threads", 4)               // Default thread count (CPU fallback)
	return &Configuration{viper: v}
}

// NewConfigurationFromFile creates a Configuration instance from a config file
func NewConfigurationFromFile(configFile string) (*Configuration, error) {
	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetDefault("stream.url", "https://ais-sa1.streamon.fm:443/7346_48k.aac")
	v.SetDefault("whisper.model_path", "/app/models/ggml-base.en.bin")
	v.SetDefault("buffer.duration_ms", 2500)
	v.SetDefault("transcription.chunk_duration_sec", 5) // Smaller chunks for streaming
	v.SetDefault("transcription.overlap_sec", 1)        // Smaller overlap for speed
	v.SetDefault("transcription.timeout_sec", 30)       // Timeout after 30 seconds of no audio
	v.SetDefault("allowlist.numbers", []string{})
	v.SetDefault("debug_mode", false)
	v.SetDefault("log.file_path", "./logs/contest_output.log")
	// GPU configuration defaults (legacy format, used as fallback)
	v.SetDefault("whisper.cublas_enabled", true)     // Enable CUDA acceleration
	v.SetDefault("whisper.cublas_auto_detect", true) // Auto-detect GPU availability
	v.SetDefault("whisper.gpu_device_id", 0)         // Default GPU device ID
	v.SetDefault("whisper.threads", 4)               // Default thread count (CPU fallback)

	// Set up environment variable mapping (same as NewConfigurationFromEnv)
	v.SetEnvPrefix("RADIO")
	v.AutomaticEnv()

	// Map specific environment variables
	v.BindEnv("stream.url", "STREAM_URL")
	v.BindEnv("whisper.model_path", "WHISPER_MODEL_PATH")
	v.BindEnv("whisper.model_name", "WHISPER_MODEL")
	v.BindEnv("buffer.duration_ms", "BUFFER_DURATION_MS")
	v.BindEnv("allowlist.numbers", "ALLOWLIST_NUMBERS")
	v.BindEnv("debug_mode", "DEBUG_MODE")
	v.BindEnv("log.file_path", "LOG_FILE_PATH")
	// GPU configuration environment variables (new format)
	v.BindEnv("gpu.enabled", "GPU_ENABLED")
	v.BindEnv("gpu.auto_detect", "GPU_AUTO_DETECT")
	v.BindEnv("gpu.device_id", "GPU_DEVICE_ID")
	// GPU configuration environment variables (legacy format)
	v.BindEnv("whisper.cublas_enabled", "WHISPER_CUBLAS")
	v.BindEnv("whisper.cublas_auto_detect", "WHISPER_CUBLAS_AUTO_DETECT")
	v.BindEnv("whisper.gpu_device_id", "WHISPER_GPU_DEVICE_ID")
	v.BindEnv("whisper.threads", "WHISPER_THREADS")

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
	// Note: whisper.model_path default is handled in GetWhisperModelPath() to allow model_name to take precedence
	v.SetDefault("buffer.duration_ms", 2500)
	v.SetDefault("transcription.chunk_duration_sec", 5) // Smaller chunks for streaming
	v.SetDefault("transcription.overlap_sec", 1)        // Smaller overlap for speed
	v.SetDefault("transcription.timeout_sec", 30)       // Timeout after 30 seconds of no audio
	v.SetDefault("allowlist.numbers", []string{})
	v.SetDefault("debug_mode", false)
	v.SetDefault("log.file_path", "./logs/contest_output.log")
	// GPU configuration defaults (legacy format, used as fallback)
	v.SetDefault("whisper.cublas_enabled", true)     // Enable CUDA acceleration
	v.SetDefault("whisper.cublas_auto_detect", true) // Auto-detect GPU availability
	v.SetDefault("whisper.gpu_device_id", 0)         // Default GPU device ID
	v.SetDefault("whisper.threads", 4)               // Default thread count (CPU fallback)

	// Set up environment variable mapping
	v.SetEnvPrefix("RADIO")
	v.AutomaticEnv()

	// Map specific environment variables
	v.BindEnv("stream.url", "STREAM_URL")
	v.BindEnv("whisper.model_path", "WHISPER_MODEL_PATH")
	v.BindEnv("whisper.model_name", "WHISPER_MODEL")
	v.BindEnv("buffer.duration_ms", "BUFFER_DURATION_MS")
	v.BindEnv("allowlist.numbers", "ALLOWLIST_NUMBERS")
	v.BindEnv("debug_mode", "DEBUG_MODE")
	v.BindEnv("log.file_path", "LOG_FILE_PATH")
	// GPU configuration environment variables
	v.BindEnv("whisper.cublas_enabled", "WHISPER_CUBLAS")
	v.BindEnv("whisper.cublas_auto_detect", "WHISPER_CUBLAS_AUTO_DETECT")
	v.BindEnv("whisper.gpu_device_id", "WHISPER_GPU_DEVICE_ID")
	v.BindEnv("whisper.threads", "WHISPER_THREADS")

	return &Configuration{viper: v}, nil
}

// GetStreamURL returns the configured stream URL
func (c *Configuration) GetStreamURL() string {
	return c.viper.GetString("stream.url")
}

// GetWhisperModelPath returns the configured Whisper model path
func (c *Configuration) GetWhisperModelPath() string {
	// Check if model path was explicitly set (not using default)
	if c.viper.IsSet("whisper.model_path") {
		return c.viper.GetString("whisper.model_path")
	}

	// If model name is set, construct path
	modelName := c.viper.GetString("whisper.model_name")
	if modelName != "" {
		return fmt.Sprintf("/app/models/ggml-%s.bin", modelName)
	}

	// Return default
	return "/app/models/ggml-base.en.bin"
}

// GetWhisperModelName returns the configured Whisper model name
func (c *Configuration) GetWhisperModelName() string {
	return c.viper.GetString("whisper.model_name")
}

// GetBufferDurationMS returns the configured buffer duration in milliseconds
func (c *Configuration) GetBufferDurationMS() int {
	return c.viper.GetInt("buffer.duration_ms")
}

// GetTranscriptionChunkDurationSec returns the configured transcription chunk duration in seconds
func (c *Configuration) GetTranscriptionChunkDurationSec() int {
	return c.viper.GetInt("transcription.chunk_duration_sec")
}

// GetTranscriptionOverlapSec returns the configured transcription overlap duration in seconds
func (c *Configuration) GetTranscriptionOverlapSec() int {
	return c.viper.GetInt("transcription.overlap_sec")
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

// GetDebugMode returns whether debug mode is enabled
func (c *Configuration) GetDebugMode() bool {
	return c.viper.GetBool("debug_mode")
}

// SetDebugMode sets the debug mode state
func (c *Configuration) SetDebugMode(enabled bool) {
	c.viper.Set("debug_mode", enabled)
}

// GetLogFilePath returns the configured log file path
func (c *Configuration) GetLogFilePath() string {
	return c.viper.GetString("log.file_path")
}

// GetTranscriptionTimeoutSec returns the configured transcription timeout in seconds
func (c *Configuration) GetTranscriptionTimeoutSec() int {
	return c.viper.GetInt("transcription.timeout_sec")
}

// SetTranscriptionTimeoutSec sets the transcription timeout in seconds
func (c *Configuration) SetTranscriptionTimeoutSec(timeoutSec int) {
	c.viper.Set("transcription.timeout_sec", timeoutSec)
}

// GPU Configuration Methods

// GetCUBLASEnabled returns whether CUDA acceleration is enabled
func (c *Configuration) GetCUBLASEnabled() bool {
	// Check new gpu.enabled format first, then fall back to whisper.cublas_enabled
	if c.viper.IsSet("gpu.enabled") {
		return c.viper.GetBool("gpu.enabled")
	}
	if c.viper.IsSet("whisper.cublas_enabled") {
		return c.viper.GetBool("whisper.cublas_enabled")
	}
	// Default to true to enable GPU acceleration by default
	return true
}

// SetCUBLASEnabled sets whether CUDA acceleration is enabled
func (c *Configuration) SetCUBLASEnabled(enabled bool) {
	// Set both formats for compatibility
	c.viper.Set("gpu.enabled", enabled)
	c.viper.Set("whisper.cublas_enabled", enabled)
}

// GetCUBLASAutoDetect returns whether GPU auto-detection is enabled
func (c *Configuration) GetCUBLASAutoDetect() bool {
	// Check new gpu.auto_detect format first, then fall back to whisper.cublas_auto_detect
	if c.viper.IsSet("gpu.auto_detect") {
		return c.viper.GetBool("gpu.auto_detect")
	}
	if c.viper.IsSet("whisper.cublas_auto_detect") {
		return c.viper.GetBool("whisper.cublas_auto_detect")
	}
	// Default to true to enable auto-detection by default
	return true
}

// SetCUBLASAutoDetect sets whether GPU auto-detection is enabled
func (c *Configuration) SetCUBLASAutoDetect(autoDetect bool) {
	// Set both formats for compatibility
	c.viper.Set("gpu.auto_detect", autoDetect)
	c.viper.Set("whisper.cublas_auto_detect", autoDetect)
}

// GetGPUDeviceID returns the configured GPU device ID
func (c *Configuration) GetGPUDeviceID() int {
	// Check new gpu.device_id format first, then fall back to whisper.gpu_device_id
	if c.viper.IsSet("gpu.device_id") {
		return c.viper.GetInt("gpu.device_id")
	}
	if c.viper.IsSet("whisper.gpu_device_id") {
		return c.viper.GetInt("whisper.gpu_device_id")
	}
	// Default to device 0
	return 0
}

// SetGPUDeviceID sets the GPU device ID
func (c *Configuration) SetGPUDeviceID(deviceID int) {
	// Set both formats for compatibility
	c.viper.Set("gpu.device_id", deviceID)
	c.viper.Set("whisper.gpu_device_id", deviceID)
}

// GetWhisperThreads returns the configured number of threads
func (c *Configuration) GetWhisperThreads() int {
	return c.viper.GetInt("whisper.threads")
}

// SetWhisperThreads sets the number of threads
func (c *Configuration) SetWhisperThreads(threads int) {
	c.viper.Set("whisper.threads", threads)
}
