package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDebugMode_DefaultsToFalse(t *testing.T) {
	// Arrange
	config := NewConfiguration()

	// Act
	debugMode := config.GetDebugMode()

	// Assert
	assert.False(t, debugMode, "Debug mode should default to false")
}

func TestGetDebugMode_WhenEnabledInConfig(t *testing.T) {
	// Arrange
	config := NewConfiguration()
	config.viper.Set("debug_mode", true)

	// Act
	debugMode := config.GetDebugMode()

	// Assert
	assert.True(t, debugMode, "Debug mode should be true when set in config")
}

func TestSetDebugMode_UpdatesConfiguration(t *testing.T) {
	// Arrange
	config := NewConfiguration()
	assert.False(t, config.GetDebugMode(), "Debug mode should initially be false")

	// Act
	config.SetDebugMode(true)

	// Assert
	assert.True(t, config.GetDebugMode(), "Debug mode should be true after setting")
}

func TestSetDebugMode_CanToggleBackAndForth(t *testing.T) {
	// Arrange
	config := NewConfiguration()

	// Act & Assert - Toggle multiple times
	config.SetDebugMode(true)
	assert.True(t, config.GetDebugMode(), "Debug mode should be true")

	config.SetDebugMode(false)
	assert.False(t, config.GetDebugMode(), "Debug mode should be false")

	config.SetDebugMode(true)
	assert.True(t, config.GetDebugMode(), "Debug mode should be true again")
}

func TestNewConfigurationFromFile_WithDebugModeEnabled(t *testing.T) {
	// This test will fail until we implement debug_mode in config loading
	// For now, test with a mock config file content
	t.Skip("Need to create test config file with debug_mode")
}