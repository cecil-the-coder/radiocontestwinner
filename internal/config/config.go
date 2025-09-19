package config

import (
	"fmt"

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
	return &Configuration{viper: v}
}

// NewConfigurationFromFile creates a Configuration instance from a config file
func NewConfigurationFromFile(configFile string) (*Configuration, error) {
	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetDefault("stream.url", "https://ais-sa1.streamon.fm:443/7346_48k.aac")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}

	return &Configuration{viper: v}, nil
}

// NewConfigurationFromEnv creates a Configuration instance that reads from environment variables
func NewConfigurationFromEnv() (*Configuration, error) {
	v := viper.New()
	v.SetDefault("stream.url", "https://ais-sa1.streamon.fm:443/7346_48k.aac")

	// Set up environment variable mapping
	v.SetEnvPrefix("RADIO")
	v.AutomaticEnv()

	// Map specific environment variables
	v.BindEnv("stream.url", "STREAM_URL")

	return &Configuration{viper: v}, nil
}

// GetStreamURL returns the configured stream URL
func (c *Configuration) GetStreamURL() string {
	return c.viper.GetString("stream.url")
}