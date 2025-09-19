package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"radiocontestwinner/internal/buffer"
	"radiocontestwinner/internal/config"
)

func TestContestParser_Integration(t *testing.T) {
	t.Run("should filter contexts using allowlist from configuration file", func(t *testing.T) {
		// Arrange - create temporary config file with allowlist
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")
		configContent := `allowlist:
  numbers:
    - "73"
    - "146"
    - "222"`

		err := os.WriteFile(configFile, []byte(configContent), 0644)
		assert.NoError(t, err)

		// Load configuration
		cfg, err := config.NewConfigurationFromFile(configFile)
		assert.NoError(t, err)

		// Create Contest Parser with configuration allowlist
		allowlist := cfg.GetAllowlist()
		parser := NewContestParser(allowlist)

		// Create test contexts
		inputCh := make(chan buffer.BufferedContext, 4)
		outputCh := make(chan buffer.BufferedContext, 4)

		contexts := []buffer.BufferedContext{
			{Text: "This is 73 calling CQ contest", StartMS: 1000, EndMS: 2000},        // Should pass (73)
			{Text: "Commercial break for insurance", StartMS: 2000, EndMS: 3000},      // Should be filtered out
			{Text: "Station 146 please respond", StartMS: 3000, EndMS: 4000},         // Should pass (146)
			{Text: "Weather report: 75 degrees", StartMS: 4000, EndMS: 5000},         // Should be filtered out (75 not in allowlist)
		}

		// Act - send contexts and process
		for _, ctx := range contexts {
			inputCh <- ctx
		}
		close(inputCh)

		parser.ProcessBufferedContext(inputCh, outputCh)

		// Assert - should only receive contexts with allowlisted numbers
		var results []buffer.BufferedContext
		for result := range outputCh {
			results = append(results, result)
		}

		assert.Len(t, results, 2, "should pass through 2 contexts with allowlisted numbers")
		assert.Equal(t, "This is 73 calling CQ contest", results[0].Text)
		assert.Equal(t, "Station 146 please respond", results[1].Text)
	})

	t.Run("should filter contexts using allowlist from environment variable", func(t *testing.T) {
		// Arrange - set environment variable
		os.Setenv("ALLOWLIST_NUMBERS", "73,146")
		defer os.Unsetenv("ALLOWLIST_NUMBERS")

		// Load configuration from environment
		cfg, err := config.NewConfigurationFromEnv()
		assert.NoError(t, err)

		// Create Contest Parser with configuration allowlist
		allowlist := cfg.GetAllowlist()
		parser := NewContestParser(allowlist)

		// Create test contexts
		inputCh := make(chan buffer.BufferedContext, 2)
		outputCh := make(chan buffer.BufferedContext, 2)

		contexts := []buffer.BufferedContext{
			{Text: "Station 73 calling", StartMS: 1000, EndMS: 2000},              // Should pass (73)
			{Text: "Random chatter without numbers", StartMS: 2000, EndMS: 3000}, // Should be filtered out
		}

		// Act
		for _, ctx := range contexts {
			inputCh <- ctx
		}
		close(inputCh)

		parser.ProcessBufferedContext(inputCh, outputCh)

		// Assert
		var results []buffer.BufferedContext
		for result := range outputCh {
			results = append(results, result)
		}

		assert.Len(t, results, 1, "should pass through 1 context with allowlisted number")
		assert.Equal(t, "Station 73 calling", results[0].Text)
	})

	t.Run("should handle empty allowlist gracefully", func(t *testing.T) {
		// Arrange - create config with empty allowlist
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")
		configContent := `allowlist:
  numbers: []`

		err := os.WriteFile(configFile, []byte(configContent), 0644)
		assert.NoError(t, err)

		cfg, err := config.NewConfigurationFromFile(configFile)
		assert.NoError(t, err)

		allowlist := cfg.GetAllowlist()
		parser := NewContestParser(allowlist)

		// Create test contexts
		inputCh := make(chan buffer.BufferedContext, 2)
		outputCh := make(chan buffer.BufferedContext, 2)

		contexts := []buffer.BufferedContext{
			{Text: "This is 73 calling", StartMS: 1000, EndMS: 2000},
			{Text: "Station 146 responding", StartMS: 2000, EndMS: 3000},
		}

		// Act
		for _, ctx := range contexts {
			inputCh <- ctx
		}
		close(inputCh)

		parser.ProcessBufferedContext(inputCh, outputCh)

		// Assert - should filter out all contexts when allowlist is empty
		var results []buffer.BufferedContext
		for result := range outputCh {
			results = append(results, result)
		}

		assert.Empty(t, results, "should filter out all contexts when allowlist is empty")
	})

	t.Run("should handle configuration without allowlist section", func(t *testing.T) {
		// Arrange - create config without allowlist section
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")
		configContent := `stream:
  url: "https://test.example.com/stream.aac"`

		err := os.WriteFile(configFile, []byte(configContent), 0644)
		assert.NoError(t, err)

		cfg, err := config.NewConfigurationFromFile(configFile)
		assert.NoError(t, err)

		allowlist := cfg.GetAllowlist()
		parser := NewContestParser(allowlist)

		// Create test context
		inputCh := make(chan buffer.BufferedContext, 1)
		outputCh := make(chan buffer.BufferedContext, 1)

		context := buffer.BufferedContext{
			Text:    "This is 73 calling",
			StartMS: 1000,
			EndMS:   2000,
		}

		// Act
		inputCh <- context
		close(inputCh)

		parser.ProcessBufferedContext(inputCh, outputCh)

		// Assert - should filter out context when no allowlist is configured
		var results []buffer.BufferedContext
		for result := range outputCh {
			results = append(results, result)
		}

		assert.Empty(t, results, "should filter out all contexts when no allowlist is configured")
	})
}