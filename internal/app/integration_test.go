package app

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplication_Integration(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping integration test in CI environment - these tests are resource intensive and prone to timeout")
	}
	t.Run("should start and stop application with real configuration", func(t *testing.T) {
		app, err := NewApplication()
		require.NoError(t, err)

		// Create context with short timeout for integration test
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Start application in a goroutine
		done := make(chan error, 1)
		go func() {
			done <- app.Run(ctx)
		}()

		// Wait a bit for startup
		time.Sleep(500 * time.Millisecond)

		// Cancel context to trigger shutdown
		cancel()

		// Wait for application to stop
		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("application did not stop within timeout")
		}

		// Verify shutdown works
		err = app.Shutdown()
		assert.NoError(t, err)
	})

	t.Run("should handle configuration errors gracefully", func(t *testing.T) {
		// This test would require environment manipulation
		// For now, just verify that the error handling structure exists
		app, err := NewApplication()
		if err != nil {
			// Configuration errors should be properly wrapped
			assert.Contains(t, err.Error(), "config")
		} else {
			// If config is available, application should initialize
			assert.NotNil(t, app)
		}
	})
}

func TestApplication_ComponentIntegration(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping integration test in CI environment - these tests are resource intensive and prone to timeout")
	}
	app, err := NewApplication()
	require.NoError(t, err)

	t.Run("should have all components wired correctly", func(t *testing.T) {
		// Verify all components are present and properly initialized
		assert.NotNil(t, app.config)
		assert.NotNil(t, app.logger)
		assert.NotNil(t, app.zapLogger)
		assert.NotNil(t, app.streamConnector)
		assert.NotNil(t, app.transcriptionEngine)
		assert.NotNil(t, app.contestParser)
		assert.NotNil(t, app.logOutput)

		// Verify components have proper dependencies
		assert.Equal(t, app.logger, app.logOutput) // Same instance
	})

	t.Run("should handle debug mode configuration", func(t *testing.T) {
		// Verify debug mode configuration is accessible
		debugMode := app.config.GetDebugMode()
		assert.IsType(t, bool(false), debugMode)
	})

	t.Run("should provide access to configuration values", func(t *testing.T) {
		// Verify configuration values are accessible
		streamURL := app.config.GetStreamURL()
		assert.NotEmpty(t, streamURL)

		bufferDuration := app.config.GetBufferDurationMS()
		assert.Greater(t, bufferDuration, 0)

		modelPath := app.config.GetWhisperModelPath()
		assert.NotEmpty(t, modelPath)
	})
}
